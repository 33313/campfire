package core

import (
	"context"
	"strconv"
	"strings"

	"github.com/campfire/campfire/internal/store"
)

// NewItem is what creating a ticket or a bucket takes.
type NewItem struct {
	Kind        string   `json:"kind"`
	TeamID      int64    `json:"teamId"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	AssigneeID  int64    `json:"assigneeId,omitempty"`
	Priority    int64    `json:"priority,omitempty"`
	Icon        string   `json:"icon,omitempty"`
	BucketID    int64    `json:"bucketId,omitempty"`
	PlannedEnd  string   `json:"plannedEnd,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	// Accepted ticks "Mark as accepted": the item enters the accepted state
	// rather than the default one, and its clock starts (SPEC.md §7.3).
	Accepted bool `json:"accepted,omitempty"`
}

// ItemPatch edits an existing item. A nil field is left alone; a zero value is
// a clear, which is why every field is a pointer.
type ItemPatch struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	AssigneeID  *int64  `json:"assigneeId,omitempty"`
	Priority    *int64  `json:"priority,omitempty"`
	Icon        *string `json:"icon,omitempty"`
	BucketID    *int64  `json:"bucketId,omitempty"`
	PlannedEnd  *string `json:"plannedEnd,omitempty"`
	// Version is the value the editor last saw. Zero means "whatever is there
	// now", which is what a bulk edit and an agent send.
	Version int64 `json:"version,omitempty"`
}

const maxTitle = 300

func validateTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", invalidField("title", "an item needs a title")
	}
	if len([]rune(title)) > maxTitle {
		return "", invalidField("title", "a title is at most %d characters", maxTitle)
	}
	return title, nil
}

func validatePriority(p int64) error {
	if p < 1 || p > 4 {
		return invalidField("priority", "priority is P1 to P4")
	}
	return nil
}

// validatePlannedEnd accepts an ISO date or the empty string. Dates are stored
// as text so that they sort and compare without a timezone entering the model.
func validatePlannedEnd(d string) error {
	if d == "" {
		return nil
	}
	if len(d) != 10 || d[4] != '-' || d[7] != '-' {
		return invalidField("plannedEnd", "a planned end is a date, YYYY-MM-DD")
	}
	for i, r := range d {
		if i == 4 || i == 7 {
			continue
		}
		if r < '0' || r > '9' {
			return invalidField("plannedEnd", "a planned end is a date, YYYY-MM-DD")
		}
	}
	return nil
}

// CreateItem creates a ticket or a bucket.
//
// A new item enters its team's default state, or the accepted state if the
// creator ticked "Mark as accepted" (SPEC.md §7.3). The reporter is the
// creating user and is not editable; the assignee is empty and is never
// inferred from the reporter (§6.1).
func (s *Service) CreateItem(ctx context.Context, a Actor, in NewItem) (*Item, error) {
	if in.Kind != store.KindTicket && in.Kind != store.KindBucket {
		return nil, invalidField("kind", "an item is a ticket or a bucket")
	}
	title, err := validateTitle(in.Title)
	if err != nil {
		return nil, err
	}
	if in.Priority == 0 {
		in.Priority = 3 // P3 is the default (SPEC.md §6.1)
	}
	if err := validatePriority(in.Priority); err != nil {
		return nil, err
	}
	if err := validatePlannedEnd(in.PlannedEnd); err != nil {
		return nil, err
	}
	if in.Kind == store.KindBucket {
		if in.BucketID != 0 {
			// A bucket WILL NOT belong to another bucket (SPEC.md §6.2).
			return nil, invalidField("bucketId", "a bucket cannot go inside another bucket")
		}
		if in.Icon == "" {
			in.Icon = store.DefaultBucketIcon
		}
		if !store.ValidIcon(in.Icon) {
			return nil, invalidField("icon", "that is not one of the bucket icons")
		}
	} else if in.Icon != "" {
		// A ticket carries nothing of the kind (SPEC.md §6.1).
		return nil, invalidField("icon", "a ticket has no icon")
	}

	var out *Item
	err = s.write(ctx, a, func(t *wtx) error {
		if err := s.requireTeamMember(ctx, t.Tx, a, in.TeamID); err != nil {
			return err
		}
		role := store.RoleDefault
		if in.Accepted {
			role = store.RoleAccepted
		}
		st, err := store.WorkStateByRole(ctx, t.Tx, in.TeamID, role)
		if err != nil {
			return err
		}
		it := store.Item{
			Kind: in.Kind, OriginTeamID: in.TeamID, TeamID: in.TeamID,
			Title: title, Description: in.Description, ReporterID: a.UserID,
			AssigneeID: in.AssigneeID, Priority: in.Priority, Icon: in.Icon,
			StateID: st.ID, PlannedEnd: in.PlannedEnd,
			CreatedAt: t.now, UpdatedAt: t.now,
		}
		if in.Accepted {
			// Accepting work starts its clock (SPEC.md §6.1).
			it.ActualStart = t.now
		}
		if in.BucketID != 0 {
			if err := s.checkBucketTarget(ctx, t.Tx, in.BucketID); err != nil {
				return err
			}
			it.BucketID = in.BucketID
		}
		created, err := store.CreateItem(ctx, t.Tx, it)
		if err != nil {
			return err
		}
		if err := t.log(ctx, store.Event{Kind: store.EvItemCreated, Entity: "item",
			EntityID: created.ID, ItemID: created.ID, TeamID: created.TeamID,
			NewValue: created.Title}); err != nil {
			return err
		}
		for _, name := range in.Labels {
			if err := s.attachLabel(ctx, t, created, name); err != nil {
				return err
			}
		}
		full, err := s.loadItem(ctx, t.Tx, created.ID)
		if err != nil {
			return err
		}
		out = full
		return nil
	})
	return out, err
}

// Item is an item with everything its detail view renders.
type Item struct {
	store.Item
	Relations []store.Relation `json:"relations,omitempty"`
	// Progress is a bucket's contents count: items in a terminal state over
	// the total. It is a view, not a field (SPEC.md §6.2).
	Progress *Progress `json:"progress,omitempty"`
}

type Progress struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

func (s *Service) loadItem(ctx context.Context, q store.Q, id int64) (*Item, error) {
	it, err := store.ItemByID(ctx, q, id)
	if err != nil {
		return nil, err
	}
	out := &Item{Item: it}
	items := []store.Item{it}
	if err := store.LoadLabels(ctx, q, items); err != nil {
		return nil, err
	}
	out.Item = items[0]
	if out.Relations, err = store.ItemRelations(ctx, q, id); err != nil {
		return nil, err
	}
	if it.Kind == store.KindBucket {
		p, err := progress(ctx, q, id)
		if err != nil {
			return nil, err
		}
		out.Progress = &p
	}
	if out.Relations == nil {
		out.Relations = []store.Relation{}
	}
	if out.Labels == nil {
		out.Labels = []string{}
	}
	return out, nil
}

// GetItem reads one item. Reading is instance-wide: every authenticated user
// may read every item and comment (SPEC.md §3.2).
func (s *Service) GetItem(ctx context.Context, a Actor, id int64) (*Item, error) {
	if err := requireUser(a); err != nil {
		return nil, err
	}
	it, err := s.loadItem(ctx, s.DB.R, id)
	return it, mapErr(err)
}

// GetItemByIdentifier resolves a permalink such as SOL-42.
func (s *Service) GetItemByIdentifier(ctx context.Context, a Actor, identifier string) (*Item, error) {
	if err := requireUser(a); err != nil {
		return nil, err
	}
	it, err := store.ItemByIdentifier(ctx, s.DB.R, identifier)
	if err != nil {
		return nil, mapErr(err)
	}
	return s.GetItem(ctx, a, it.ID)
}

// UpdateItem applies a patch, writing one event per field that changed.
func (s *Service) UpdateItem(ctx context.Context, a Actor, id int64, p ItemPatch) (*Item, error) {
	var out *Item
	err := s.write(ctx, a, func(t *wtx) error {
		it, err := store.ItemByID(ctx, t.Tx, id)
		if err != nil {
			return err
		}
		if err := s.requireItemEdit(ctx, t.Tx, a, it); err != nil {
			// The one exception: an administrator of the bucket's owning team
			// may remove a ticket from that bucket, and nothing else
			// (SPEC.md §6.4).
			if !isBucketRemoval(p) {
				return err
			}
			ok, cerr := s.canRemoveFromBucket(ctx, t.Tx, a, it)
			if cerr != nil {
				return cerr
			}
			if !ok {
				return err
			}
		}
		if p.Version != 0 && p.Version != it.Version {
			return &Error{Code: CodeConflict, Message: store.ErrConflict.Error()}
		}

		before := it
		if p.Title != nil {
			title, err := validateTitle(*p.Title)
			if err != nil {
				return err
			}
			it.Title = title
		}
		if p.Description != nil {
			it.Description = *p.Description
		}
		if p.AssigneeID != nil {
			if *p.AssigneeID != 0 {
				if _, err := store.UserByID(ctx, t.Tx, *p.AssigneeID); err != nil {
					return err
				}
			}
			it.AssigneeID = *p.AssigneeID
		}
		if p.Priority != nil {
			if err := validatePriority(*p.Priority); err != nil {
				return err
			}
			it.Priority = *p.Priority
		}
		if p.Icon != nil {
			if it.Kind != store.KindBucket {
				return invalidField("icon", "a ticket has no icon")
			}
			if !store.ValidIcon(*p.Icon) {
				return invalidField("icon", "that is not one of the bucket icons")
			}
			it.Icon = *p.Icon
		}
		if p.PlannedEnd != nil {
			if err := validatePlannedEnd(*p.PlannedEnd); err != nil {
				return err
			}
			it.PlannedEnd = *p.PlannedEnd
		}
		if p.BucketID != nil {
			if it.Kind != store.KindTicket {
				return invalidField("bucketId", "a bucket cannot go inside another bucket")
			}
			if *p.BucketID != 0 {
				if err := s.checkBucketTarget(ctx, t.Tx, *p.BucketID); err != nil {
					return err
				}
			}
			it.BucketID = *p.BucketID
		}
		it.UpdatedAt = t.now
		if err := store.UpdateItem(ctx, t.Tx, &it); err != nil {
			return err
		}
		if err := s.logDiff(ctx, t, before, it); err != nil {
			return err
		}
		out, err = s.loadItem(ctx, t.Tx, id)
		return err
	})
	return out, err
}

// isBucketRemoval reports whether a patch does nothing but take a ticket out
// of its bucket.
func isBucketRemoval(p ItemPatch) bool {
	return p.BucketID != nil && *p.BucketID == 0 &&
		p.Title == nil && p.Description == nil && p.AssigneeID == nil &&
		p.Priority == nil && p.Icon == nil && p.PlannedEnd == nil
}

func (s *Service) checkBucketTarget(ctx context.Context, q store.Q, bucketID int64) error {
	b, err := store.ItemByID(ctx, q, bucketID)
	if err != nil {
		return err
	}
	if b.Kind != store.KindBucket {
		return invalidField("bucketId", "%s is not a bucket", b.Identifier)
	}
	// A bucket WILL accept tickets from any team. This is how cross-team
	// milestones work (SPEC.md §6.2), so there is no team check here.
	return nil
}

func (s *Service) logDiff(ctx context.Context, t *wtx, before, after store.Item) error {
	type change struct{ field, old, new string }
	var changes []change
	add := func(field, old, new string) {
		if old != new {
			changes = append(changes, change{field, old, new})
		}
	}
	add("title", before.Title, after.Title)
	if before.Description != after.Description {
		// Prose is not worth two copies in the log; that it changed is the
		// fact the log is answering.
		changes = append(changes, change{"description", "", ""})
	}
	add("assignee", strconv.FormatInt(before.AssigneeID, 10), strconv.FormatInt(after.AssigneeID, 10))
	add("priority", "P"+strconv.FormatInt(before.Priority, 10), "P"+strconv.FormatInt(after.Priority, 10))
	add("icon", before.Icon, after.Icon)
	add("bucket", strconv.FormatInt(before.BucketID, 10), strconv.FormatInt(after.BucketID, 10))
	add("plannedEnd", before.PlannedEnd, after.PlannedEnd)
	add("team", strconv.FormatInt(before.TeamID, 10), strconv.FormatInt(after.TeamID, 10))
	for _, c := range changes {
		if err := t.logField(ctx, after, c.field, c.old, c.new); err != nil {
			return err
		}
	}
	return nil
}

package core

import (
	"context"

	"github.com/33313/campfire/internal/store"
)

// A bulk edit gives a selection one new value for one field, in one action
// (SPEC.md §6.5). It is a loop over one operation, not a second write path:
// every item goes through the same handler and the same permission check as
// the single-item edit.
//
// One field per action. A bulk edit is not a form over several fields at once:
// a selection is where a mis-click is least visible.

// The four fields a selection can take (SPEC.md §6.5). Title, description,
// planned end and labels are deferred (§3.3) and are not here.
const (
	BulkState    = "state"
	BulkAssignee = "assignee"
	BulkPriority = "priority"
	BulkBucket   = "bucket"
)

type BulkEdit struct {
	ItemIDs []int64 `json:"itemIds"`
	Field   string  `json:"field"`
	// Value is the new value: a state id, a user id, a priority, or a bucket
	// id. Zero clears the assignee or removes the bucket.
	Value int64 `json:"value"`
	// Reason applies to the whole selection when the target is the rejected
	// state.
	Reason string `json:"reason,omitempty"`
	Note   string `json:"note,omitempty"`
}

type BulkRefusal struct {
	ItemID int64  `json:"itemId"`
	Reason string `json:"reason"`
}

type BulkResult struct {
	Changed []*Item       `json:"changed"`
	Refused []BulkRefusal `json:"refused"`
}

// ApplyBulkEdit runs the edit over the selection, one item at a time.
//
// Each item is its own transaction, because an item the actor may not edit
// MUST be refused on its own and MUST NOT abort the rest. What was refused is
// returned for the interface to report.
//
// Every item touched writes its own event. A bulk operation writes no summary
// event: a log that summarises is a log that cannot answer what happened to
// one ticket.
func (s *Service) ApplyBulkEdit(ctx context.Context, a Actor, in BulkEdit) (*BulkResult, error) {
	if err := requireUser(a); err != nil {
		return nil, err
	}
	if len(in.ItemIDs) == 0 {
		return nil, invalidField("itemIds", "select something first")
	}
	if len(in.ItemIDs) > 500 {
		return nil, invalidField("itemIds", "at most 500 items at a time")
	}

	out := &BulkResult{}
	for _, id := range in.ItemIDs {
		var it *Item
		var err error
		switch in.Field {
		case BulkState:
			it, err = s.MoveItem(ctx, a, id, in.Value, in.Reason, in.Note)
		case BulkAssignee:
			v := in.Value
			it, err = s.UpdateItem(ctx, a, id, ItemPatch{AssigneeID: &v})
		case BulkPriority:
			v := in.Value
			it, err = s.UpdateItem(ctx, a, id, ItemPatch{Priority: &v})
		case BulkBucket:
			v := in.Value
			it, err = s.UpdateItem(ctx, a, id, ItemPatch{BucketID: &v})
		default:
			return nil, invalidField("field", "a bulk edit changes state, assignee, priority or bucket")
		}
		if err != nil {
			out.Refused = append(out.Refused, BulkRefusal{ItemID: id, Reason: reasonOf(err)})
			continue
		}
		out.Changed = append(out.Changed, it)
	}
	return out, nil
}

func reasonOf(err error) string {
	if e, ok := err.(*Error); ok {
		return e.Message
	}
	return err.Error()
}

// DeleteItem hard-deletes an item.
//
// This is an instance-administrator operation, not the disposal route:
// rejection with reason `cancelled` is (SPEC.md §6.4). Buckets are never
// hard-deleted through the ordinary interface, and a deletion must state how
// many items across how many teams it affects — which is what BucketImpact is
// for, and which the client shows before calling this.
func (s *Service) DeleteItem(ctx context.Context, a Actor, itemID int64) error {
	if err := RequireInstanceAdmin(a); err != nil {
		return err
	}
	return s.write(ctx, a, func(t *wtx) error {
		it, err := store.ItemByID(ctx, t.Tx, itemID)
		if err != nil {
			return err
		}
		// The event is written first: the row it points at is about to go, and
		// the log is append-only, so this is the only record that will remain.
		if err := t.log(ctx, store.Event{Kind: store.EvItemDeleted, Entity: "item",
			EntityID: it.ID, TeamID: it.TeamID, OldValue: it.Identifier + " " + it.Title}); err != nil {
			return err
		}
		return store.DeleteItem(ctx, t.Tx, itemID)
	})
}

// ItemImpact reports what deleting a bucket would affect: how many items,
// across how many teams (SPEC.md §6.4).
func (s *Service) ItemImpact(ctx context.Context, a Actor, itemID int64) (items, teams int, err error) {
	if err := requireUser(a); err != nil {
		return 0, 0, err
	}
	items, teams, err = store.BucketImpact(ctx, s.DB.R, itemID)
	return items, teams, mapErr(err)
}

// ReassignBucket moves a bucket to another owning team.
//
// A milestone must survive the dissolution of the team that planned it, so an
// instance administrator may reassign one. The bucket keeps its identifier:
// the prefix records origin, not current ownership (SPEC.md §6.3). A ticket's
// owning team MUST NOT change, so this refuses anything but a bucket.
func (s *Service) ReassignBucket(ctx context.Context, a Actor, bucketID, teamID int64) (*Item, error) {
	if err := RequireInstanceAdmin(a); err != nil {
		return nil, err
	}
	var out *Item
	err := s.write(ctx, a, func(t *wtx) error {
		it, err := store.ItemByID(ctx, t.Tx, bucketID)
		if err != nil {
			return err
		}
		if it.Kind != store.KindBucket {
			return invalidField("bucketId", "a ticket's owning team cannot change")
		}
		if it.TeamID == teamID {
			return nil
		}
		// Buckets use their owning team's work states, so the state has to
		// come across too. The role is preserved, not the state row.
		from, err := store.WorkStateByID(ctx, t.Tx, it.StateID)
		if err != nil {
			return err
		}
		target, err := s.matchingState(ctx, t, teamID, from)
		if err != nil {
			return err
		}
		before := it
		it.TeamID, it.StateID = teamID, target.ID
		if it.Rank, err = store.RankAtEnd(ctx, t.Tx, target.ID); err != nil {
			return err
		}
		it.UpdatedAt = t.now
		if err := store.UpdateItem(ctx, t.Tx, &it); err != nil {
			return err
		}
		if err := s.logDiff(ctx, t, before, it); err != nil {
			return err
		}
		out, err = s.loadItem(ctx, t.Tx, bucketID)
		return err
	})
	return out, err
}

// matchingState finds the state in the new team that means what the old one
// meant: the same role, or the default state when the old one had none.
func (s *Service) matchingState(ctx context.Context, t *wtx, teamID int64, from store.WorkState) (store.WorkState, error) {
	role := from.Role
	if role == "" {
		role = store.RoleDefault
	}
	return store.WorkStateByRole(ctx, t.Tx, teamID, role)
}

package core

import (
	"context"
	"strings"

	"github.com/33313/campfire/internal/store"
)

// Any item may move to any state at any time. The application enforces no
// transition graph, no guard rules and no per-state permissions: enforcement
// is the most common source of friction in trackers of this kind, and the
// audit log already records who moved what (SPEC.md §7.3, Appendix B).

// MoveItem changes an item's work state.
//
// A move to the rejected state requires a reason. Entering a terminal state
// sets actual end; leaving one clears it. The first move out of the default
// state sets actual start, and it is never set twice.
func (s *Service) MoveItem(ctx context.Context, a Actor, itemID, stateID int64, reason, note string) (*Item, error) {
	var out *Item
	err := s.write(ctx, a, func(t *wtx) error {
		it, err := store.ItemByID(ctx, t.Tx, itemID)
		if err != nil {
			return err
		}
		if err := s.requireItemEdit(ctx, t.Tx, a, it); err != nil {
			return err
		}
		if err := s.moveItem(ctx, t, &it, stateID, reason, note); err != nil {
			return err
		}
		out, err = s.loadItem(ctx, t.Tx, itemID)
		return err
	})
	return out, err
}

// moveItem is the transition itself, shared by the single move, the bulk move
// and the rejection a split can perform on its source. There is one lifecycle
// mechanism, so there is one implementation of it.
func (s *Service) moveItem(ctx context.Context, t *wtx, it *store.Item, stateID int64, reason, note string) error {
	target, err := store.WorkStateByID(ctx, t.Tx, stateID)
	if err != nil {
		return err
	}
	if target.TeamID != it.TeamID {
		return invalidField("stateId", "that work state belongs to another team")
	}
	from, err := store.WorkStateByID(ctx, t.Tx, it.StateID)
	if err != nil {
		return err
	}
	if target.Role == store.RoleRejected {
		if reason != store.ReasonCancelled && reason != store.ReasonDuplicate {
			return invalidField("reason", "rejecting needs a reason: cancelled or duplicate")
		}
	} else {
		reason, note = "", ""
	}
	if target.ID == from.ID && reason == it.RejectReason && note == it.RejectNote {
		return nil
	}

	it.StateID = target.ID
	it.RejectReason, it.RejectNote = reason, strings.TrimSpace(note)
	// The first move out of the default state starts the clock, and only the
	// first: actual start records when work began, not when it last resumed.
	if from.Role == store.RoleDefault && target.Role != store.RoleDefault && it.ActualStart == 0 {
		it.ActualStart = t.now
	}
	switch {
	case target.Terminal():
		it.ActualEnd = t.now
	case from.Terminal():
		it.ActualEnd = 0
	}
	// The item lands at the end of its new column. A drag that both moves and
	// positions calls RankItem afterwards.
	if target.ID != from.ID {
		rank, err := store.RankAtEnd(ctx, t.Tx, target.ID)
		if err != nil {
			return err
		}
		it.Rank = rank
	}
	it.UpdatedAt = t.now
	if err := store.UpdateItem(ctx, t.Tx, it); err != nil {
		return err
	}
	newValue := target.Name
	if reason != "" {
		newValue = target.Name + " (" + reason + ")"
	}
	return t.log(ctx, store.Event{Kind: store.EvItemStateChanged, Entity: "item",
		EntityID: it.ID, ItemID: it.ID, TeamID: it.TeamID, Field: "state",
		OldValue: from.Name, NewValue: newValue})
}

// RankItem places an item between two neighbours in a column.
//
// The rank is a short string sorting between theirs, so a move writes one row
// (SPEC.md §9.3). Two clients can compute the same midpoint from the same
// read, which the UNIQUE (state_id, rank) index turns into a retry rather than
// a collision.
func (s *Service) RankItem(ctx context.Context, a Actor, itemID, stateID, afterID, beforeID int64) (*Item, error) {
	var out *Item
	err := s.write(ctx, a, func(t *wtx) error {
		it, err := store.ItemByID(ctx, t.Tx, itemID)
		if err != nil {
			return err
		}
		if err := s.requireItemEdit(ctx, t.Tx, a, it); err != nil {
			return err
		}
		if stateID != 0 && stateID != it.StateID {
			if err := s.moveItem(ctx, t, &it, stateID, "", ""); err != nil {
				return err
			}
		}
		rank, err := store.RankBetweenItems(ctx, t.Tx, it.StateID, afterID, beforeID)
		if err != nil {
			return errf(CodeInvalid, "%s", err.Error())
		}
		it.Rank, it.UpdatedAt = rank, t.now
		if err := store.UpdateItem(ctx, t.Tx, &it); err != nil {
			if !store.IsUniqueViolation(err) {
				return err
			}
			// Someone took this midpoint between the read and the write.
			// Recompute against what is actually there and try once more; the
			// second attempt cannot collide, because the row that won is now
			// the neighbour being measured from.
			if it.Rank, err = store.RankAfterExisting(ctx, t.Tx, it.StateID, afterID); err != nil {
				return err
			}
			if err := store.UpdateItem(ctx, t.Tx, &it); err != nil {
				return err
			}
		}
		out, err = s.loadItem(ctx, t.Tx, itemID)
		return err
	})
	return out, err
}

// Clone turns one ticket into two.
//
// The new ticket takes a fresh identifier from its team's counter and copies
// title, description, priority, assignee, bucket and labels. It enters the
// default state, its reporter is the cloning user, and it carries a "relates
// to" relation back to the ticket it came from — provenance is a relation
// rather than a new field, because a relation is already stored, already
// displayed from both ends, and already filterable (SPEC.md §6.5).
func (s *Service) Clone(ctx context.Context, a Actor, itemID int64) (*Item, error) {
	var out *Item
	err := s.write(ctx, a, func(t *wtx) error {
		src, err := store.ItemByID(ctx, t.Tx, itemID)
		if err != nil {
			return err
		}
		if src.Kind != store.KindTicket {
			return invalidField("itemId", "only a ticket can be cloned")
		}
		if err := s.requireItemEdit(ctx, t.Tx, a, src); err != nil {
			return err
		}
		created, err := s.derive(ctx, t, src, src.Title, store.RelRelatesTo)
		if err != nil {
			return err
		}
		out, err = s.loadItem(ctx, t.Tx, created.ID)
		return err
	})
	return out, err
}

// SplitResult is what a split produced, so the interface can link straight to
// the new tickets.
type SplitResult struct {
	Source *Item   `json:"source"`
	New    []*Item `json:"new"`
}

// Split turns one ticket into several.
//
// Each new ticket is created the way a clone is but carries a "follows up"
// relation to the source, and a "relates to" to each of the others the split
// made (DEV-94). The source keeps its state unless the actor also
// rejects it in the same action, in which case the reason is `cancelled`.
//
// A split never deletes or absorbs the source: deletion is not a state, and
// the tickets that replaced it are reachable from it (SPEC.md §6.5).
func (s *Service) Split(ctx context.Context, a Actor, itemID int64, titles []string, rejectSource bool) (*SplitResult, error) {
	if len(titles) == 0 {
		return nil, invalidField("titles", "a split needs at least one new title")
	}
	var out *SplitResult
	err := s.write(ctx, a, func(t *wtx) error {
		src, err := store.ItemByID(ctx, t.Tx, itemID)
		if err != nil {
			return err
		}
		if src.Kind != store.KindTicket {
			return invalidField("itemId", "only a ticket can be split")
		}
		if err := s.requireItemEdit(ctx, t.Tx, a, src); err != nil {
			return err
		}
		res := &SplitResult{}
		var made []store.Item
		for _, title := range titles {
			title, err := validateTitle(title)
			if err != nil {
				return err
			}
			// One item at a time: each allocates its identifier atomically and
			// a batch never reserves a range (SPEC.md §6.5).
			created, err := s.derive(ctx, t, src, title, store.RelFollowsUp)
			if err != nil {
				return err
			}
			for _, sibling := range made {
				if err := s.addRelation(ctx, t, sibling, created, store.RelRelatesTo); err != nil {
					return err
				}
			}
			made = append(made, created)
		}
		for _, created := range made {
			full, err := s.loadItem(ctx, t.Tx, created.ID)
			if err != nil {
				return err
			}
			res.New = append(res.New, full)
		}
		if rejectSource {
			rejected, err := store.WorkStateByRole(ctx, t.Tx, src.TeamID, store.RoleRejected)
			if err != nil {
				return err
			}
			if err := s.moveItem(ctx, t, &src, rejected.ID, store.ReasonCancelled, ""); err != nil {
				return err
			}
		}
		if res.Source, err = s.loadItem(ctx, t.Tx, itemID); err != nil {
			return err
		}
		out = res
		return nil
	})
	return out, err
}

// derive creates the new ticket a clone or a split produces, and the one edge
// back to its source.
//
// A derived item MUST NOT copy the source's comments, history, relations,
// actual start or actual end: the edge back to the source is the only relation
// it is created with (SPEC.md §6.5), besides a split's edges to its siblings.
func (s *Service) derive(ctx context.Context, t *wtx, src store.Item, title, relation string) (store.Item, error) {
	def, err := store.WorkStateByRole(ctx, t.Tx, src.TeamID, store.RoleDefault)
	if err != nil {
		return store.Item{}, err
	}
	created, err := store.CreateItem(ctx, t.Tx, store.Item{
		Kind: store.KindTicket, OriginTeamID: src.TeamID, TeamID: src.TeamID,
		Title: title, Description: src.Description, ReporterID: t.actor.UserID,
		AssigneeID: src.AssigneeID, Priority: src.Priority, BucketID: src.BucketID,
		StateID: def.ID, CreatedAt: t.now, UpdatedAt: t.now,
	})
	if err != nil {
		return store.Item{}, err
	}
	if err := t.log(ctx, store.Event{Kind: store.EvItemCreated, Entity: "item",
		EntityID: created.ID, ItemID: created.ID, TeamID: created.TeamID,
		NewValue: created.Title, Field: relationField(relation), OldValue: src.Identifier}); err != nil {
		return store.Item{}, err
	}
	labels, err := store.ItemLabels(ctx, t.Tx, src.ID)
	if err != nil {
		return store.Item{}, err
	}
	for _, l := range labels {
		if err := store.AddItemLabel(ctx, t.Tx, created.ID, l.ID); err != nil {
			return store.Item{}, err
		}
	}
	if err := s.addRelation(ctx, t, created, src, relation); err != nil {
		return store.Item{}, err
	}
	return created, nil
}

func relationField(relation string) string {
	if relation == store.RelFollowsUp {
		return "split_from"
	}
	return "cloned_from"
}

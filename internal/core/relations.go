package core

import (
	"context"

	"github.com/campfire/campfire/internal/store"
)

// A fixed set of relation types; users cannot define new ones (SPEC.md §8.2).
// Relations may cross teams and may link a ticket to a bucket.
//
// A relation carries no lifecycle meaning: a "blocked by" edge is information
// and MUST NOT prevent any transition. Nothing in this file consults one.

func (s *Service) AddRelation(ctx context.Context, a Actor, fromID, toID int64, kind string) (*Item, error) {
	if !store.ValidRelationKind(kind) {
		return nil, invalidField("kind", "that is not one of the relation types")
	}
	if fromID == toID {
		return nil, invalidField("toId", "an item cannot relate to itself")
	}
	var out *Item
	err := s.write(ctx, a, func(t *wtx) error {
		from, err := store.ItemByID(ctx, t.Tx, fromID)
		if err != nil {
			return err
		}
		to, err := store.ItemByID(ctx, t.Tx, toID)
		if err != nil {
			return err
		}
		// The edge is written on the "from" item, so that is the item whose
		// edit permission governs it.
		if err := s.requireItemEdit(ctx, t.Tx, a, from); err != nil {
			return err
		}
		if err := s.addRelation(ctx, t, from, to, kind); err != nil {
			return err
		}
		out, err = s.loadItem(ctx, t.Tx, fromID)
		return err
	})
	return out, err
}

func (s *Service) addRelation(ctx context.Context, t *wtx, from, to store.Item, kind string) error {
	if _, err := store.AddRelation(ctx, t.Tx, from.ID, to.ID, kind, t.actor.UserID, t.now); err != nil {
		if store.IsUniqueViolation(err) {
			return nil // the edge is already there; saying so twice is not an error
		}
		return err
	}
	// Stored once, displayed from both ends — so one event, on the item the
	// edge was added from.
	return t.log(ctx, store.Event{Kind: store.EvRelationAdded, Entity: "item",
		EntityID: from.ID, ItemID: from.ID, TeamID: from.TeamID,
		Field: kind, NewValue: to.Identifier})
}

func (s *Service) RemoveRelation(ctx context.Context, a Actor, relationID int64) error {
	return s.write(ctx, a, func(t *wtx) error {
		fromID, toID, kind, err := store.RelationByID(ctx, t.Tx, relationID)
		if err != nil {
			return err
		}
		from, err := store.ItemByID(ctx, t.Tx, fromID)
		if err != nil {
			return err
		}
		to, err := store.ItemByID(ctx, t.Tx, toID)
		if err != nil {
			return err
		}
		// Either end may cut the edge: it is displayed from both, and a
		// relation nobody on one side can remove is a relation that outlives
		// its usefulness.
		if err := s.requireItemEdit(ctx, t.Tx, a, from); err != nil {
			if err2 := s.requireItemEdit(ctx, t.Tx, a, to); err2 != nil {
				return err
			}
		}
		if err := store.RemoveRelation(ctx, t.Tx, relationID); err != nil {
			return err
		}
		return t.log(ctx, store.Event{Kind: store.EvRelationRemoved, Entity: "item",
			EntityID: from.ID, ItemID: from.ID, TeamID: from.TeamID,
			Field: kind, OldValue: to.Identifier})
	})
}

package core

import (
	"context"
	"strings"

	"github.com/campfire/campfire/internal/store"
)

// Labels are free text, created by typing them, and never administered
// (SPEC.md §8.1). They appear on an item's detail view only: a card carries
// identifier, title, priority, assignee and bucket, and cards carrying labels
// become unreadable.

const maxLabel = 60

func validateLabel(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", invalidField("label", "a label needs some text")
	}
	if len([]rune(name)) > maxLabel {
		return "", invalidField("label", "a label is at most %d characters", maxLabel)
	}
	return name, nil
}

func (s *Service) attachLabel(ctx context.Context, t *wtx, it store.Item, name string) error {
	name, err := validateLabel(name)
	if err != nil {
		return err
	}
	l, err := store.EnsureLabel(ctx, t.Tx, name, t.now)
	if err != nil {
		return err
	}
	if err := store.AddItemLabel(ctx, t.Tx, it.ID, l.ID); err != nil {
		return err
	}
	return t.log(ctx, store.Event{Kind: store.EvLabelAdded, Entity: "item",
		EntityID: it.ID, ItemID: it.ID, TeamID: it.TeamID, Field: "label", NewValue: l.Name})
}

func (s *Service) AddLabel(ctx context.Context, a Actor, itemID int64, name string) (*Item, error) {
	var out *Item
	err := s.write(ctx, a, func(t *wtx) error {
		it, err := store.ItemByID(ctx, t.Tx, itemID)
		if err != nil {
			return err
		}
		if err := s.requireItemEdit(ctx, t.Tx, a, it); err != nil {
			return err
		}
		if err := s.attachLabel(ctx, t, it, name); err != nil {
			return err
		}
		out, err = s.loadItem(ctx, t.Tx, itemID)
		return err
	})
	return out, err
}

func (s *Service) RemoveLabel(ctx context.Context, a Actor, itemID int64, name string) (*Item, error) {
	var out *Item
	err := s.write(ctx, a, func(t *wtx) error {
		it, err := store.ItemByID(ctx, t.Tx, itemID)
		if err != nil {
			return err
		}
		if err := s.requireItemEdit(ctx, t.Tx, a, it); err != nil {
			return err
		}
		l, err := store.LabelByName(ctx, t.Tx, name)
		if err != nil {
			return err
		}
		if err := store.RemoveItemLabel(ctx, t.Tx, itemID, l.ID); err != nil {
			return err
		}
		if err := t.log(ctx, store.Event{Kind: store.EvLabelRemoved, Entity: "item",
			EntityID: itemID, ItemID: itemID, TeamID: it.TeamID,
			Field: "label", OldValue: l.Name}); err != nil {
			return err
		}
		out, err = s.loadItem(ctx, t.Tx, itemID)
		return err
	})
	return out, err
}

// ListLabels is what the filter bar's picker offers.
func (s *Service) ListLabels(ctx context.Context, a Actor) ([]store.Label, error) {
	if err := requireUser(a); err != nil {
		return nil, err
	}
	labels, err := store.ListLabels(ctx, s.DB.R)
	return list(labels, mapErr(err))
}

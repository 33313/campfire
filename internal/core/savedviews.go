package core

import (
	"context"
	"strconv"
	"strings"

	"github.com/campfire/campfire/internal/store"
)

// A saved view is a name attached to a filter the application can already
// express (SPEC.md §9.4). It introduces no query language, no new filter
// dimension and no new permission concept.
//
// Ownership is one team, set at creation. Creating, editing and deleting are
// ordinary edit permission — any member or administrator of the owning team —
// because naming a filter is not an administrative act, and a creator-based
// rule would strand every view whose author leaves. Every authenticated user
// may read and apply any view: reading is instance-wide.

type ViewInput struct {
	TeamID     int64          `json:"teamId"`
	Name       string         `json:"name"`
	PinBoard   bool           `json:"pinBoard"`
	PinBacklog bool           `json:"pinBacklog"`
	Clauses    []store.Clause `json:"clauses"`
}

func validateClauses(clauses []store.Clause) error {
	for _, c := range clauses {
		if !store.ValidDimension(c.Dimension) {
			return invalidField("clauses", "%q is not a filter dimension", c.Dimension)
		}
		switch c.Dimension {
		case store.DimText:
			if strings.TrimSpace(c.Text) == "" {
				return invalidField("clauses", "a text clause needs some text")
			}
		case store.DimPriority:
			if c.RefID < 1 || c.RefID > 4 {
				return invalidField("clauses", "priority is P1 to P4")
			}
		case store.DimAssignee:
			// Zero is a real value of this dimension — unassigned — and is the
			// one the filter bar's "Unassigned" chip saves.
			if c.RefID < 0 {
				return invalidField("clauses", "an assignee clause needs something to point at")
			}
		default:
			if c.RefID == 0 {
				return invalidField("clauses", "a %s clause needs something to point at", c.Dimension)
			}
		}
	}
	return nil
}

func (s *Service) CreateSavedView(ctx context.Context, a Actor, in ViewInput) (*store.SavedView, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, invalidField("name", "a saved view needs a name")
	}
	if err := validateClauses(in.Clauses); err != nil {
		return nil, err
	}
	var out *store.SavedView
	err := s.write(ctx, a, func(t *wtx) error {
		if err := s.requireTeamMember(ctx, t.Tx, a, in.TeamID); err != nil {
			return err
		}
		id, err := store.CreateSavedView(ctx, t.Tx, store.SavedView{
			TeamID: in.TeamID, Name: name, PinBoard: in.PinBoard, PinBacklog: in.PinBacklog,
			CreatedAt: t.now, UpdatedAt: t.now})
		if err != nil {
			return err
		}
		if err := store.ReplaceClauses(ctx, t.Tx, id, in.Clauses); err != nil {
			return err
		}
		v, err := store.SavedViewByID(ctx, t.Tx, id)
		if err != nil {
			return err
		}
		out = &v
		return t.log(ctx, store.Event{Kind: store.EvViewCreated, Entity: "saved_view",
			EntityID: id, TeamID: in.TeamID, NewValue: name})
	})
	return out, err
}

// UpdateSavedView rewrites a view. Every mutation writes an event: creation,
// rename, clause change, pinning and deletion (SPEC.md §9.4).
func (s *Service) UpdateSavedView(ctx context.Context, a Actor, id int64, in ViewInput) (*store.SavedView, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, invalidField("name", "a saved view needs a name")
	}
	if err := validateClauses(in.Clauses); err != nil {
		return nil, err
	}
	var out *store.SavedView
	err := s.write(ctx, a, func(t *wtx) error {
		v, err := store.SavedViewByID(ctx, t.Tx, id)
		if err != nil {
			return err
		}
		if err := s.requireTeamMember(ctx, t.Tx, a, v.TeamID); err != nil {
			return err
		}
		before := v
		v.Name, v.PinBoard, v.PinBacklog, v.UpdatedAt = name, in.PinBoard, in.PinBacklog, t.now
		if err := store.UpdateSavedView(ctx, t.Tx, v); err != nil {
			return err
		}
		if err := store.ReplaceClauses(ctx, t.Tx, id, in.Clauses); err != nil {
			return err
		}
		if before.Name != name {
			if err := t.log(ctx, store.Event{Kind: store.EvViewUpdated, Entity: "saved_view",
				EntityID: id, TeamID: v.TeamID, Field: "name",
				OldValue: before.Name, NewValue: name}); err != nil {
				return err
			}
		}
		if before.PinBoard != in.PinBoard || before.PinBacklog != in.PinBacklog {
			if err := t.log(ctx, store.Event{Kind: store.EvViewUpdated, Entity: "saved_view",
				EntityID: id, TeamID: v.TeamID, Field: "pinning",
				NewValue: pinLabel(in.PinBoard, in.PinBacklog)}); err != nil {
				return err
			}
		}
		if err := t.log(ctx, store.Event{Kind: store.EvViewUpdated, Entity: "saved_view",
			EntityID: id, TeamID: v.TeamID, Field: "clauses",
			NewValue: strconv.Itoa(len(in.Clauses))}); err != nil {
			return err
		}
		updated, err := store.SavedViewByID(ctx, t.Tx, id)
		if err != nil {
			return err
		}
		out = &updated
		return nil
	})
	return out, err
}

func pinLabel(board, backlog bool) string {
	switch {
	case board && backlog:
		return "board and backlog"
	case board:
		return "board"
	case backlog:
		return "backlog"
	}
	return "neither"
}

func (s *Service) DeleteSavedView(ctx context.Context, a Actor, id int64) error {
	return s.write(ctx, a, func(t *wtx) error {
		v, err := store.SavedViewByID(ctx, t.Tx, id)
		if err != nil {
			return err
		}
		if err := s.requireTeamMember(ctx, t.Tx, a, v.TeamID); err != nil {
			return err
		}
		if err := store.DeleteSavedView(ctx, t.Tx, id); err != nil {
			return err
		}
		return t.log(ctx, store.Event{Kind: store.EvViewDeleted, Entity: "saved_view",
			EntityID: id, TeamID: v.TeamID, OldValue: v.Name})
	})
}

func (s *Service) ListSavedViews(ctx context.Context, a Actor, teamID int64) ([]store.SavedView, error) {
	if err := requireUser(a); err != nil {
		return nil, err
	}
	v, err := store.ListSavedViews(ctx, s.DB.R, teamID)
	return list(v, mapErr(err))
}

// SavedViewFilter turns a stored view into the filter the list endpoints take.
//
// A view owned by SOL may filter on TERRA's tickets: the owning team governs
// where the view is listed, not what it may match (SPEC.md §9.4).
func (s *Service) SavedViewFilter(ctx context.Context, a Actor, id int64) (Filter, error) {
	if err := requireUser(a); err != nil {
		return Filter{}, err
	}
	v, err := store.SavedViewByID(ctx, s.DB.R, id)
	if err != nil {
		return Filter{}, mapErr(err)
	}
	sf := v.AsFilter()
	return Filter{
		StateIDs: sf.StateIDs, LabelIDs: sf.LabelIDs, AssigneeIDs: sf.AssigneeIDs,
		ReporterIDs: sf.ReporterIDs, BucketIDs: sf.BucketIDs, Priorities: sf.Priorities,
		TeamIDs: sf.TeamIDs, Text: sf.Text,
	}, nil
}

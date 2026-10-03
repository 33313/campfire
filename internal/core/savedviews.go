package core

import (
	"context"
	"strings"
	"time"

	"github.com/33313/campfire/internal/csql"
	"github.com/33313/campfire/internal/store"
)

// A saved view is a name attached to a CSQL query (SPEC.md §9.4). It
// introduces no new filter dimension and no new permission concept.
//
// Ownership is one team, set at creation. Creating, editing and deleting are
// ordinary edit permission — any member or administrator of the owning team —
// because naming a filter is not an administrative act, and a creator-based
// rule would strand every view whose author leaves. Every authenticated user
// may read and apply any view: reading is instance-wide.

type ViewInput struct {
	TeamID     int64  `json:"teamId"`
	Name       string `json:"name"`
	PinBoard   bool   `json:"pinBoard"`
	PinBacklog bool   `json:"pinBacklog"`
	Query      string `json:"query"`
}

// viewQuery is a view's query as stored: canonical, its referents by #id, and
// those referents; and as the log records it, written as it was typed.
func (s *Service) viewQuery(ctx context.Context, a Actor, q string) (canonical, typed string, refs []store.Ref, err error) {
	n, err := s.parseQuery(ctx, a, q, time.UTC) // days stay relative when stored
	if err != nil {
		return "", "", nil, err
	}
	for _, r := range csql.Refs(n) {
		refs = append(refs, store.Ref{Field: r.Field, ID: r.ID})
	}
	typed = csql.Format(n, nil)
	return csql.Canonical(n), typed, refs, nil
}

func (s *Service) CreateSavedView(ctx context.Context, a Actor, in ViewInput) (*store.SavedView, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, invalidField("name", "a saved view needs a name")
	}
	query, _, refs, err := s.viewQuery(ctx, a, in.Query)
	if err != nil {
		return nil, err
	}
	var out *store.SavedView
	err = s.write(ctx, a, func(t *wtx) error {
		if err := s.requireTeamMember(ctx, t.Tx, a, in.TeamID); err != nil {
			return err
		}
		id, err := store.CreateSavedView(ctx, t.Tx, store.SavedView{
			TeamID: in.TeamID, Name: name, PinBoard: in.PinBoard, PinBacklog: in.PinBacklog,
			Query: query, CreatedAt: t.now, UpdatedAt: t.now})
		if err != nil {
			return err
		}
		if err := store.ReplaceRefs(ctx, t.Tx, id, refs); err != nil {
			return err
		}
		v, err := store.SavedViewByID(ctx, t.Tx, id)
		if err != nil {
			return err
		}
		humanize(&v)
		out = &v
		return t.log(ctx, store.Event{Kind: store.EvViewCreated, Entity: "saved_view",
			EntityID: id, TeamID: in.TeamID, NewValue: name})
	})
	return out, err
}

// UpdateSavedView rewrites a view. Every mutation writes an event: creation,
// rename, query change, pinning and deletion (SPEC.md §9.4).
func (s *Service) UpdateSavedView(ctx context.Context, a Actor, id int64, in ViewInput) (*store.SavedView, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, invalidField("name", "a saved view needs a name")
	}
	query, typed, refs, err := s.viewQuery(ctx, a, in.Query)
	if err != nil {
		return nil, err
	}
	var out *store.SavedView
	err = s.write(ctx, a, func(t *wtx) error {
		v, err := store.SavedViewByID(ctx, t.Tx, id)
		if err != nil {
			return err
		}
		if err := s.requireTeamMember(ctx, t.Tx, a, v.TeamID); err != nil {
			return err
		}
		before := v
		v.Name, v.PinBoard, v.PinBacklog, v.Query, v.UpdatedAt = name, in.PinBoard, in.PinBacklog, query, t.now
		if err := store.UpdateSavedView(ctx, t.Tx, v); err != nil {
			return err
		}
		if err := store.ReplaceRefs(ctx, t.Tx, id, refs); err != nil {
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
		if before.Query != query {
			old := before
			humanize(&old)
			if err := t.log(ctx, store.Event{Kind: store.EvViewUpdated, Entity: "saved_view",
				EntityID: id, TeamID: v.TeamID, Field: "query",
				OldValue: old.Query, NewValue: typed}); err != nil {
				return err
			}
		}
		updated, err := store.SavedViewByID(ctx, t.Tx, id)
		if err != nil {
			return err
		}
		humanize(&updated)
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
	for i := range v {
		humanize(&v[i])
	}
	return list(v, mapErr(err))
}

// SavedViewFilter is the filter a stored view applies: its query, which
// reads `me` as whoever applies it.
//
// A view owned by SOL may filter on TERRA's tickets: the owning team governs
// where the view is listed, not what it may match (SPEC.md §9.4).
func (s *Service) SavedViewFilter(ctx context.Context, a Actor, id int64) (Filter, error) {
	if err := requireUser(a); err != nil {
		return Filter{}, err
	}
	if _, err := store.SavedViewByID(ctx, s.DB.R, id); err != nil {
		return Filter{}, mapErr(err)
	}
	return Filter{ViewIDs: []int64{id}}, nil
}

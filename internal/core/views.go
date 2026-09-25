package core

import (
	"context"
	"slices"

	"github.com/campfire/campfire/internal/store"
)

// The board and the backlog are the same state model seen twice
// (SPEC.md §7.2). The board shows every state except the default one and the
// rejected one; the backlog shows the default state. Rejected items reach
// neither and are found by filter or by permalink.
//
// Backlog is a consequence of the state model, not a separate feature, so
// there is one query behind both.

type Column struct {
	State store.WorkState `json:"state"`
	Items []store.Item    `json:"items"`
	// Count is the column total, which is what the header's count box shows —
	// not the length of this page.
	Count  int    `json:"count"`
	Cursor string `json:"cursor,omitempty"`
}

type Board struct {
	Team    store.Team `json:"team"`
	Columns []Column   `json:"columns"`
}

// Filter is the structured filter of SPEC.md §9.2 as an interface takes it.
// There is no query language: these are the dimensions, and a saved view is
// the same thing under a name.
type Filter struct {
	StateIDs    []int64  `json:"stateIds,omitempty"`
	LabelIDs    []int64  `json:"labelIds,omitempty"`
	AssigneeIDs []int64  `json:"assigneeIds,omitempty"`
	Unassigned  bool     `json:"unassigned,omitempty"`
	ReporterIDs []int64  `json:"reporterIds,omitempty"`
	BucketIDs   []int64  `json:"bucketIds,omitempty"`
	Priorities  []int64  `json:"priorities,omitempty"`
	TeamIDs     []int64  `json:"teamIds,omitempty"`
	Text        string   `json:"text,omitempty"`
	Kinds       []string `json:"kinds,omitempty"`
}

func (f Filter) store() store.ItemFilter {
	return store.ItemFilter{
		Kinds: f.Kinds, TeamIDs: f.TeamIDs, StateIDs: f.StateIDs, LabelIDs: f.LabelIDs,
		AssigneeIDs: f.AssigneeIDs, Unassigned: f.Unassigned, ReporterIDs: f.ReporterIDs,
		BucketIDs: f.BucketIDs, Priorities: f.Priorities, Text: f.Text,
	}
}

// DoneFor is how long an item stays in the board's final column after it
// gets there. After that it is hidden unless the board asks for all of them
// (SPEC.md §7.2).
const DoneFor = 3 * 24 * 60 * 60 * 1_000_000 // microseconds, like every timestamp

// Board returns one team's board: one column per work state, excluding the
// default and rejected states.
//
// Buckets are hidden from board columns by default and shown by a per-board
// toggle, so that an engineering board is not diluted by milestones and a
// planning team's board can consist entirely of them (SPEC.md §6.2). Items
// done for longer than DoneFor are hidden the same way, behind allDone.
func (s *Service) Board(ctx context.Context, a Actor, teamID int64, f Filter, showBuckets, allDone bool, perColumn int) (*Board, error) {
	if err := requireUser(a); err != nil {
		return nil, err
	}
	team, err := store.TeamByID(ctx, s.DB.R, teamID)
	if err != nil {
		return nil, mapErr(err)
	}
	states, err := store.ListWorkStates(ctx, s.DB.R, teamID)
	if err != nil {
		return nil, mapErr(err)
	}
	b := &Board{Team: team}
	for _, st := range states {
		if st.Role == store.RoleDefault || st.Role == store.RoleRejected {
			continue
		}
		var endedSince int64
		if st.Role == store.RoleFinal && !allDone {
			endedSince = s.now() - DoneFor
		}
		col, err := s.column(ctx, teamID, st, f, showBuckets, endedSince, perColumn)
		if err != nil {
			return nil, err
		}
		b.Columns = append(b.Columns, col)
	}
	return b, nil
}

// Backlog returns the team's items in its default state, as a single column.
func (s *Service) Backlog(ctx context.Context, a Actor, teamID int64, f Filter, showBuckets bool, limit int) (*Column, error) {
	if err := requireUser(a); err != nil {
		return nil, err
	}
	st, err := store.WorkStateByRole(ctx, s.DB.R, teamID, store.RoleDefault)
	if err != nil {
		return nil, mapErr(err)
	}
	col, err := s.column(ctx, teamID, st, f, showBuckets, 0, limit)
	return &col, err
}

func (s *Service) column(ctx context.Context, teamID int64, st store.WorkState, f Filter, showBuckets bool, endedSince int64, limit int) (Column, error) {
	sf := f.store()
	sf.TeamIDs = []int64{teamID}
	sf.EndedSince = endedSince
	// A saved view may carry a state clause of its own. The column is that
	// clause intersected with the column's own state, never the column's state
	// silently replacing it (SPEC.md §9.4).
	if len(sf.StateIDs) > 0 && !slices.Contains(sf.StateIDs, st.ID) {
		return Column{State: st, Items: []store.Item{}}, nil
	}
	sf.StateIDs = []int64{st.ID}
	if !showBuckets && len(sf.Kinds) == 0 {
		sf.Kinds = []string{store.KindTicket}
	}
	items, cursor, err := store.ListItems(ctx, s.DB.R, sf, "", clampLimit(limit))
	if err != nil {
		return Column{}, mapErr(err)
	}
	if err := store.LoadLabels(ctx, s.DB.R, items); err != nil {
		return Column{}, mapErr(err)
	}
	count, err := store.CountItems(ctx, s.DB.R, sf)
	if err != nil {
		return Column{}, mapErr(err)
	}
	if items == nil {
		items = []store.Item{}
	}
	return Column{State: st, Items: items, Count: count, Cursor: cursor}, nil
}

// Page is a keyset page of items, which every unbounded list returns
// (SPEC.md §13.2 rule 9).
type Page struct {
	Items  []store.Item `json:"items"`
	Cursor string       `json:"cursor,omitempty"`
	Total  int          `json:"total"`
}

// ListItems is the filtered list behind search, a saved view applied outside a
// board, and a column's "load more".
func (s *Service) ListItems(ctx context.Context, a Actor, f Filter, cursor string, limit int) (*Page, error) {
	if err := requireUser(a); err != nil {
		return nil, err
	}
	sf := f.store()
	items, next, err := store.ListItems(ctx, s.DB.R, sf, cursor, clampLimit(limit))
	if err != nil {
		return nil, mapErr(err)
	}
	if err := store.LoadLabels(ctx, s.DB.R, items); err != nil {
		return nil, mapErr(err)
	}
	total, err := store.CountItems(ctx, s.DB.R, sf)
	if err != nil {
		return nil, mapErr(err)
	}
	if items == nil {
		items = []store.Item{}
	}
	return &Page{Items: items, Cursor: next, Total: total}, nil
}

// Bucket index scopes (SPEC.md §9.1).
const (
	// ScopeParticipation is the default: buckets my team owns, plus buckets
	// containing at least one of my team's tickets. Scoping by ownership alone
	// would show a team the buckets it created and hide the release it is
	// working toward.
	ScopeParticipation = "participation"
	ScopeOwned         = "owned"
	ScopeInstance      = "instance"
)

type BucketCard struct {
	store.Item
	Progress Progress `json:"progress"`
}

// BucketIndex lists buckets under one of the three scopes.
func (s *Service) BucketIndex(ctx context.Context, a Actor, teamID int64, scope, cursor string, limit int) ([]BucketCard, string, error) {
	if err := requireUser(a); err != nil {
		return nil, "", err
	}
	f := store.ItemFilter{Kinds: []string{store.KindBucket}}
	switch scope {
	case ScopeInstance:
	case ScopeOwned:
		f.TeamIDs = []int64{teamID}
	default:
		ids, err := s.participatingBuckets(ctx, teamID)
		if err != nil {
			return nil, "", err
		}
		if len(ids) == 0 {
			return []BucketCard{}, "", nil
		}
		f.IDs = ids
	}
	items, next, err := store.ListItems(ctx, s.DB.R, f, cursor, clampLimit(limit))
	if err != nil {
		return nil, "", mapErr(err)
	}
	out := make([]BucketCard, 0, len(items))
	for _, it := range items {
		p, err := progress(ctx, s.DB.R, it.ID)
		if err != nil {
			return nil, "", err
		}
		out = append(out, BucketCard{Item: it, Progress: p})
	}
	return out, next, nil
}

// progress counts a bucket's contents in a terminal state over the total —
// the "37 / 50" of SPEC.md §6.2 — and, between the two, what has been picked
// up and is not finished. A bucket with nothing done still reads differently
// depending on whether its tickets have been started.
func progress(ctx context.Context, q store.Q, bucketID int64) (Progress, error) {
	total, err := store.CountItems(ctx, q, store.ItemFilter{BucketIDs: []int64{bucketID}})
	if err != nil {
		return Progress{}, mapErr(err)
	}
	done, err := store.CountItems(ctx, q, store.ItemFilter{BucketIDs: []int64{bucketID},
		StateRoles: []string{store.RoleFinal, store.RoleRejected}})
	if err != nil {
		return Progress{}, mapErr(err)
	}
	// Every role but the default one and the two terminal ones, which is also
	// every intermediate state a team has invented: those carry no role at all.
	accepted, err := store.CountItems(ctx, q, store.ItemFilter{BucketIDs: []int64{bucketID},
		ExcludeRoles: []string{store.RoleDefault, store.RoleFinal, store.RoleRejected}})
	if err != nil {
		return Progress{}, mapErr(err)
	}
	return Progress{Done: done, Accepted: accepted, Total: total}, nil
}

// participatingBuckets is the default scope's set: buckets the team owns, plus
// buckets containing at least one of the team's tickets.
func (s *Service) participatingBuckets(ctx context.Context, teamID int64) ([]int64, error) {
	rows, err := s.DB.R.QueryContext(ctx, `
		SELECT id FROM items WHERE kind = 'bucket' AND team_id = ?
		UNION
		SELECT DISTINCT t.bucket_id FROM items t
		WHERE t.team_id = ? AND t.bucket_id IS NOT NULL`, teamID, teamID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// TeamGroup is a bucket's contents for one owning team.
type TeamGroup struct {
	Team  store.Team   `json:"team"`
	Items []store.Item `json:"items"`
}

// BucketContents lists everything assigned to a bucket, grouped by owning
// team. This is a view, not a field: a bucket's lifecycle is independent of
// its contents in both directions (SPEC.md §6.4).
func (s *Service) BucketContents(ctx context.Context, a Actor, bucketID int64) ([]TeamGroup, error) {
	if err := requireUser(a); err != nil {
		return nil, err
	}
	items, _, err := store.ListItems(ctx, s.DB.R,
		store.ItemFilter{BucketIDs: []int64{bucketID}}, "", 500)
	if err != nil {
		return nil, mapErr(err)
	}
	if err := store.LoadLabels(ctx, s.DB.R, items); err != nil {
		return nil, mapErr(err)
	}
	var groups []TeamGroup
	index := map[int64]int{}
	for _, it := range items {
		i, ok := index[it.TeamID]
		if !ok {
			team, err := store.TeamByID(ctx, s.DB.R, it.TeamID)
			if err != nil {
				return nil, mapErr(err)
			}
			groups = append(groups, TeamGroup{Team: team})
			i = len(groups) - 1
			index[it.TeamID] = i
		}
		groups[i].Items = append(groups[i].Items, it)
	}
	return list(groups, nil)
}

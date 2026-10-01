package store

import (
	"context"
	"database/sql"
	"errors"
)

// A saved view is a name attached to a CSQL query (SPEC.md §9.4). The query is
// stored canonical, naming its referents by #id; beside it, one row per thing
// it names, so a label a view uses is never reaped and a deleted referent can
// be named wherever the view is offered.

// Ref is one thing a view's query names by id, and what it is called now.
type Ref struct {
	Field string
	ID    int64
	// Name is how a query writes it today: a username, a label's name, a
	// bucket's identifier, a team's short code, a state's name. Empty when
	// the referent has been deleted.
	Name string
}

type SavedView struct {
	ID         int64  `json:"id"`
	TeamID     int64  `json:"teamId"`
	Name       string `json:"name"`
	PinBoard   bool   `json:"pinBoard"`
	PinBacklog bool   `json:"pinBacklog"`
	CreatedAt  int64  `json:"createdAt"`
	UpdatedAt  int64  `json:"updatedAt"`
	// Query is canonical as stored; the service hands it out written with
	// names, and lists in Missing what it names that has been deleted.
	Query   string   `json:"query"`
	Missing []string `json:"missing,omitempty"`
	Refs    []Ref    `json:"-"`
}

func CreateSavedView(ctx context.Context, q Q, v SavedView) (int64, error) {
	res, err := q.ExecContext(ctx, `
		INSERT INTO saved_views (team_id, name, name_folded, pin_board, pin_backlog, query,
		                         created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		v.TeamID, v.Name, Fold(v.Name), v.PinBoard, v.PinBacklog, v.Query, v.CreatedAt, v.UpdatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func UpdateSavedView(ctx context.Context, q Q, v SavedView) error {
	_, err := q.ExecContext(ctx, `
		UPDATE saved_views SET name = ?, name_folded = ?, pin_board = ?, pin_backlog = ?,
		                       query = ?, updated_at = ? WHERE id = ?`,
		v.Name, Fold(v.Name), v.PinBoard, v.PinBacklog, v.Query, v.UpdatedAt, v.ID)
	return err
}

func DeleteSavedView(ctx context.Context, q Q, id int64) error {
	// A saved view is deleted outright rather than archived. It holds no work,
	// and what it used to be is in the log (SPEC.md §9.4).
	_, err := q.ExecContext(ctx, `DELETE FROM saved_views WHERE id = ?`, id)
	return err
}

// ReplaceRefs records what a view's query names by id.
func ReplaceRefs(ctx context.Context, q Q, viewID int64, refs []Ref) error {
	if _, err := q.ExecContext(ctx, `DELETE FROM saved_view_refs WHERE view_id = ?`, viewID); err != nil {
		return err
	}
	for _, r := range refs {
		if _, err := q.ExecContext(ctx,
			`INSERT INTO saved_view_refs (view_id, dimension, ref_id) VALUES (?, ?, ?)`,
			viewID, r.Field, r.ID); err != nil {
			return err
		}
	}
	return nil
}

const viewColumns = `id, team_id, name, pin_board, pin_backlog, query, created_at, updated_at`

func scanView(s interface{ Scan(...any) error }) (SavedView, error) {
	var v SavedView
	err := s.Scan(&v.ID, &v.TeamID, &v.Name, &v.PinBoard, &v.PinBacklog, &v.Query, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}

func SavedViewByID(ctx context.Context, q Q, id int64) (SavedView, error) {
	v, err := scanView(q.QueryRowContext(ctx, `SELECT `+viewColumns+` FROM saved_views WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return SavedView{}, ErrNotFound
	}
	if err != nil {
		return SavedView{}, err
	}
	v.Refs, err = ViewRefs(ctx, q, v.ID)
	return v, err
}

// ListSavedViews returns one team's views, alphabetical by name, case-folded
// (SPEC.md §9.4). Manual rank exists for board columns and is not worth a drag
// target on a list of names.
func ListSavedViews(ctx context.Context, q Q, teamID int64) ([]SavedView, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT `+viewColumns+` FROM saved_views WHERE team_id = ? ORDER BY name_folded`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SavedView
	for rows.Next() {
		v, err := scanView(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if out[i].Refs, err = ViewRefs(ctx, q, out[i].ID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// ViewRefs names each referent of a view as a query writes it today. A
// deleted one comes back with no name.
func ViewRefs(ctx context.Context, q Q, viewID int64) ([]Ref, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT r.dimension, r.ref_id, COALESCE(s.name, l.name, u.username, t.short_code,
		       CASE WHEN b.id IS NOT NULL THEN bt.short_code || '-' || b.number END, '')
		FROM saved_view_refs r
		LEFT JOIN work_states s ON r.dimension = 'state' AND s.id = r.ref_id
		LEFT JOIN labels l ON r.dimension = 'label' AND l.id = r.ref_id
		LEFT JOIN users u ON r.dimension IN ('assignee','reporter') AND u.id = r.ref_id
		LEFT JOIN items b ON r.dimension = 'bucket' AND b.id = r.ref_id
		LEFT JOIN teams bt ON bt.id = b.origin_team_id
		LEFT JOIN teams t ON r.dimension = 'team' AND t.id = r.ref_id
		WHERE r.view_id = ? ORDER BY r.id`, viewID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Ref
	for rows.Next() {
		var r Ref
		if err := rows.Scan(&r.Field, &r.ID, &r.Name); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

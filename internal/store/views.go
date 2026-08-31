package store

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
)

// A saved view is a name attached to a filter the application can already
// express (SPEC.md §9.4). It is stored as its own structured representation —
// one row per clause, referencing its target by identity — never as a
// serialised query string.

// The filter dimensions of SPEC.md §9.2, and no others.
const (
	DimState    = "state"
	DimLabel    = "label"
	DimAssignee = "assignee"
	DimReporter = "reporter"
	DimBucket   = "bucket"
	DimPriority = "priority"
	DimTeam     = "team"
	DimText     = "text"
)

func ValidDimension(d string) bool {
	switch d {
	case DimState, DimLabel, DimAssignee, DimReporter, DimBucket, DimPriority, DimTeam, DimText:
		return true
	}
	return false
}

type Clause struct {
	ID        int64  `json:"id"`
	Dimension string `json:"dimension"`
	RefID     int64  `json:"refId,omitempty"`
	Text      string `json:"text,omitempty"`
	// Missing is set when the clause names something that has been deleted.
	// The clause is kept and matches nothing; the view names the missing
	// reference wherever it is offered, because a view that quietly repairs
	// itself is a view nobody can trust (SPEC.md §9.4).
	Missing bool   `json:"missing,omitempty"`
	Display string `json:"display"`
}

type SavedView struct {
	ID         int64    `json:"id"`
	TeamID     int64    `json:"teamId"`
	Name       string   `json:"name"`
	PinBoard   bool     `json:"pinBoard"`
	PinBacklog bool     `json:"pinBacklog"`
	CreatedAt  int64    `json:"createdAt"`
	UpdatedAt  int64    `json:"updatedAt"`
	Clauses    []Clause `json:"clauses"`
}

func CreateSavedView(ctx context.Context, q Q, v SavedView) (int64, error) {
	res, err := q.ExecContext(ctx, `
		INSERT INTO saved_views (team_id, name, name_folded, pin_board, pin_backlog,
		                         created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		v.TeamID, v.Name, Fold(v.Name), v.PinBoard, v.PinBacklog, v.CreatedAt, v.UpdatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func UpdateSavedView(ctx context.Context, q Q, v SavedView) error {
	_, err := q.ExecContext(ctx, `
		UPDATE saved_views SET name = ?, name_folded = ?, pin_board = ?, pin_backlog = ?,
		                       updated_at = ? WHERE id = ?`,
		v.Name, Fold(v.Name), v.PinBoard, v.PinBacklog, v.UpdatedAt, v.ID)
	return err
}

func DeleteSavedView(ctx context.Context, q Q, id int64) error {
	// A saved view is deleted outright rather than archived. It holds no work,
	// and what it used to be is in the log (SPEC.md §9.4).
	_, err := q.ExecContext(ctx, `DELETE FROM saved_views WHERE id = ?`, id)
	return err
}

func ReplaceClauses(ctx context.Context, q Q, viewID int64, clauses []Clause) error {
	if _, err := q.ExecContext(ctx, `DELETE FROM saved_view_clauses WHERE view_id = ?`, viewID); err != nil {
		return err
	}
	for _, c := range clauses {
		if _, err := q.ExecContext(ctx,
			`INSERT INTO saved_view_clauses (view_id, dimension, ref_id, text_value) VALUES (?, ?, ?, ?)`,
			viewID, c.Dimension, null(c.RefID), null(c.Text)); err != nil {
			return err
		}
	}
	return nil
}

func SavedViewByID(ctx context.Context, q Q, id int64) (SavedView, error) {
	var v SavedView
	err := q.QueryRowContext(ctx, `
		SELECT id, team_id, name, pin_board, pin_backlog, created_at, updated_at
		FROM saved_views WHERE id = ?`, id).
		Scan(&v.ID, &v.TeamID, &v.Name, &v.PinBoard, &v.PinBacklog, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return SavedView{}, ErrNotFound
	}
	if err != nil {
		return SavedView{}, err
	}
	v.Clauses, err = ViewClauses(ctx, q, v.ID)
	return v, err
}

// ListSavedViews returns one team's views, alphabetical by name, case-folded
// (SPEC.md §9.4). Manual rank exists for board columns and is not worth a drag
// target on a list of names.
func ListSavedViews(ctx context.Context, q Q, teamID int64) ([]SavedView, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, team_id, name, pin_board, pin_backlog, created_at, updated_at
		FROM saved_views WHERE team_id = ? ORDER BY name_folded`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SavedView
	for rows.Next() {
		var v SavedView
		if err := rows.Scan(&v.ID, &v.TeamID, &v.Name, &v.PinBoard, &v.PinBacklog,
			&v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if out[i].Clauses, err = ViewClauses(ctx, q, out[i].ID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// ViewClauses resolves each clause's referent for display. A clause whose
// referent has been deleted comes back with Missing set and a Display naming
// what has gone.
func ViewClauses(ctx context.Context, q Q, viewID int64) ([]Clause, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT c.id, c.dimension, c.ref_id, c.text_value,
		       s.name, l.name, u.username, ot.short_code, b.number, b.title, t.name
		FROM saved_view_clauses c
		LEFT JOIN work_states s ON c.dimension = 'state' AND s.id = c.ref_id
		LEFT JOIN labels l ON c.dimension = 'label' AND l.id = c.ref_id
		LEFT JOIN users u ON c.dimension IN ('assignee','reporter') AND u.id = c.ref_id
		LEFT JOIN items b ON c.dimension = 'bucket' AND b.id = c.ref_id
		LEFT JOIN teams ot ON ot.id = b.origin_team_id
		LEFT JOIN teams t ON c.dimension = 'team' AND t.id = c.ref_id
		WHERE c.view_id = ? ORDER BY c.id`, viewID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Clause
	for rows.Next() {
		var c Clause
		var refID, bucketNumber sql.NullInt64
		var textValue, stateName, labelName, username, bucketCode, bucketTitle, teamName sql.NullString
		if err := rows.Scan(&c.ID, &c.Dimension, &refID, &textValue,
			&stateName, &labelName, &username, &bucketCode, &bucketNumber, &bucketTitle, &teamName); err != nil {
			return nil, err
		}
		c.RefID, c.Text = num(refID), text(textValue)
		switch c.Dimension {
		case DimText:
			c.Display = c.Text
		case DimPriority:
			c.Display = "P" + strconv.FormatInt(c.RefID, 10)
		case DimState:
			c.Display, c.Missing = text(stateName), !stateName.Valid
		case DimLabel:
			c.Display, c.Missing = text(labelName), !labelName.Valid
		case DimAssignee, DimReporter:
			if c.Dimension == DimAssignee && c.RefID == 0 {
				c.Display = "Unassigned"
				break
			}
			c.Display, c.Missing = text(username), !username.Valid
		case DimBucket:
			c.Missing = !bucketTitle.Valid
			if !c.Missing {
				c.Display = Identifier(text(bucketCode), num(bucketNumber)) + " " + text(bucketTitle)
			}
		case DimTeam:
			c.Display, c.Missing = text(teamName), !teamName.Valid
		}
		if c.Missing {
			c.Display = "deleted " + c.Dimension + " #" + strconv.FormatInt(c.RefID, 10)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// AsFilter turns a view's clauses into the filter the list endpoints take. A
// clause naming something deleted contributes an identifier nothing matches,
// which is exactly the behaviour §9.4 asks for: an empty result, never a
// silently wider one.
func (v SavedView) AsFilter() ItemFilter {
	var f ItemFilter
	for _, c := range v.Clauses {
		switch c.Dimension {
		case DimState:
			f.StateIDs = append(f.StateIDs, c.RefID)
		case DimLabel:
			f.LabelIDs = append(f.LabelIDs, c.RefID)
		case DimAssignee:
			if c.RefID == 0 {
				f.Unassigned = true
				break
			}
			f.AssigneeIDs = append(f.AssigneeIDs, c.RefID)
		case DimReporter:
			f.ReporterIDs = append(f.ReporterIDs, c.RefID)
		case DimBucket:
			f.BucketIDs = append(f.BucketIDs, c.RefID)
		case DimPriority:
			f.Priorities = append(f.Priorities, c.RefID)
		case DimTeam:
			f.TeamIDs = append(f.TeamIDs, c.RefID)
		case DimText:
			f.Text = c.Text
		}
	}
	return f
}

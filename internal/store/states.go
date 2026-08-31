package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// The four lifecycle roles. A team MUST always have all four filled
// (SPEC.md §7.1); a state without a role is intermediate.
const (
	RoleDefault  = "default"
	RoleAccepted = "accepted"
	RoleRejected = "rejected"
	RoleFinal    = "final"
)

// DefaultStates is the list a new team starts with — the example of
// SPEC.md §7.1, which is also the arrangement every artboard draws.
var DefaultStates = []WorkState{
	{Name: "Backlog", Role: RoleDefault},
	{Name: "To Do", Role: RoleAccepted},
	{Name: "In Progress"},
	{Name: "In Review"},
	{Name: "Done", Role: RoleFinal},
	{Name: "Rejected", Role: RoleRejected},
}

type WorkState struct {
	ID        int64  `json:"id"`
	TeamID    int64  `json:"teamId"`
	Name      string `json:"name"`
	Role      string `json:"role,omitempty"`
	Position  int64  `json:"position"`
	CreatedAt int64  `json:"createdAt"`
}

// Terminal reports whether entering this state sets an item's actual end
// (SPEC.md §7.3).
func (s WorkState) Terminal() bool { return s.Role == RoleFinal || s.Role == RoleRejected }

func CreateWorkState(ctx context.Context, q Q, s WorkState) (int64, error) {
	res, err := q.ExecContext(ctx, `
		INSERT INTO work_states (team_id, name, name_folded, role, position, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		s.TeamID, s.Name, Fold(s.Name), null(s.Role), s.Position, s.CreatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func scanState(row interface{ Scan(...any) error }) (WorkState, error) {
	var s WorkState
	var role sql.NullString
	err := row.Scan(&s.ID, &s.TeamID, &s.Name, &role, &s.Position, &s.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return WorkState{}, ErrNotFound
	}
	s.Role = text(role)
	return s, err
}

const stateColumns = `id, team_id, name, role, position, created_at`

func WorkStateByID(ctx context.Context, q Q, id int64) (WorkState, error) {
	return scanState(q.QueryRowContext(ctx, `SELECT `+stateColumns+` FROM work_states WHERE id = ?`, id))
}

func WorkStateByRole(ctx context.Context, q Q, teamID int64, role string) (WorkState, error) {
	return scanState(q.QueryRowContext(ctx,
		`SELECT `+stateColumns+` FROM work_states WHERE team_id = ? AND role = ?`, teamID, role))
}

func ListWorkStates(ctx context.Context, q Q, teamID int64) ([]WorkState, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT `+stateColumns+` FROM work_states WHERE team_id = ? ORDER BY position, id`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WorkState
	for rows.Next() {
		s, err := scanState(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func UpdateWorkState(ctx context.Context, q Q, s WorkState) error {
	_, err := q.ExecContext(ctx, `
		UPDATE work_states SET name = ?, name_folded = ?, role = ?, position = ? WHERE id = ?`,
		s.Name, Fold(s.Name), null(s.Role), s.Position, s.ID)
	return err
}

// DeleteWorkState removes an empty state. It reports how many items are in the
// way rather than moving them somewhere the caller did not choose.
func DeleteWorkState(ctx context.Context, q Q, id int64) error {
	var n int
	if err := q.QueryRowContext(ctx, `SELECT count(*) FROM items WHERE state_id = ?`, id).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("%d item(s) are still in this state", n)
	}
	_, err := q.ExecContext(ctx, `DELETE FROM work_states WHERE id = ?`, id)
	return err
}

// CreateDefaultStates gives a new team the six states of SPEC.md §7.1.
func CreateDefaultStates(ctx context.Context, q Q, teamID, now int64) error {
	for i, s := range DefaultStates {
		s.TeamID, s.Position, s.CreatedAt = teamID, int64(i+1), now
		if _, err := CreateWorkState(ctx, q, s); err != nil {
			return err
		}
	}
	return nil
}

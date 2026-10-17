package store

import (
	"context"
	"database/sql"
	"errors"
)

type Team struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	ShortCode   string `json:"shortCode"`
	Avatar      string `json:"avatar,omitempty"`
	Description string `json:"description,omitempty"`
	NextNumber  int64  `json:"-"`
	ArchivedAt  int64  `json:"archivedAt,omitempty"`
	CreatedAt   int64  `json:"createdAt"`
}

const teamColumns = `id, name, short_code, avatar, description, next_number,
	archived_at, created_at`

func scanTeam(row interface{ Scan(...any) error }) (Team, error) {
	var t Team
	var archived sql.NullInt64
	var avatar, description sql.NullString
	err := row.Scan(&t.ID, &t.Name, &t.ShortCode, &avatar, &description,
		&t.NextNumber, &archived, &t.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Team{}, ErrNotFound
	}
	t.ArchivedAt = num(archived)
	t.Avatar, t.Description = text(avatar), text(description)
	return t, err
}

func CreateTeam(ctx context.Context, q Q, t Team) (int64, error) {
	res, err := q.ExecContext(ctx, `
		INSERT INTO teams (name, name_folded, short_code, short_code_folded,
		                   next_number, created_at)
		VALUES (?, ?, ?, ?, 1, ?)`,
		t.Name, Fold(t.Name), t.ShortCode, Fold(t.ShortCode), t.CreatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func TeamByID(ctx context.Context, q Q, id int64) (Team, error) {
	return scanTeam(q.QueryRowContext(ctx, `SELECT `+teamColumns+` FROM teams WHERE id = ?`, id))
}

func TeamByCode(ctx context.Context, q Q, code string) (Team, error) {
	return scanTeam(q.QueryRowContext(ctx,
		`SELECT `+teamColumns+` FROM teams WHERE short_code_folded = ?`, Fold(code)))
}

func ListTeams(ctx context.Context, q Q, includeArchived bool) ([]Team, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT `+teamColumns+` FROM teams WHERE (? OR archived_at IS NULL) ORDER BY name_folded`,
		includeArchived)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Team
	for rows.Next() {
		t, err := scanTeam(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func RenameTeam(ctx context.Context, q Q, id int64, name string) error {
	// The short code is not here on purpose: it is set at creation and MUST
	// NOT change (SPEC.md §5.3).
	_, err := q.ExecContext(ctx, `UPDATE teams SET name = ?, name_folded = ? WHERE id = ?`,
		name, Fold(name), id)
	return err
}

func SetTeamProfile(ctx context.Context, q Q, id int64, avatar, description string) error {
	_, err := q.ExecContext(ctx, `UPDATE teams SET avatar = ?, description = ? WHERE id = ?`,
		null(avatar), null(description), id)
	return err
}

func ArchiveTeam(ctx context.Context, q Q, id, now int64) error {
	_, err := q.ExecContext(ctx, `UPDATE teams SET archived_at = ? WHERE id = ?`, null(now), id)
	return err
}

// NextItemNumber increments a team's counter and returns the new value in one
// statement (SPEC.md §13.2 rule 2). RETURNING makes it atomic; the caller runs
// it inside the inserting transaction, and UNIQUE (origin_team_id, number) is
// the backstop.
//
// SELECT max(number) + 1 followed by an insert is forbidden even under a
// single writer: a duplicate identifier is unrecoverable once it is in URLs,
// branch names and messages.
func NextItemNumber(ctx context.Context, q Q, teamID int64) (int64, error) {
	var n int64
	err := q.QueryRowContext(ctx,
		`UPDATE teams SET next_number = next_number + 1 WHERE id = ? RETURNING next_number - 1`,
		teamID).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return n, err
}

// Membership. A user may belong to any number of teams with a different role
// in each (SPEC.md §5.2).

type Member struct {
	TeamID   int64  `json:"teamId"`
	UserID   int64  `json:"userId"`
	Username string `json:"username"`
	Role     string `json:"role"` // "admin" or "member"
	AddedAt  int64  `json:"addedAt"`
}

func SetMember(ctx context.Context, q Q, teamID, userID int64, role string, now int64) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO team_members (team_id, user_id, role, added_at) VALUES (?, ?, ?, ?)
		ON CONFLICT (team_id, user_id) DO UPDATE SET role = excluded.role`,
		teamID, userID, role, now)
	return err
}

func RemoveMember(ctx context.Context, q Q, teamID, userID int64) error {
	_, err := q.ExecContext(ctx, `DELETE FROM team_members WHERE team_id = ? AND user_id = ?`,
		teamID, userID)
	return err
}

// MemberRole returns "" when the user is not in the team.
func MemberRole(ctx context.Context, q Q, teamID, userID int64) (string, error) {
	var role string
	err := q.QueryRowContext(ctx,
		`SELECT role FROM team_members WHERE team_id = ? AND user_id = ?`, teamID, userID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return role, err
}

func ListMembers(ctx context.Context, q Q, teamID int64) ([]Member, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT m.team_id, m.user_id, u.username, m.role, m.added_at
		FROM team_members m JOIN users u ON u.id = m.user_id
		WHERE m.team_id = ? ORDER BY u.username_folded`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.TeamID, &m.UserID, &m.Username, &m.Role, &m.AddedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func ListUserTeams(ctx context.Context, q Q, userID int64) ([]Member, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT m.team_id, m.user_id, u.username, m.role, m.added_at
		FROM team_members m
		JOIN users u ON u.id = m.user_id
		JOIN teams t ON t.id = m.team_id
		WHERE m.user_id = ? AND t.archived_at IS NULL
		ORDER BY t.name_folded`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.TeamID, &m.UserID, &m.Username, &m.Role, &m.AddedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

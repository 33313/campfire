package store

import (
	"context"
	"database/sql"
	"errors"
)

// ErrNotFound is returned by every lookup in this package when the row is
// absent. Callers turn it into a 404 or a "no such user" as appropriate.
var ErrNotFound = errors.New("not found")

type User struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	Email        string `json:"email,omitempty"`
	TOTPSecret   string `json:"-"`
	Avatar       string `json:"avatar,omitempty"`
	Bio          string `json:"bio,omitempty"`
	IsAdmin      bool   `json:"isAdmin"`
	CreatedAt    int64  `json:"createdAt"`
}

const userColumns = `id, username, password_hash, email, totp_secret, avatar, bio, is_admin, created_at`

func scanUser(row interface{ Scan(...any) error }) (User, error) {
	var u User
	var email, secret, avatar, bio sql.NullString
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &email, &secret, &avatar, &bio,
		&u.IsAdmin, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	u.Email, u.TOTPSecret, u.Avatar, u.Bio = text(email), text(secret), text(avatar), text(bio)
	return u, nil
}

// CreateUser inserts a user, folding the case-insensitive columns here rather
// than leaving it to the database (SPEC.md §13.2 rule 4).
func CreateUser(ctx context.Context, q Q, u User) (int64, error) {
	res, err := q.ExecContext(ctx, `
		INSERT INTO users (username, username_folded, password_hash, email, email_folded,
		                   totp_secret, avatar, is_admin, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.Username, Fold(u.Username), u.PasswordHash,
		null(u.Email), null(Fold(u.Email)), null(u.TOTPSecret), null(u.Avatar),
		u.IsAdmin, u.CreatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func UserByID(ctx context.Context, q Q, id int64) (User, error) {
	return scanUser(q.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id = ?`, id))
}

func UserByUsername(ctx context.Context, q Q, username string) (User, error) {
	return scanUser(q.QueryRowContext(ctx,
		`SELECT `+userColumns+` FROM users WHERE username_folded = ?`, Fold(username)))
}

// ListUsers returns users after the given id, in id order, at most limit.
// Keyset, like every unbounded list (SPEC.md §13.2 rule 9).
func ListUsers(ctx context.Context, q Q, afterID int64, limit int) ([]User, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT `+userColumns+` FROM users WHERE id > ? ORDER BY id LIMIT ?`, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// TeamBadge is one membership as the administration screen's users table
// shows it: "SOL admin, TERRA member".
type TeamBadge struct {
	ShortCode string `json:"shortCode"`
	Role      string `json:"role"`
}

// MembershipBadges returns every user's memberships, keyed by user id.
func MembershipBadges(ctx context.Context, q Q) (map[int64][]TeamBadge, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT m.user_id, t.short_code, m.role
		FROM team_members m JOIN teams t ON t.id = m.team_id
		WHERE t.archived_at IS NULL
		ORDER BY t.short_code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]TeamBadge{}
	for rows.Next() {
		var userID int64
		var b TeamBadge
		if err := rows.Scan(&userID, &b.ShortCode, &b.Role); err != nil {
			return nil, err
		}
		out[userID] = append(out[userID], b)
	}
	return out, rows.Err()
}

// LastSeen returns each user's most recent session activity, keyed by user
// id. A user with no session does not appear.
func LastSeen(ctx context.Context, q Q) (map[int64]int64, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT user_id, max(last_used_at) FROM sessions GROUP BY user_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]int64{}
	for rows.Next() {
		var userID, at int64
		if err := rows.Scan(&userID, &at); err != nil {
			return nil, err
		}
		out[userID] = at
	}
	return out, rows.Err()
}

func CountUsers(ctx context.Context, q Q) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT count(*) FROM users`).Scan(&n)
	return n, err
}

func SetPassword(ctx context.Context, q Q, userID int64, hash string) error {
	_, err := q.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, hash, userID)
	return err
}

func SetTOTPSecret(ctx context.Context, q Q, userID int64, secret string) error {
	_, err := q.ExecContext(ctx, `UPDATE users SET totp_secret = ? WHERE id = ?`, null(secret), userID)
	return err
}

func SetProfile(ctx context.Context, q Q, userID int64, email, avatar, bio string) error {
	_, err := q.ExecContext(ctx,
		`UPDATE users SET email = ?, email_folded = ?, avatar = ?, bio = ? WHERE id = ?`,
		null(email), null(Fold(email)), null(avatar), null(bio), userID)
	return err
}

func SetAdmin(ctx context.Context, q Q, userID int64, isAdmin bool) error {
	_, err := q.ExecContext(ctx, `UPDATE users SET is_admin = ? WHERE id = ?`, isAdmin, userID)
	return err
}

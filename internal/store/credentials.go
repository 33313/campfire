package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// Sessions and agent tokens are credentials, not entities (SPEC.md §4, §14.2).
// Both live in the database rather than in process memory, which is what makes
// a restart invisible instead of a mass logout and revocation effective on the
// next call (SPEC.md §13.2 rule 5).

type Session struct {
	ID         int64
	UserID     int64
	CreatedAt  int64
	LastUsedAt int64
	ExpiresAt  int64
}

func CreateSession(ctx context.Context, q Q, userID int64, tokenHash string, now, expires int64) (int64, error) {
	res, err := q.ExecContext(ctx, `
		INSERT INTO sessions (token_hash, user_id, created_at, last_used_at, expires_at)
		VALUES (?, ?, ?, ?, ?)`, tokenHash, userID, now, now, expires)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// SessionByToken returns a live session, or ErrNotFound if it is absent or
// expired. Expiry is applied in the statement so that no caller can forget it.
func SessionByToken(ctx context.Context, q Q, tokenHash string, now int64) (Session, error) {
	var s Session
	err := q.QueryRowContext(ctx, `
		SELECT id, user_id, created_at, last_used_at, expires_at
		FROM sessions WHERE token_hash = ? AND expires_at > ?`, tokenHash, now).
		Scan(&s.ID, &s.UserID, &s.CreatedAt, &s.LastUsedAt, &s.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	return s, err
}

func TouchSession(ctx context.Context, q Q, id, now, expires int64) error {
	_, err := q.ExecContext(ctx,
		`UPDATE sessions SET last_used_at = ?, expires_at = ? WHERE id = ?`, now, expires, id)
	return err
}

func DeleteSession(ctx context.Context, q Q, id int64) error {
	_, err := q.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	return err
}

func DeleteUserSessions(ctx context.Context, q Q, userID int64) error {
	_, err := q.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

// DeleteOtherUserSessions ends every session for the account except one. A
// password change has no reason to sign the person making it out.
func DeleteOtherUserSessions(ctx context.Context, q Q, userID, keepID int64) error {
	_, err := q.ExecContext(ctx,
		`DELETE FROM sessions WHERE user_id = ? AND id <> ?`, userID, keepID)
	return err
}

// ReapExpiredSessions is the lazy cleanup of SPEC.md §13.2 rule 6: there are
// no background jobs, so expired rows go on a write that was happening anyway.
func ReapExpiredSessions(ctx context.Context, q Q, now int64) error {
	_, err := q.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, now)
	return err
}

type AgentToken struct {
	ID         int64  `json:"id"`
	UserID     int64  `json:"userId"`
	Name       string `json:"name"`
	CreatedAt  int64  `json:"createdAt"`
	LastUsedAt int64  `json:"lastUsedAt,omitempty"`
	RevokedAt  int64  `json:"revokedAt,omitempty"`
}

func CreateAgentToken(ctx context.Context, q Q, userID int64, name, tokenHash string, now int64) (int64, error) {
	res, err := q.ExecContext(ctx, `
		INSERT INTO agent_tokens (token_hash, user_id, name, created_at)
		VALUES (?, ?, ?, ?)`, tokenHash, userID, name, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// AgentTokenByHash returns a token that has not been revoked. A revoked token
// authenticates nothing on the very next call, because nothing about it is
// held in the process.
func AgentTokenByHash(ctx context.Context, q Q, tokenHash string) (AgentToken, error) {
	var t AgentToken
	var lastUsed, revoked sql.NullInt64
	err := q.QueryRowContext(ctx, `
		SELECT id, user_id, name, created_at, last_used_at, revoked_at
		FROM agent_tokens WHERE token_hash = ? AND revoked_at IS NULL`, tokenHash).
		Scan(&t.ID, &t.UserID, &t.Name, &t.CreatedAt, &lastUsed, &revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return AgentToken{}, ErrNotFound
	}
	t.LastUsedAt, t.RevokedAt = num(lastUsed), num(revoked)
	return t, err
}

func AgentTokenByID(ctx context.Context, q Q, id int64) (AgentToken, error) {
	var t AgentToken
	var lastUsed, revoked sql.NullInt64
	err := q.QueryRowContext(ctx, `
		SELECT id, user_id, name, created_at, last_used_at, revoked_at
		FROM agent_tokens WHERE id = ?`, id).
		Scan(&t.ID, &t.UserID, &t.Name, &t.CreatedAt, &lastUsed, &revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return AgentToken{}, ErrNotFound
	}
	t.LastUsedAt, t.RevokedAt = num(lastUsed), num(revoked)
	return t, err
}

func TouchAgentToken(ctx context.Context, q Q, id, now int64) error {
	_, err := q.ExecContext(ctx, `UPDATE agent_tokens SET last_used_at = ? WHERE id = ?`, now, id)
	return err
}

func RevokeAgentToken(ctx context.Context, q Q, id, now int64) error {
	_, err := q.ExecContext(ctx,
		`UPDATE agent_tokens SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`, now, id)
	return err
}

// ListAgentTokens returns one user's tokens, or every token when userID is 0,
// which is how an instance administrator revokes any of them (SPEC.md §14.2).
func ListAgentTokens(ctx context.Context, q Q, userID int64) ([]AgentToken, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, user_id, name, created_at, last_used_at, revoked_at
		FROM agent_tokens WHERE (? = 0 OR user_id = ?) AND revoked_at IS NULL ORDER BY id`,
		userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AgentToken
	for rows.Next() {
		var t AgentToken
		var lastUsed, revoked sql.NullInt64
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &t.CreatedAt, &lastUsed, &revoked); err != nil {
			return nil, err
		}
		t.LastUsedAt, t.RevokedAt = num(lastUsed), num(revoked)
		out = append(out, t)
	}
	return out, rows.Err()
}

// A one-time link is how an account is created or a password is reset. No flow
// depends on email delivery (SPEC.md §5.1), so an administrator issues the
// link and hands it over out of band.
type OneTimeLink struct {
	ID        int64
	Kind      string // "invite", "reset", or "github" for a GitHub round trip
	UserID    int64  // the account a reset targets; 0 for an invitation
	CreatedBy int64
	CreatedAt int64
	ExpiresAt int64
	UsedAt    int64
}

func CreateOneTimeLink(ctx context.Context, q Q, l OneTimeLink, tokenHash string) (int64, error) {
	res, err := q.ExecContext(ctx, `
		INSERT INTO one_time_links (token_hash, kind, user_id, created_by, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		tokenHash, l.Kind, null(l.UserID), l.CreatedBy, l.CreatedAt, l.ExpiresAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// OneTimeLinkByToken finds a live link of one of the given kinds. Naming them
// is what keeps a GitHub round trip's state from redeeming as an invitation.
func OneTimeLinkByToken(ctx context.Context, q Q, tokenHash string, now int64, kinds ...string) (OneTimeLink, error) {
	var l OneTimeLink
	var userID, usedAt sql.NullInt64
	args := []any{tokenHash, now}
	for _, k := range kinds {
		args = append(args, k)
	}
	err := q.QueryRowContext(ctx, `
		SELECT id, kind, user_id, created_by, created_at, expires_at, used_at
		FROM one_time_links
		WHERE token_hash = ? AND used_at IS NULL AND expires_at > ?
		  AND kind IN (`+strings.TrimSuffix(strings.Repeat("?,", len(kinds)), ",")+`)`, args...).
		Scan(&l.ID, &l.Kind, &userID, &l.CreatedBy, &l.CreatedAt, &l.ExpiresAt, &usedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return OneTimeLink{}, ErrNotFound
	}
	l.UserID, l.UsedAt = num(userID), num(usedAt)
	return l, err
}

// UseOneTimeLink marks a link spent. It reports ErrNotFound when the link was
// already used, so that two clients racing on one invitation cannot both win:
// the invariant is held by the statement, not by a prior read (rule 1).
func UseOneTimeLink(ctx context.Context, q Q, id, now int64) error {
	res, err := q.ExecContext(ctx,
		`UPDATE one_time_links SET used_at = ? WHERE id = ? AND used_at IS NULL`, now, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ReapOneTimeLinks removes links that are spent or expired. Lazy cleanup on a
// write, because there are no background jobs (SPEC.md §13.2 rule 6).
func ReapOneTimeLinks(ctx context.Context, q Q, now int64) error {
	_, err := q.ExecContext(ctx,
		`DELETE FROM one_time_links WHERE expires_at <= ? OR used_at IS NOT NULL`, now)
	return err
}

// RateLimit records one attempt against bucket and reports whether the caller
// is now over limit within the window.
//
// The counter is a row, not a map in the process (rule 5), so a restart does
// not clear a lockout and every worker sees the same count.
func RateLimit(ctx context.Context, q Q, bucket string, limit int, now, window int64) (bool, error) {
	_, err := q.ExecContext(ctx, `
		INSERT INTO rate_limits (bucket, window_start, count) VALUES (?, ?, 1)
		ON CONFLICT (bucket) DO UPDATE SET
			window_start = CASE WHEN excluded.window_start - rate_limits.window_start >= ?
			                    THEN excluded.window_start ELSE rate_limits.window_start END,
			count        = CASE WHEN excluded.window_start - rate_limits.window_start >= ?
			                    THEN 1 ELSE rate_limits.count + 1 END`,
		bucket, now, window, window)
	if err != nil {
		return false, err
	}
	var count int
	if err := q.QueryRowContext(ctx,
		`SELECT count FROM rate_limits WHERE bucket = ?`, bucket).Scan(&count); err != nil {
		return false, err
	}
	return count > limit, nil
}

func ClearRateLimit(ctx context.Context, q Q, bucket string) error {
	_, err := q.ExecContext(ctx, `DELETE FROM rate_limits WHERE bucket = ?`, bucket)
	return err
}

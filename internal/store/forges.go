package store

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
)

// The forges pull requests are read from (DEV-114), and the pull requests
// whose titles name items.

// Forge is one connection. Neither the token nor the webhook secret is ever
// sent to a client; the secret is shown once, when it is made.
type Forge struct {
	ID            int64  `json:"id"`
	Kind          string `json:"kind"`
	Mode          string `json:"mode"` // poll or webhook
	URL           string `json:"url"`
	Account       string `json:"account"`
	Token         string `json:"-"`
	WebhookSecret string `json:"-"`
	CreatedBy     int64  `json:"createdBy"`
	CreatedAt     int64  `json:"createdAt"`
	SyncedAt      int64  `json:"syncedAt,omitempty"`
	SyncError     string `json:"syncError,omitempty"`
	PollMinutes   int64  `json:"pollMinutes"`
}

func CreateForge(ctx context.Context, q Q, f Forge) (int64, error) {
	res, err := q.ExecContext(ctx, `
		INSERT INTO forges (kind, mode, url, account, token, webhook_secret, poll_minutes, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.Kind, f.Mode, f.URL, f.Account, f.Token, f.WebhookSecret, f.PollMinutes, f.CreatedBy, f.CreatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// SetForgeToken replaces a token that expired or was revoked.
func SetForgeToken(ctx context.Context, q Q, id int64, account, token string) error {
	res, err := q.ExecContext(ctx, `UPDATE forges SET account = ?, token = ? WHERE id = ?`, account, token, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetForgeSecret replaces a webhook secret that was lost or leaked. A polled
// forge has none to replace: ErrNotFound.
func SetForgeSecret(ctx context.Context, q Q, id int64, secret string) error {
	res, err := q.ExecContext(ctx, `UPDATE forges SET webhook_secret = ? WHERE id = ? AND mode = 'webhook'`, secret, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

const forgeColumns = `id, kind, mode, url, account, token, webhook_secret, poll_minutes, created_by, created_at, synced_at, sync_error`

func scanForge(row interface{ Scan(...any) error }) (Forge, error) {
	var f Forge
	var synced sql.NullInt64
	var syncErr sql.NullString
	err := row.Scan(&f.ID, &f.Kind, &f.Mode, &f.URL, &f.Account, &f.Token, &f.WebhookSecret, &f.PollMinutes,
		&f.CreatedBy, &f.CreatedAt, &synced, &syncErr)
	if errors.Is(err, sql.ErrNoRows) {
		return f, ErrNotFound
	}
	f.SyncedAt, f.SyncError = num(synced), text(syncErr)
	return f, err
}

func LoadForge(ctx context.Context, q Q, id int64) (Forge, error) {
	return scanForge(q.QueryRowContext(ctx, `SELECT `+forgeColumns+` FROM forges WHERE id = ?`, id))
}

// ListForges is every forge, oldest first, secrets included.
func ListForges(ctx context.Context, q Q) ([]Forge, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+forgeColumns+` FROM forges ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Forge
	for rows.Next() {
		f, err := scanForge(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// DeleteForge returns the address it was at, or ErrNotFound. Pull requests
// stay on the items they name.
func DeleteForge(ctx context.Context, q Q, id int64) (string, error) {
	var url string
	err := q.QueryRowContext(ctx, `DELETE FROM forges WHERE id = ? RETURNING url`, id).Scan(&url)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return url, err
}

func MarkSynced(ctx context.Context, q Q, id, at int64, syncErr string) error {
	_, err := q.ExecContext(ctx, `UPDATE forges SET synced_at = ?, sync_error = ? WHERE id = ?`,
		at, null(syncErr), id)
	return err
}

// PullRequest is one pull request as an item shows it.
type PullRequest struct {
	ID        int64  `json:"id"`
	Remote    string `json:"-"`
	Repo      string `json:"repo"`
	Number    int64  `json:"number"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	State     string `json:"state"`
	Author    string `json:"author"`
	UpdatedAt int64  `json:"updatedAt"`
}

// Ref is how a pull request is named in the log: acme/web#12.
func (p PullRequest) Ref() string { return p.Repo + "#" + strconv.FormatInt(p.Number, 10) }

// PullRequestByRemote is the stored pull request's id and updated_at, or 0s
// when there is none.
func PullRequestByRemote(ctx context.Context, q Q, remote string) (id, updatedAt int64, err error) {
	err = q.QueryRowContext(ctx, `SELECT id, updated_at FROM pull_requests WHERE remote = ?`, remote).Scan(&id, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, nil
	}
	return id, updatedAt, err
}

// UpsertPullRequest writes a pull request unless the stored copy is newer,
// in one statement (SPEC.md §13.2 rule 1). It returns the pull request's id,
// or 0 when this copy was not written.
func UpsertPullRequest(ctx context.Context, q Q, p PullRequest) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `
		INSERT INTO pull_requests (remote, repo, number, title, url, state, author, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (remote) DO UPDATE SET
		  repo = excluded.repo, number = excluded.number,
		  title = excluded.title, url = excluded.url, state = excluded.state,
		  author = excluded.author, updated_at = excluded.updated_at
		WHERE excluded.updated_at >= pull_requests.updated_at
		RETURNING id`,
		p.Remote, p.Repo, p.Number, p.Title, p.URL, p.State, p.Author, p.UpdatedAt).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

// PullRequestItems is the items a pull request is linked to.
func PullRequestItems(ctx context.Context, q Q, prID int64) ([]int64, error) {
	rows, err := q.QueryContext(ctx, `SELECT item_id FROM item_pull_requests WHERE pull_request_id = ?`, prID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func LinkPullRequest(ctx context.Context, q Q, itemID, prID int64) error {
	_, err := q.ExecContext(ctx,
		`INSERT OR IGNORE INTO item_pull_requests (item_id, pull_request_id) VALUES (?, ?)`, itemID, prID)
	return err
}

func UnlinkPullRequest(ctx context.Context, q Q, itemID, prID int64) error {
	_, err := q.ExecContext(ctx,
		`DELETE FROM item_pull_requests WHERE item_id = ? AND pull_request_id = ?`, itemID, prID)
	return err
}

func DeletePullRequestIfUnlinked(ctx context.Context, q Q, prID int64) error {
	_, err := q.ExecContext(ctx, `DELETE FROM pull_requests WHERE id = ?
		AND NOT EXISTS (SELECT 1 FROM item_pull_requests WHERE pull_request_id = ?)`, prID, prID)
	return err
}

// ItemPullRequests is what an item's detail view lists, newest first.
func ItemPullRequests(ctx context.Context, q Q, itemID int64) ([]PullRequest, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT p.id, p.repo, p.number, p.title, p.url, p.state, p.author, p.updated_at
		FROM item_pull_requests ip JOIN pull_requests p ON p.id = ip.pull_request_id
		WHERE ip.item_id = ? ORDER BY p.updated_at DESC`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PullRequest
	for rows.Next() {
		var p PullRequest
		if err := rows.Scan(&p.ID, &p.Repo, &p.Number, &p.Title, &p.URL, &p.State, &p.Author, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

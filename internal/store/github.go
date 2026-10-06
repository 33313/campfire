package store

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
)

// The GitHub integration's rows (DEV-13): the app, the accounts it is linked
// to, and the pull requests whose titles name items.

type GitHubApp struct {
	ID            int64
	Slug          string
	HTMLURL       string
	Owner         string
	Public        bool
	PrivateKey    string
	WebhookSecret string
	CreatedBy     int64
	CreatedAt     int64
}

// SaveGitHubApp stores an app, replacing one with its id.
func SaveGitHubApp(ctx context.Context, q Q, a GitHubApp) error {
	_, err := q.ExecContext(ctx, `
		INSERT OR REPLACE INTO github_apps
		  (id, slug, html_url, owner, public, private_key, webhook_secret, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Slug, a.HTMLURL, a.Owner, a.Public, a.PrivateKey, a.WebhookSecret, a.CreatedBy, a.CreatedAt)
	return err
}

const appColumns = `id, slug, html_url, owner, public, private_key, webhook_secret, created_by, created_at`

func scanApp(row interface{ Scan(...any) error }) (GitHubApp, error) {
	var a GitHubApp
	err := row.Scan(&a.ID, &a.Slug, &a.HTMLURL, &a.Owner, &a.Public, &a.PrivateKey, &a.WebhookSecret,
		&a.CreatedBy, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}

func LoadGitHubApp(ctx context.Context, q Q, id int64) (GitHubApp, error) {
	return scanApp(q.QueryRowContext(ctx, `SELECT `+appColumns+` FROM github_apps WHERE id = ?`, id))
}

// ListGitHubApps is every app, oldest first, secrets included.
func ListGitHubApps(ctx context.Context, q Q) ([]GitHubApp, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+appColumns+` FROM github_apps ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GitHubApp
	for rows.Next() {
		a, err := scanApp(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// DeleteGitHubApp forgets an app and, by cascade, the accounts linked
// through it. Pull requests stay on the items they name.
func DeleteGitHubApp(ctx context.Context, q Q, id int64) (string, error) {
	var slug string
	err := q.QueryRowContext(ctx, `DELETE FROM github_apps WHERE id = ? RETURNING slug`, id).Scan(&slug)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return slug, err
}

type GitHubInstallation struct {
	ID          int64  `json:"id"`
	AppID       int64  `json:"appId"`
	Account     string `json:"account"`
	AccountType string `json:"accountType"`
	LinkedBy    int64  `json:"linkedBy"`
	LinkedAt    int64  `json:"linkedAt"`
	SyncedAt    int64  `json:"syncedAt,omitempty"`
	SyncError   string `json:"syncError,omitempty"`
}

// LinkInstallation records an installation, or refreshes its account name.
// It reports whether it was new.
func LinkInstallation(ctx context.Context, q Q, i GitHubInstallation) (bool, error) {
	res, err := q.ExecContext(ctx, `
		INSERT INTO github_installations (id, app_id, account, account_type, linked_by, linked_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO NOTHING`, i.ID, i.AppID, i.Account, i.AccountType, i.LinkedBy, i.LinkedAt)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil || n == 1 {
		return n == 1, err
	}
	_, err = q.ExecContext(ctx, `UPDATE github_installations SET account = ?, account_type = ? WHERE id = ?`,
		i.Account, i.AccountType, i.ID)
	return false, err
}

// UnlinkInstallation returns the account it was linked to, or ErrNotFound.
func UnlinkInstallation(ctx context.Context, q Q, id int64) (string, error) {
	var account string
	err := q.QueryRowContext(ctx, `DELETE FROM github_installations WHERE id = ? RETURNING account`, id).Scan(&account)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return account, err
}

const installationColumns = `id, app_id, account, account_type, linked_by, linked_at, synced_at, sync_error`

func scanInstallation(row interface{ Scan(...any) error }) (GitHubInstallation, error) {
	var i GitHubInstallation
	var synced sql.NullInt64
	var syncErr sql.NullString
	err := row.Scan(&i.ID, &i.AppID, &i.Account, &i.AccountType, &i.LinkedBy, &i.LinkedAt, &synced, &syncErr)
	if errors.Is(err, sql.ErrNoRows) {
		return i, ErrNotFound
	}
	i.SyncedAt, i.SyncError = num(synced), text(syncErr)
	return i, err
}

func Installation(ctx context.Context, q Q, id int64) (GitHubInstallation, error) {
	return scanInstallation(q.QueryRowContext(ctx,
		`SELECT `+installationColumns+` FROM github_installations WHERE id = ?`, id))
}

func ListInstallations(ctx context.Context, q Q) ([]GitHubInstallation, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT `+installationColumns+` FROM github_installations ORDER BY account`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GitHubInstallation
	for rows.Next() {
		i, err := scanInstallation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func MarkSynced(ctx context.Context, q Q, id, at int64, syncErr string) error {
	_, err := q.ExecContext(ctx, `UPDATE github_installations SET synced_at = ?, sync_error = ? WHERE id = ?`,
		at, null(syncErr), id)
	return err
}

// PullRequest is one pull request as an item shows it.
type PullRequest struct {
	ID             int64  `json:"id"`
	InstallationID int64  `json:"-"`
	Repo           string `json:"repo"`
	Number         int64  `json:"number"`
	Title          string `json:"title"`
	URL            string `json:"url"`
	State          string `json:"state"`
	Author         string `json:"author"`
	UpdatedAt      int64  `json:"updatedAt"`
}

// Ref is how a pull request is named in the log: acme/web#12.
func (p PullRequest) Ref() string { return p.Repo + "#" + strconv.FormatInt(p.Number, 10) }

// UpsertPullRequest writes a pull request unless the stored copy is newer,
// in one statement (SPEC.md §13.2 rule 1). It returns the state stored before
// and whether this copy was written.
func UpsertPullRequest(ctx context.Context, q Q, p PullRequest) (oldState string, written bool, err error) {
	err = q.QueryRowContext(ctx, `SELECT state FROM pull_requests WHERE id = ?`, p.ID).Scan(&oldState)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", false, err
	}
	res, err := q.ExecContext(ctx, `
		INSERT INTO pull_requests (id, installation_id, repo, number, title, url, state, author, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
		  installation_id = excluded.installation_id, repo = excluded.repo, number = excluded.number,
		  title = excluded.title, url = excluded.url, state = excluded.state,
		  author = excluded.author, updated_at = excluded.updated_at
		WHERE excluded.updated_at >= pull_requests.updated_at`,
		p.ID, null(p.InstallationID), p.Repo, p.Number, p.Title, p.URL, p.State, p.Author, p.UpdatedAt)
	if err != nil {
		return "", false, err
	}
	n, err := res.RowsAffected()
	return oldState, n == 1, err
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

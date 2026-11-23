package store

import "context"

func AddManualPull(ctx context.Context, q Q, itemID int64, p PullRequest, now int64) error {
	_, err := q.ExecContext(ctx, `INSERT INTO manual_pull_requests
		(item_id, url, repo, number, title, state, author, updated_at, checked_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		itemID, p.URL, p.Repo, p.Number, p.Title, p.State, p.Author, p.UpdatedAt, now)
	return err
}

func RemoveManualPull(ctx context.Context, q Q, item, id int64) error {
	res, err := q.ExecContext(ctx, `DELETE FROM manual_pull_requests WHERE item_id = ? AND id = ?`, item, id)
	if err == nil {
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
	}
	return err
}

type ManualPull struct {
	ID, Item, Team int64
	URL            string
}

func DueManualPulls(ctx context.Context, q Q, before int64) ([]ManualPull, error) {
	rows, err := q.QueryContext(ctx, `SELECT m.id, m.item_id, i.team_id, m.url FROM manual_pull_requests m
		JOIN items i ON i.id = m.item_id WHERE m.checked_at <= ? ORDER BY m.checked_at, m.id LIMIT 8`, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ManualPull
	for rows.Next() {
		var p ManualPull
		if err := rows.Scan(&p.ID, &p.Item, &p.Team, &p.URL); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func RefreshManualPull(ctx context.Context, q Q, id int64, p PullRequest, now int64) (bool, error) {
	res, err := q.ExecContext(ctx, `UPDATE manual_pull_requests
		SET repo = ?, number = ?, title = ?, state = ?, author = ?, updated_at = ?, checked_at = ?
		WHERE id = ? AND url = ? AND (repo, number, title, state, author, updated_at) <> (?, ?, ?, ?, ?, ?)`,
		p.Repo, p.Number, p.Title, p.State, p.Author, p.UpdatedAt, now, id, p.URL,
		p.Repo, p.Number, p.Title, p.State, p.Author, p.UpdatedAt)
	if err != nil {
		return false, err
	}
	if n, _ := res.RowsAffected(); n != 0 {
		return true, nil
	}
	_, err = q.ExecContext(ctx, `UPDATE manual_pull_requests SET checked_at = ? WHERE id = ? AND url = ?`, now, id, p.URL)
	return false, err
}

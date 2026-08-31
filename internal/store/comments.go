package store

import (
	"context"
	"database/sql"
	"errors"
)

// Comments are markdown, flat, editable, and may be written by any
// authenticated user regardless of team membership (SPEC.md §5.2, §6.1).

type Comment struct {
	ID         int64  `json:"id"`
	ItemID     int64  `json:"itemId"`
	AuthorID   int64  `json:"authorId"`
	AuthorName string `json:"authorName"`
	Body       string `json:"body"`
	CreatedAt  int64  `json:"createdAt"`
	UpdatedAt  int64  `json:"updatedAt"`
}

func CreateComment(ctx context.Context, q Q, c Comment) (int64, error) {
	res, err := q.ExecContext(ctx, `
		INSERT INTO comments (item_id, author_id, body, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`, c.ItemID, c.AuthorID, c.Body, c.CreatedAt, c.UpdatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func CommentByID(ctx context.Context, q Q, id int64) (Comment, error) {
	var c Comment
	err := q.QueryRowContext(ctx, `
		SELECT c.id, c.item_id, c.author_id, u.username, c.body, c.created_at, c.updated_at
		FROM comments c JOIN users u ON u.id = c.author_id WHERE c.id = ?`, id).
		Scan(&c.ID, &c.ItemID, &c.AuthorID, &c.AuthorName, &c.Body, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Comment{}, ErrNotFound
	}
	return c, err
}

func UpdateComment(ctx context.Context, q Q, id int64, body string, now int64) error {
	_, err := q.ExecContext(ctx,
		`UPDATE comments SET body = ?, updated_at = ? WHERE id = ?`, body, now, id)
	return err
}

func DeleteComment(ctx context.Context, q Q, id int64) error {
	_, err := q.ExecContext(ctx, `DELETE FROM comments WHERE id = ?`, id)
	return err
}

// ListComments is keyset-paginated by id, oldest first: a comment thread is
// read from the top and grows at the bottom.
func ListComments(ctx context.Context, q Q, itemID, afterID int64, limit int) ([]Comment, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT c.id, c.item_id, c.author_id, u.username, c.body, c.created_at, c.updated_at
		FROM comments c JOIN users u ON u.id = c.author_id
		WHERE c.item_id = ? AND c.id > ? ORDER BY c.id LIMIT ?`, itemID, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.ItemID, &c.AuthorID, &c.AuthorName, &c.Body,
			&c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

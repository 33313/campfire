package store

import (
	"context"
	"database/sql"
	"errors"
)

// Pictures pasted into descriptions and comments (DEV-40). Nothing refers to
// one but the Markdown that shows it.

type Image struct {
	ContentType string
	Data        []byte
}

func CreateImage(ctx context.Context, q Q, uploaderID int64, img Image, now int64) (int64, error) {
	res, err := q.ExecContext(ctx, `
		INSERT INTO images (uploader_id, content_type, data, created_at) VALUES (?, ?, ?, ?)`,
		uploaderID, img.ContentType, img.Data, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func ImageByID(ctx context.Context, q Q, id int64) (Image, error) {
	var img Image
	err := q.QueryRowContext(ctx, `SELECT content_type, data FROM images WHERE id = ?`, id).
		Scan(&img.ContentType, &img.Data)
	if errors.Is(err, sql.ErrNoRows) {
		return Image{}, ErrNotFound
	}
	return img, err
}

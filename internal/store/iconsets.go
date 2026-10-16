package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// IconSet is an instance's own set of bucket icons (DEV-79).
type IconSet struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Prefix    string `json:"prefix"`
	Hidden    bool   `json:"hidden"`
	CreatedAt int64  `json:"createdAt"`
	Icons     []Icon `json:"icons"`
}

// Icon is one icon of a set. Its key is what a bucket stores: prefix.name.
type Icon struct {
	Key     string `json:"key"`
	Name    string `json:"-"`
	ViewBox string `json:"viewBox"`
	Body    string `json:"body"`
}

func CreateIconSet(ctx context.Context, q Q, set IconSet, uploaderID int64) (int64, error) {
	res, err := q.ExecContext(ctx, `
		INSERT INTO icon_sets (name, prefix, uploader_id, created_at) VALUES (?, ?, ?, ?)`,
		set.Name, set.Prefix, uploaderID, set.CreatedAt)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	for _, ic := range set.Icons {
		if _, err := q.ExecContext(ctx, `INSERT INTO icons (set_id, name, view_box, body) VALUES (?, ?, ?, ?)`,
			id, ic.Name, ic.ViewBox, ic.Body); err != nil {
			return 0, err
		}
	}
	return id, nil
}

// IconSets lists every set with its icons, oldest set first.
func IconSets(ctx context.Context, q Q) ([]IconSet, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT s.id, s.name, s.prefix, s.hidden, s.created_at, i.name, i.view_box, i.body
		FROM icon_sets s LEFT JOIN icons i ON i.set_id = s.id
		ORDER BY s.id, i.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []IconSet{}
	for rows.Next() {
		var s IconSet
		var name, viewBox, body sql.NullString
		if err := rows.Scan(&s.ID, &s.Name, &s.Prefix, &s.Hidden, &s.CreatedAt, &name, &viewBox, &body); err != nil {
			return nil, err
		}
		if len(out) == 0 || out[len(out)-1].ID != s.ID {
			s.Icons = []Icon{}
			out = append(out, s)
		}
		if name.Valid {
			last := &out[len(out)-1]
			last.Icons = append(last.Icons, Icon{Key: s.Prefix + "." + name.String, Name: name.String,
				ViewBox: viewBox.String, Body: body.String})
		}
	}
	return out, rows.Err()
}

func IconSetByID(ctx context.Context, q Q, id int64) (IconSet, error) {
	var s IconSet
	err := q.QueryRowContext(ctx, `SELECT id, name, prefix, hidden, created_at FROM icon_sets WHERE id = ?`, id).
		Scan(&s.ID, &s.Name, &s.Prefix, &s.Hidden, &s.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return IconSet{}, ErrNotFound
	}
	return s, err
}

func PrefixTaken(ctx context.Context, q Q, prefix string) (bool, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT count(*) FROM icon_sets WHERE prefix = ?`, prefix).Scan(&n)
	return n > 0, err
}

func SetIconSetHidden(ctx context.Context, q Q, id int64, hidden bool) error {
	_, err := q.ExecContext(ctx, `UPDATE icon_sets SET hidden = ? WHERE id = ?`, hidden, id)
	return err
}

func DeleteIconSet(ctx context.Context, q Q, id int64) error {
	_, err := q.ExecContext(ctx, `DELETE FROM icon_sets WHERE id = ?`, id)
	return err
}

// CustomIconExists says whether key names an icon of one of the sets.
func CustomIconExists(ctx context.Context, q Q, key string) (bool, error) {
	prefix, name, ok := strings.Cut(key, ".")
	if !ok {
		return false, nil
	}
	var n int
	err := q.QueryRowContext(ctx, `
		SELECT count(*) FROM icons i JOIN icon_sets s ON s.id = i.set_id
		WHERE s.prefix = ? AND i.name = ?`, prefix, name).Scan(&n)
	return n > 0, err
}

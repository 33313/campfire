package store

import (
	"context"
	"database/sql"
	"errors"
)

// Labels are free text, instance-wide, and created by typing them. They are
// not administered and do not come from a predefined set (SPEC.md §8.1).

type Label struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"createdAt"`
}

// EnsureLabel returns the existing label with that folded name, or creates it.
// One statement, so two clients typing the same label at once cannot both
// insert it (SPEC.md §13.2 rule 1).
func EnsureLabel(ctx context.Context, q Q, name string, now int64) (Label, error) {
	var l Label
	err := q.QueryRowContext(ctx, `
		INSERT INTO labels (name, name_folded, created_at) VALUES (?, ?, ?)
		ON CONFLICT (name_folded) DO UPDATE SET name = labels.name
		RETURNING id, name, created_at`, name, Fold(name), now).
		Scan(&l.ID, &l.Name, &l.CreatedAt)
	return l, err
}

func LabelByName(ctx context.Context, q Q, name string) (Label, error) {
	var l Label
	err := q.QueryRowContext(ctx,
		`SELECT id, name, created_at FROM labels WHERE name_folded = ?`, Fold(name)).
		Scan(&l.ID, &l.Name, &l.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Label{}, ErrNotFound
	}
	return l, err
}

// ListLabels returns every label, for the filter bar's picker. The set is
// small enough — one row per distinct word a team has typed — that it is not
// paginated.
func ListLabels(ctx context.Context, q Q) ([]Label, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, name, created_at FROM labels ORDER BY name_folded`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Label
	for rows.Next() {
		var l Label
		if err := rows.Scan(&l.ID, &l.Name, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func AddItemLabel(ctx context.Context, q Q, itemID, labelID int64) error {
	_, err := q.ExecContext(ctx,
		`INSERT INTO item_labels (item_id, label_id) VALUES (?, ?) ON CONFLICT DO NOTHING`,
		itemID, labelID)
	return err
}

// RemoveItemLabel detaches a label and reaps it if nothing else carries it.
//
// This is the lazy cleanup of SPEC.md §13.2 rule 6 — there are no background
// jobs. A label referenced by a saved view's clause is left alone, because a
// view MUST keep naming a reference that has gone rather than quietly
// widening itself (§9.4).
func RemoveItemLabel(ctx context.Context, q Q, itemID, labelID int64) error {
	if _, err := q.ExecContext(ctx,
		`DELETE FROM item_labels WHERE item_id = ? AND label_id = ?`, itemID, labelID); err != nil {
		return err
	}
	_, err := q.ExecContext(ctx, `
		DELETE FROM labels WHERE id = ?
		  AND NOT EXISTS (SELECT 1 FROM item_labels WHERE label_id = ?)
		  AND NOT EXISTS (SELECT 1 FROM saved_view_refs
		                  WHERE dimension = 'label' AND ref_id = ?)`,
		labelID, labelID, labelID)
	return err
}

func ItemLabels(ctx context.Context, q Q, itemID int64) ([]Label, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT l.id, l.name, l.created_at FROM labels l
		JOIN item_labels il ON il.label_id = l.id
		WHERE il.item_id = ? ORDER BY l.name_folded`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Label
	for rows.Next() {
		var l Label
		if err := rows.Scan(&l.ID, &l.Name, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// LoadLabels fills the Labels field of a page of items in one query, so that a
// board of two hundred cards is two queries and not two hundred.
func LoadLabels(ctx context.Context, q Q, items []Item) error {
	if len(items) == 0 {
		return nil
	}
	byID := make(map[int64]*Item, len(items))
	args := make([]any, 0, len(items))
	for i := range items {
		byID[items[i].ID] = &items[i]
		args = append(args, items[i].ID)
	}
	rows, err := q.QueryContext(ctx, `
		SELECT il.item_id, l.name FROM item_labels il
		JOIN labels l ON l.id = il.label_id
		WHERE il.item_id IN (`+placeholders(len(items))+`)
		ORDER BY l.name_folded`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var itemID int64
		var name string
		if err := rows.Scan(&itemID, &name); err != nil {
			return err
		}
		if it := byID[itemID]; it != nil {
			it.Labels = append(it.Labels, name)
		}
	}
	return rows.Err()
}

package store

import (
	"context"
	"database/sql"
	"errors"
)

// A fixed set of relation types; users cannot define new ones (SPEC.md §8.2).
// Each edge is stored once, directed, and displayed from both ends.
const (
	RelBlocks     = "blocks"
	RelFollowsUp  = "follows_up"
	RelDuplicates = "duplicates"
	RelRelatesTo  = "relates_to"
)

// inverseLabel names the edge as seen from its far end.
var inverseLabel = map[string]string{
	RelBlocks:     "blocked_by",
	RelFollowsUp:  "followed_up_by",
	RelDuplicates: "duplicated_by",
	RelRelatesTo:  RelRelatesTo,
}

func ValidRelationKind(kind string) bool {
	_, ok := inverseLabel[kind]
	return ok
}

// StoredRelation is the edge a kind names, seen from the item it is added
// from: blocked_by is a blocks edge pointing the other way, so it comes back
// as blocks, reversed.
func StoredRelation(kind string) (stored string, reversed, ok bool) {
	if ValidRelationKind(kind) {
		return kind, false, true
	}
	for stored, inverse := range inverseLabel {
		if kind == inverse {
			return stored, true, true
		}
	}
	return "", false, false
}

// Relation is one edge as shown on one item's detail view. Kind is already
// resolved to the direction the viewer is looking from: an item that is
// blocked reports "blocked_by".
//
// A relation carries no lifecycle meaning. A blocked-by edge is information
// and MUST NOT prevent any transition (SPEC.md §8.2).
type Relation struct {
	ID         int64  `json:"id"`
	Kind       string `json:"kind"`
	Outgoing   bool   `json:"outgoing"`
	ItemID     int64  `json:"itemId"`
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
	ItemKind   string `json:"itemKind"`
	StateName  string `json:"stateName,omitempty"`
	StateRole  string `json:"stateRole,omitempty"`
	CreatedAt  int64  `json:"createdAt"`
}

func AddRelation(ctx context.Context, q Q, from, to int64, kind string, by, now int64) (int64, error) {
	res, err := q.ExecContext(ctx, `
		INSERT INTO relations (from_item_id, to_item_id, kind, created_by, created_at)
		VALUES (?, ?, ?, ?, ?)`, from, to, kind, by, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func RelationByID(ctx context.Context, q Q, id int64) (from, to int64, kind string, err error) {
	err = q.QueryRowContext(ctx,
		`SELECT from_item_id, to_item_id, kind FROM relations WHERE id = ?`, id).Scan(&from, &to, &kind)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, "", ErrNotFound
	}
	return
}

func RemoveRelation(ctx context.Context, q Q, id int64) error {
	_, err := q.ExecContext(ctx, `DELETE FROM relations WHERE id = ?`, id)
	return err
}

// ItemRelations returns every edge touching the item, in both directions, each
// labelled as the viewer sees it.
func ItemRelations(ctx context.Context, q Q, itemID int64) ([]Relation, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT r.id, r.kind, r.from_item_id = ?, o.id, ot.short_code, o.number, o.title,
		       o.kind, s.name, s.role, r.created_at
		FROM relations r
		JOIN items o ON o.id = CASE WHEN r.from_item_id = ? THEN r.to_item_id ELSE r.from_item_id END
		JOIN teams ot ON ot.id = o.origin_team_id
		JOIN work_states s ON s.id = o.state_id
		WHERE r.from_item_id = ? OR r.to_item_id = ?
		ORDER BY r.id`, itemID, itemID, itemID, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Relation
	for rows.Next() {
		var rel Relation
		var stored, code string
		var number int64
		var role sql.NullString
		if err := rows.Scan(&rel.ID, &stored, &rel.Outgoing, &rel.ItemID, &code, &number,
			&rel.Title, &rel.ItemKind, &rel.StateName, &role, &rel.CreatedAt); err != nil {
			return nil, err
		}
		rel.Kind = stored
		if !rel.Outgoing {
			rel.Kind = inverseLabel[stored]
		}
		rel.StateRole = text(role)
		rel.Identifier = Identifier(code, number)
		out = append(out, rel)
	}
	return out, rows.Err()
}

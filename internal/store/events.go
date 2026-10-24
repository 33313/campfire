package store

import (
	"context"
	"database/sql"
	"strings"
)

// One append-only table serves every history surface (SPEC.md §10). Rows are
// never edited or deleted, the log is ordered by seq and never by timestamp,
// and anything filterable is a real column — there is no opaque payload to
// query into.

// Event kinds. The client ignores kinds it does not recognise, so adding one
// is not a breaking change (SPEC.md §12.4).
const (
	EvItemCreated      = "item.created"
	EvItemUpdated      = "item.updated"
	EvItemStateChanged = "item.state_changed"
	EvItemDeleted      = "item.deleted"
	EvLabelAdded       = "item.label_added"
	EvLabelRemoved     = "item.label_removed"
	EvRelationAdded    = "item.relation_added"
	EvRelationRemoved  = "item.relation_removed"
	// A pull request naming the item changed (DEV-114). It tells open screens
	// to refetch and is never written to the log: the forge is its record.
	EvPullRequestChanged = "item.pull_request_changed"
	EvCommentCreated     = "comment.created"
	EvCommentUpdated     = "comment.updated"
	EvCommentDeleted     = "comment.deleted"
	EvTeamCreated        = "team.created"
	EvTeamUpdated        = "team.updated"
	EvTeamArchived       = "team.archived"
	EvMemberAdded        = "team.member_added"
	EvMemberRemoved      = "team.member_removed"
	EvStateCreated       = "work_state.created"
	EvStateUpdated       = "work_state.updated"
	EvStateDeleted       = "work_state.deleted"
	EvViewCreated        = "saved_view.created"
	EvViewUpdated        = "saved_view.updated"
	EvViewDeleted        = "saved_view.deleted"
	EvUserCreated        = "user.created"
	EvUserUpdated        = "user.updated"
	EvInstance           = "instance.action"
)

type Event struct {
	Seq          int64  `json:"seq"`
	At           int64  `json:"at"`
	ActorID      int64  `json:"actorId,omitempty"`
	AgentTokenID int64  `json:"agentTokenId,omitempty"`
	Kind         string `json:"kind"`
	Entity       string `json:"entity"`
	EntityID     int64  `json:"entityId,omitempty"`
	ItemID       int64  `json:"itemId,omitempty"`
	TeamID       int64  `json:"teamId,omitempty"`
	Field        string `json:"field,omitempty"`
	OldValue     string `json:"oldValue,omitempty"`
	NewValue     string `json:"newValue,omitempty"`

	// Joined for display. History surfaces name the token beside the actor —
	// "marta (agent: laptop)" — so an agent's work is distinguishable at a
	// glance (SPEC.md §14.4).
	ActorName      string `json:"actorName,omitempty"`
	AgentName      string `json:"agentName,omitempty"`
	ItemIdentifier string `json:"itemIdentifier,omitempty"`
	TeamShortCode  string `json:"teamShortCode,omitempty"`
}

// AppendEvent writes one row. Every mutation calls it, inside the same
// transaction as the mutation it records, so the log cannot disagree with the
// data.
func AppendEvent(ctx context.Context, q Q, e Event) (int64, error) {
	var seq int64
	err := q.QueryRowContext(ctx, `
		INSERT INTO events (at, actor_id, agent_token_id, kind, entity, entity_id,
		                    item_id, team_id, field, old_value, new_value)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING seq`,
		e.At, null(e.ActorID), null(e.AgentTokenID), e.Kind, e.Entity, null(e.EntityID),
		null(e.ItemID), null(e.TeamID), null(e.Field), null(e.OldValue), null(e.NewValue)).Scan(&seq)
	return seq, err
}

// EventFilter scopes a log read. Zero fields mean "unfiltered", which is the
// global log and is for instance administrators only (SPEC.md §10).
type EventFilter struct {
	ItemID  int64
	TeamID  int64
	ActorID int64
	// Field restricts to changes of one field. Anything filterable is a real
	// column (SPEC.md §10), and field is one.
	Field string
	// AgentOnly restricts to actions that arrived on an agent token, which is
	// the question §14.4 exists to answer.
	AgentOnly bool
	// InAnyTeam drops instance-level events, which carry no team and which
	// only the global log shows (SPEC.md §10).
	InAnyTeam bool
}

const eventColumns = `e.seq, e.at, e.actor_id, e.agent_token_id, e.kind, e.entity, e.entity_id,
	e.item_id, e.team_id, e.field, e.old_value, e.new_value,
	u.username, a.name, i.number, ot.short_code, t.short_code`

const eventJoins = `
	FROM events e
	LEFT JOIN users u ON u.id = e.actor_id
	LEFT JOIN agent_tokens a ON a.id = e.agent_token_id
	LEFT JOIN items i ON i.id = e.item_id
	LEFT JOIN teams ot ON ot.id = i.origin_team_id
	LEFT JOIN teams t ON t.id = e.team_id`

// ListEvents returns events older than beforeSeq, newest first, at most limit.
//
// Keyset, never offset (SPEC.md §10): the footer of the log offers "load older
// from 41192", and that number is the cursor. A beforeSeq of 0 starts at the
// newest event.
func ListEvents(ctx context.Context, q Q, f EventFilter, beforeSeq int64, limit int) ([]Event, error) {
	var where []string
	var args []any
	if beforeSeq > 0 {
		where = append(where, "e.seq < ?")
		args = append(args, beforeSeq)
	}
	if f.ItemID != 0 {
		where = append(where, "e.item_id = ?")
		args = append(args, f.ItemID)
	}
	if f.TeamID != 0 {
		where = append(where, "e.team_id = ?")
		args = append(args, f.TeamID)
	}
	if f.ActorID != 0 {
		where = append(where, "e.actor_id = ?")
		args = append(args, f.ActorID)
	}
	if f.Field != "" {
		where = append(where, "e.field = ?")
		args = append(args, f.Field)
	}
	if f.AgentOnly {
		where = append(where, "e.agent_token_id IS NOT NULL")
	}
	if f.InAnyTeam {
		where = append(where, "e.team_id IS NOT NULL")
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}
	args = append(args, limit)

	rows, err := q.QueryContext(ctx,
		`SELECT `+eventColumns+eventJoins+clause+` ORDER BY e.seq DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		var e Event
		var actorID, tokenID, entityID, itemID, teamID, number sql.NullInt64
		var actorName, agentName, originCode, teamCode, field, oldV, newV sql.NullString
		if err := rows.Scan(&e.Seq, &e.At, &actorID, &tokenID, &e.Kind, &e.Entity, &entityID,
			&itemID, &teamID, &field, &oldV, &newV,
			&actorName, &agentName, &number, &originCode, &teamCode); err != nil {
			return nil, err
		}
		e.ActorID, e.AgentTokenID = num(actorID), num(tokenID)
		e.EntityID, e.ItemID, e.TeamID = num(entityID), num(itemID), num(teamID)
		e.Field, e.OldValue, e.NewValue = text(field), text(oldV), text(newV)
		e.ActorName, e.AgentName, e.TeamShortCode = text(actorName), text(agentName), text(teamCode)
		if originCode.Valid && number.Valid {
			e.ItemIdentifier = Identifier(originCode.String, number.Int64)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// LatestSeq is the newest event's sequence number, which is the cursor a
// client opens its stream at.
func LatestSeq(ctx context.Context, q Q) (int64, error) {
	var seq sql.NullInt64
	err := q.QueryRowContext(ctx, `SELECT max(seq) FROM events`).Scan(&seq)
	return num(seq), err
}

// EventsAfter returns events in ascending sequence order, which is how a
// reconnecting client replays what it missed (SPEC.md §12.2).
func EventsAfter(ctx context.Context, q Q, afterSeq int64, limit int) ([]Event, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT e.seq, e.kind, e.item_id, e.team_id, i.number, ot.short_code
		FROM events e
		LEFT JOIN items i ON i.id = e.item_id
		LEFT JOIN teams ot ON ot.id = i.origin_team_id
		WHERE e.seq > ? ORDER BY e.seq LIMIT ?`, afterSeq, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		var itemID, teamID, number sql.NullInt64
		var originCode sql.NullString
		if err := rows.Scan(&e.Seq, &e.Kind, &itemID, &teamID, &number, &originCode); err != nil {
			return nil, err
		}
		e.ItemID, e.TeamID = num(itemID), num(teamID)
		if originCode.Valid && number.Valid {
			e.ItemIdentifier = Identifier(originCode.String, number.Int64)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

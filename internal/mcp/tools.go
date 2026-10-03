package mcp

import (
	"context"
	"encoding/json"

	"github.com/campfire/campfire/internal/core"
	"github.com/campfire/campfire/internal/store"
)

// The tool list is derived from the exposed subset of api/openapi.yaml: one
// tool per operation marked `x-agent: true`, and nothing else (SPEC.md §14.1,
// §14.3).
//
// An agent may do what a member does. It may not do what an administrator
// does, and it may not perform an operation that is irreversible: no creating
// accounts, no team or membership changes, no reassigning a bucket's owning
// team, no hard deletion, no backup or restore, and no issuing or revoking
// tokens. Each of those is excluded by having no tool, not by a rule.

func obj(props map[string]any, required ...string) map[string]any {
	schema := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

var (
	pInt     = map[string]any{"type": "integer"}
	pString  = map[string]any{"type": "string"}
	pBool    = map[string]any{"type": "boolean"}
	pIntList = map[string]any{"type": "array", "items": map[string]any{"type": "integer"}}
	pStrList = map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
	pEmpty   = map[string]any{"type": "object", "properties": map[string]any{}}
	pQuery   = map[string]any{"type": "string", "description": csqlHelp}
)

// decodeInto unmarshals a tool's arguments, refusing unknown fields so that a
// mistyped argument is an error the agent can see rather than a silently
// ignored value.
func decodeInto(args json.RawMessage, v any) error {
	if err := json.Unmarshal(args, v); err != nil {
		return &core.Error{Code: core.CodeInvalid, Message: "the arguments are not valid: " + err.Error()}
	}
	return nil
}

func (s *Server) toolset() []tool {
	svc := s.svc
	return []tool{
		// Reading. Every authenticated user may read every item and comment,
		// so these carry no extra permission of their own.
		{
			Name: "list_teams", Operation: "listTeams",
			Description: "List the teams in this instance, with their short codes.",
			Schema:      obj(map[string]any{"includeArchived": pBool}),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in struct{ IncludeArchived bool }
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return svc.ListTeams(ctx, a, in.IncludeArchived)
			},
		},
		{
			Name: "get_team", Operation: "getTeam",
			Description: "One team by id.",
			Schema:      obj(map[string]any{"teamId": pInt}, "teamId"),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in struct{ TeamID int64 }
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return svc.Team(ctx, a, in.TeamID)
			},
		},
		{
			Name: "list_team_members", Operation: "listMembers",
			Description: "The members of a team and their roles.",
			Schema:      obj(map[string]any{"teamId": pInt}, "teamId"),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in struct{ TeamID int64 }
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return svc.ListMembers(ctx, a, in.TeamID)
			},
		},
		{
			Name: "list_work_states", Operation: "listWorkStates",
			Description: "A team's work states in board order. Exactly one holds each of the " +
				"roles default, accepted, rejected and final; the rest are intermediate.",
			Schema: obj(map[string]any{"teamId": pInt}, "teamId"),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in struct{ TeamID int64 }
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return svc.ListWorkStates(ctx, a, in.TeamID)
			},
		},
		{
			Name: "list_users", Operation: "listUsers",
			Description: "The accounts on this instance, for finding an assignee's id.",
			Schema:      obj(map[string]any{"after": pInt, "limit": pInt}),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in struct {
					After int64
					Limit int
				}
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return svc.ListUsers(ctx, a, in.After, in.Limit)
			},
		},
		{
			Name: "list_labels", Operation: "listLabels",
			Description: "Every label in the instance. Labels are free text, created by typing them.",
			Schema:      pEmpty,
			Call: func(ctx context.Context, a core.Actor, _ json.RawMessage) (any, error) {
				return svc.ListLabels(ctx, a)
			},
		},
		{
			Name: "get_board", Operation: "getBoard",
			Description: "A team's board: one column per work state, excluding the default " +
				"and rejected states. Buckets are items like tickets; add kind = ticket to the " +
				"query to leave them out. Items done for more than three days are hidden unless " +
				"allDone is true.",
			Schema: obj(map[string]any{
				"teamId": pInt, "allDone": pBool, "limit": pInt,
				"query": pQuery, "viewIds": pIntList, "tz": pString,
			}, "teamId"),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in struct {
					TeamID  int64
					AllDone bool
					Limit   int
					core.Filter
				}
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return svc.Board(ctx, a, in.TeamID, in.Filter, in.AllDone, in.Limit)
			},
		},
		{
			Name: "get_backlog", Operation: "getBacklog",
			Description: "A team's items in its default state, in manual rank order. " +
				"Pass the returned cursor back for the next page.",
			Schema: obj(map[string]any{
				"teamId": pInt, "cursor": pString, "limit": pInt,
				"query": pQuery, "viewIds": pIntList, "tz": pString,
			}, "teamId"),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in struct {
					TeamID int64
					Cursor string
					Limit  int
					core.Filter
				}
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return svc.Backlog(ctx, a, in.TeamID, in.Filter, in.Cursor, in.Limit)
			},
		},
		{
			Name: "list_items", Operation: "listItems",
			Description: "Items matching a CSQL query and the saved views given, all and-ed. " +
				"Keyset-paginated; pass the returned cursor back.",
			Schema: obj(map[string]any{
				"cursor": pString, "limit": pInt,
				"query": pQuery, "viewIds": pIntList, "tz": pString,
			}),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in struct {
					core.Filter
					Cursor string
					Limit  int
				}
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return svc.ListItems(ctx, a, in.Filter, in.Cursor, in.Limit)
			},
		},
		{
			Name: "get_item", Operation: "getItem",
			Description: "One item in full, with its labels, relations and bucket progress.",
			Schema:      obj(map[string]any{"itemId": pInt}, "itemId"),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in struct{ ItemID int64 }
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return svc.GetItem(ctx, a, in.ItemID)
			},
		},
		{
			Name: "lookup_item", Operation: "lookupItem",
			Description: "Resolve an identifier such as SOL-42. The identifier does not say " +
				"whether the item is a ticket or a bucket; the kind field does.",
			Schema: obj(map[string]any{"identifier": pString}, "identifier"),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in struct{ Identifier string }
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return svc.GetItemByIdentifier(ctx, a, in.Identifier)
			},
		},
		{
			Name: "list_buckets", Operation: "listBuckets",
			Description: "The bucket index. Scope is participation (owned, plus buckets " +
				"holding one of the team's tickets), owned, or instance.",
			Schema: obj(map[string]any{"teamId": pInt, "scope": pString, "cursor": pString}, "teamId"),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in struct {
					TeamID int64
					Scope  string
					Cursor string
				}
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				if in.Scope == "" {
					in.Scope = core.ScopeParticipation
				}
				cards, cursor, err := svc.BucketIndex(ctx, a, in.TeamID, in.Scope, in.Cursor, 0)
				if err != nil {
					return nil, err
				}
				return map[string]any{"buckets": cards, "cursor": cursor}, nil
			},
		},
		{
			Name: "get_bucket_contents", Operation: "getBucketContents",
			Description: "What is in a bucket, grouped by owning team. A bucket's lifecycle " +
				"is independent of its contents in both directions.",
			Schema: obj(map[string]any{"bucketId": pInt}, "bucketId"),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in struct{ BucketID int64 }
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return svc.BucketContents(ctx, a, in.BucketID)
			},
		},
		{
			Name: "get_item_impact", Operation: "getItemImpact",
			Description: "How many items across how many teams would lose a bucket if it went.",
			Schema:      obj(map[string]any{"itemId": pInt}, "itemId"),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in struct{ ItemID int64 }
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				items, teams, err := svc.ItemImpact(ctx, a, in.ItemID)
				if err != nil {
					return nil, err
				}
				return map[string]int{"items": items, "teams": teams}, nil
			},
		},
		{
			Name: "list_comments", Operation: "listComments",
			Description: "An item's comments, oldest first.",
			Schema:      obj(map[string]any{"itemId": pInt, "after": pInt}, "itemId"),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in struct{ ItemID, After int64 }
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return svc.ListComments(ctx, a, in.ItemID, in.After, 0)
			},
		},
		{
			Name: "get_log", Operation: "getLog",
			Description: "The audit log, newest first, keyset-paginated by sequence number. " +
				"This is how to find what changed: read from your last seq. Scope it to an " +
				"item or a team; the unfiltered global log needs an instance administrator.",
			Schema: obj(map[string]any{
				"itemId": pInt, "teamId": pInt, "actorId": pInt,
				"agentOnly": pBool, "before": pInt, "limit": pInt,
			}),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in struct {
					ItemID, TeamID, ActorID, Before int64
					AgentOnly                       bool
					Limit                           int
				}
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return svc.Log(ctx, a, store.EventFilter{ItemID: in.ItemID, TeamID: in.TeamID,
					ActorID: in.ActorID, AgentOnly: in.AgentOnly}, in.Before, in.Limit)
			},
		},

		// Writing. Each goes through the same handler and the same permission
		// check as its counterpart in the client.
		{
			Name: "create_item", Operation: "createItem",
			Description: "Create a ticket or a bucket in a team you belong to. It enters the " +
				"default state, or the accepted state if accepted is true. A bucket carries " +
				"an icon and optionally a colour from the fixed sets; a ticket has neither " +
				"and may sit in a bucket.",
			Schema: obj(map[string]any{
				"kind":   map[string]any{"type": "string", "enum": []string{"ticket", "bucket"}},
				"teamId": pInt, "title": pString, "description": pString,
				"assigneeId": pInt, "priority": pInt, "bucketId": pInt,
				"icon":       map[string]any{"type": "string", "enum": store.BucketIcons},
				"color":      map[string]any{"type": "string", "enum": store.BucketColours},
				"plannedEnd": pString, "labels": pStrList, "accepted": pBool,
			}, "kind", "teamId", "title"),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in core.NewItem
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return svc.CreateItem(ctx, a, in)
			},
		},
		{
			Name: "update_item", Operation: "updateItem",
			Description: "Edit an item. Omit a field to leave it alone; send zero or an empty " +
				"string to clear it. The reporter and the owning team are not editable.",
			Schema: obj(map[string]any{
				"itemId": pInt, "title": pString, "description": pString,
				"assigneeId": pInt, "priority": pInt, "icon": pString, "color": pString,
				"bucketId": pInt, "plannedEnd": pString,
			}, "itemId"),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in struct {
					ItemID int64 `json:"itemId"`
					core.ItemPatch
				}
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return svc.UpdateItem(ctx, a, in.ItemID, in.ItemPatch)
			},
		},
		{
			Name: "move_item", Operation: "moveItem",
			Description: "Move an item to a work state. Any item may move to any state at any " +
				"time; there is no transition graph. Moving to the rejected state requires a " +
				"reason of cancelled or duplicate.",
			Schema: obj(map[string]any{
				"itemId": pInt, "stateId": pInt,
				"reason": map[string]any{"type": "string", "enum": []string{"cancelled", "duplicate"}},
				"note":   pString,
			}, "itemId", "stateId"),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in struct {
					ItemID, StateID int64
					Reason, Note    string
				}
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return svc.MoveItem(ctx, a, in.ItemID, in.StateID, in.Reason, in.Note)
			},
		},
		{
			Name: "rank_item", Operation: "rankItem",
			Description: "Place an item between two neighbours in a column, by their ids. " +
				"Zero means the end. Give a stateId to move it to another column first.",
			Schema: obj(map[string]any{
				"itemId": pInt, "stateId": pInt, "afterId": pInt, "beforeId": pInt,
			}, "itemId"),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in struct{ ItemID, StateID, AfterID, BeforeID int64 }
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return svc.RankItem(ctx, a, in.ItemID, in.StateID, in.AfterID, in.BeforeID)
			},
		},
		{
			Name: "clone_item", Operation: "cloneItem",
			Description: "Clone a ticket. The copy takes a fresh identifier, enters the default " +
				"state, and carries a relates-to relation back to the original.",
			Schema: obj(map[string]any{"itemId": pInt}, "itemId"),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in struct{ ItemID int64 }
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return svc.Clone(ctx, a, in.ItemID)
			},
		},
		{
			Name: "split_item", Operation: "splitItem",
			Description: "Split a ticket into several, one per title. Each new ticket follows " +
				"up the source. The source is kept unless rejectSource is true, in which case " +
				"it is rejected as cancelled.",
			Schema: obj(map[string]any{
				"itemId": pInt, "titles": pStrList, "rejectSource": pBool,
			}, "itemId", "titles"),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in struct {
					ItemID       int64
					Titles       []string
					RejectSource bool
				}
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return svc.Split(ctx, a, in.ItemID, in.Titles, in.RejectSource)
			},
		},
		{
			Name: "bulk_edit_items", Operation: "bulkEditItems",
			Description: "Give a selection one new value for one field: state, assignee, " +
				"priority or bucket. One field per call. An item you may not edit is refused " +
				"on its own and the rest go through; the result lists what was refused.",
			Schema: obj(map[string]any{
				"itemIds": pIntList,
				"field": map[string]any{"type": "string",
					"enum": []string{"state", "assignee", "priority", "bucket"}},
				"value":  pInt,
				"reason": map[string]any{"type": "string", "enum": []string{"cancelled", "duplicate"}},
				"note":   pString,
			}, "itemIds", "field"),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in core.BulkEdit
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return svc.ApplyBulkEdit(ctx, a, in)
			},
		},
		{
			Name: "add_label", Operation: "addLabel",
			Description: "Add a label to an item, creating the label if it is new.",
			Schema:      obj(map[string]any{"itemId": pInt, "name": pString}, "itemId", "name"),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in struct {
					ItemID int64
					Name   string
				}
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return svc.AddLabel(ctx, a, in.ItemID, in.Name)
			},
		},
		{
			Name: "remove_label", Operation: "removeLabel",
			Description: "Remove a label from an item.",
			Schema:      obj(map[string]any{"itemId": pInt, "name": pString}, "itemId", "name"),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in struct {
					ItemID int64
					Name   string
				}
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return svc.RemoveLabel(ctx, a, in.ItemID, in.Name)
			},
		},
		{
			Name: "add_relation", Operation: "addRelation",
			Description: "Link two items. Kinds are blocks, follows_up, duplicates and " +
				"relates_to. Relations may cross teams and may link a ticket to a bucket. A " +
				"relation is information only: a blocks edge never prevents a move.",
			Schema: obj(map[string]any{
				"itemId": pInt, "toId": pInt,
				"kind": map[string]any{"type": "string",
					"enum": []string{"blocks", "follows_up", "duplicates", "relates_to"}},
			}, "itemId", "toId", "kind"),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in struct {
					ItemID, ToID int64
					Kind         string
				}
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return svc.AddRelation(ctx, a, in.ItemID, in.ToID, in.Kind)
			},
		},
		{
			Name: "remove_relation", Operation: "removeRelation",
			Description: "Remove a relation by its id, as shown on either item.",
			Schema:      obj(map[string]any{"relationId": pInt}, "relationId"),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in struct{ RelationID int64 }
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return nil, svc.RemoveRelation(ctx, a, in.RelationID)
			},
		},
		{
			Name: "add_comment", Operation: "addComment",
			Description: "Comment on any item, in Markdown. Team membership is not needed.",
			Schema:      obj(map[string]any{"itemId": pInt, "body": pString}, "itemId", "body"),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in struct {
					ItemID int64
					Body   string
				}
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return svc.AddComment(ctx, a, in.ItemID, in.Body)
			},
		},
		{
			Name: "update_comment", Operation: "updateComment",
			Description: "Edit a comment you wrote.",
			Schema:      obj(map[string]any{"commentId": pInt, "body": pString}, "commentId", "body"),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in struct {
					CommentID int64
					Body      string
				}
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return svc.UpdateComment(ctx, a, in.CommentID, in.Body)
			},
		},
		{
			Name: "list_saved_views", Operation: "listSavedViews",
			Description: "A team's saved views. A saved view is a named CSQL query; missing " +
				"lists what it names that has been deleted.",
			Schema: obj(map[string]any{"teamId": pInt}, "teamId"),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in struct{ TeamID int64 }
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return svc.ListSavedViews(ctx, a, in.TeamID)
			},
		},
		{
			Name: "create_saved_view", Operation: "createSavedView",
			Description: "Save a CSQL query under a name, owned by one team.",
			Schema: obj(map[string]any{
				"teamId": pInt, "name": pString, "pinBoard": pBool, "pinBacklog": pBool,
				"query": map[string]any{"type": "string", "description": csqlHelp},
			}, "teamId", "name"),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in core.ViewInput
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return svc.CreateSavedView(ctx, a, in)
			},
		},
		{
			Name: "update_saved_view", Operation: "updateSavedView",
			Description: "Rewrite a saved view: its name, its pinning and its query.",
			Schema: obj(map[string]any{
				"viewId": pInt, "teamId": pInt, "name": pString,
				"pinBoard": pBool, "pinBacklog": pBool,
				"query": map[string]any{"type": "string", "description": csqlHelp},
			}, "viewId", "name"),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in struct {
					ViewID int64 `json:"viewId"`
					core.ViewInput
				}
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return svc.UpdateSavedView(ctx, a, in.ViewID, in.ViewInput)
			},
		},
		{
			Name: "delete_saved_view", Operation: "deleteSavedView",
			Description: "Delete a saved view. It holds no work, and what it used to be is in the log.",
			Schema:      obj(map[string]any{"viewId": pInt}, "viewId"),
			Call: func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error) {
				var in struct{ ViewID int64 }
				if err := decodeInto(args, &in); err != nil {
					return nil, err
				}
				return nil, svc.DeleteSavedView(ctx, a, in.ViewID)
			},
		},
	}
}

// csqlHelp is how an agent learns CSQL: the whole language in one breath.
const csqlHelp = "A CSQL query. Conditions are field, operator, value, joined by and, or, not " +
	"and parentheses: assignee in (marta, me) and (bucket = SOL-6 or label = roses) and not priority = 4. " +
	"Fields: assignee, reporter (a username or me), label, bucket (an identifier such as SOL-6, or a " +
	"title in quotes), priority (1-4; also < <= > >=), state (a state's name), team (short code), " +
	"kind (ticket or bucket), " +
	"created, started, completed, due (a day: 2026-09-01, today, -7d, +2w; also < <= > >=), " +
	"text (~ or !~ and a phrase). Operators: = != in (...) not in (...). none is the value of an empty " +
	"field: assignee = none, label = none, due = none. != is exactly not =. #12 names a thing by id. " +
	"Days are read in tz, an IANA zone name, or UTC."

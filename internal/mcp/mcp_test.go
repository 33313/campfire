package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/33313/campfire/internal/core"
	"github.com/33313/campfire/internal/store"
)

type harness struct {
	t      *testing.T
	server *httptest.Server
	svc    *core.Service
	token  string
	admin  core.Actor
	team   *store.Team
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "campfire.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	svc := core.New(db)
	ctx := context.Background()

	u, err := svc.Bootstrap(ctx, "greg", "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	admin := core.Actor{UserID: u.ID, Username: u.Username, IsAdmin: true}
	team, err := svc.CreateTeam(ctx, admin, "Solstice", "SOL")
	if err != nil {
		t.Fatal(err)
	}
	issued, err := svc.IssueAgentToken(ctx, admin, "laptop")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(New(svc, "test"))
	t.Cleanup(srv.Close)
	return &harness{t: t, server: srv, svc: svc, token: issued.Token, admin: admin, team: team}
}

func (h *harness) rpc(method string, params any, token string) map[string]any {
	h.t.Helper()
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": method, "params": params,
	})
	req, _ := http.NewRequest("POST", h.server.URL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return map[string]any{"status": float64(res.StatusCode)}
	}
	var out map[string]any
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		h.t.Fatal(err)
	}
	return out
}

// call runs a tool and returns its decoded result, failing the test if the
// tool reported an error.
func (h *harness) call(name string, args map[string]any) map[string]any {
	h.t.Helper()
	out := h.rpc("tools/call", map[string]any{"name": name, "arguments": args}, h.token)
	result, _ := out["result"].(map[string]any)
	if result == nil {
		h.t.Fatalf("%s: %v", name, out)
	}
	if result["isError"] == true {
		h.t.Fatalf("%s: %s", name, textOf(result))
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(textOf(result)), &decoded); err != nil {
		return map[string]any{"raw": textOf(result)}
	}
	return decoded
}

func (h *harness) callErr(name string, args map[string]any) string {
	h.t.Helper()
	out := h.rpc("tools/call", map[string]any{"name": name, "arguments": args}, h.token)
	result, _ := out["result"].(map[string]any)
	if result == nil || result["isError"] != true {
		h.t.Fatalf("%s was expected to be refused, got %v", name, out)
	}
	return textOf(result)
}

func textOf(result map[string]any) string {
	content, _ := result["content"].([]any)
	if len(content) == 0 {
		return ""
	}
	first, _ := content[0].(map[string]any)
	s, _ := first["text"].(string)
	return s
}

// SPEC.md §14.1: every tool corresponds to exactly one operation in the
// OpenAPI document, and the tool list is derived from that correspondence over
// the exposed subset. A tool without an operation must not exist.
func TestToolsMatchTheExposedOperations(t *testing.T) {
	doc, err := os.ReadFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	exposed := map[string]bool{}
	var operation string
	for _, line := range strings.Split(string(doc), "\n") {
		trimmed := strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(trimmed, "operationId:"); ok {
			operation = strings.TrimSpace(after)
		}
		if trimmed == "x-agent: true" && operation != "" {
			exposed[operation] = true
		}
	}
	if len(exposed) == 0 {
		t.Fatal("no operations are marked x-agent: true")
	}

	s := New(nil, "test")
	have := map[string]bool{}
	for _, op := range s.Operations() {
		if have[op] {
			t.Errorf("two tools claim operation %s", op)
		}
		have[op] = true
	}
	for op := range exposed {
		if !have[op] {
			t.Errorf("operation %s is marked x-agent: true but no tool serves it", op)
		}
	}
	for op := range have {
		if !exposed[op] {
			t.Errorf("a tool claims operation %s, which is not exposed to agents", op)
		}
	}
}

// SPEC.md §14.3: the exclusions are a property of the interface. An excluded
// operation has no tool at all, rather than a rule refusing it.
func TestExcludedOperationsHaveNoTool(t *testing.T) {
	h := newHarness(t)
	out := h.rpc("tools/list", nil, h.token)
	result := out["result"].(map[string]any)
	names := map[string]bool{}
	for _, raw := range result["tools"].([]any) {
		names[raw.(map[string]any)["name"].(string)] = true
	}
	for _, forbidden := range []string{
		"create_user", "create_invitation", "create_password_reset",
		"create_team", "archive_team", "set_member", "remove_member",
		"create_work_state", "update_work_state", "delete_work_state",
		"reassign_bucket", "delete_item", "delete_comment",
		"issue_agent_token", "revoke_agent_token",
		"list_snapshots", "create_snapshot", "restore_snapshot",
	} {
		if names[forbidden] {
			t.Errorf("%s is exposed to agents", forbidden)
		}
	}
	// And calling one by name is "no such tool", not "forbidden".
	out = h.rpc("tools/call", map[string]any{"name": "delete_item",
		"arguments": map[string]any{"itemId": 1}}, h.token)
	rpcErr, _ := out["error"].(map[string]any)
	if rpcErr == nil || !strings.Contains(rpcErr["message"].(string), "no such tool") {
		t.Fatalf("calling an excluded tool gave %v", out)
	}
}

func TestUnauthenticatedCallsAreRefused(t *testing.T) {
	h := newHarness(t)
	if got := h.rpc("tools/list", nil, "")["status"]; got != float64(http.StatusUnauthorized) {
		t.Fatalf("no token: status %v", got)
	}
	if got := h.rpc("tools/list", nil, "not-a-real-token")["status"]; got != float64(http.StatusUnauthorized) {
		t.Fatalf("a nonsense token: status %v", got)
	}
}

// A revoked token stops working on the next call, because no session or
// permission state is held in the process (SPEC.md §14.2).
func TestRevocationTakesEffectImmediately(t *testing.T) {
	h := newHarness(t)
	h.call("list_teams", map[string]any{})
	tokens, err := h.svc.ListAgentTokens(context.Background(), h.admin, h.admin.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.svc.RevokeAgentToken(context.Background(), h.admin, tokens[0].ID); err != nil {
		t.Fatal(err)
	}
	if got := h.rpc("tools/list", nil, h.token)["status"]; got != float64(http.StatusUnauthorized) {
		t.Fatalf("a revoked token still worked: %v", got)
	}
}

func TestInitializeAndToolsList(t *testing.T) {
	h := newHarness(t)
	out := h.rpc("initialize", map[string]any{}, h.token)
	result := out["result"].(map[string]any)
	if result["protocolVersion"] != protocolVersion {
		t.Fatalf("protocol version %v", result["protocolVersion"])
	}
	if result["capabilities"].(map[string]any)["tools"] == nil {
		t.Fatal("the server does not advertise tools")
	}

	out = h.rpc("tools/list", nil, h.token)
	tools := out["result"].(map[string]any)["tools"].([]any)
	if len(tools) < 20 {
		t.Fatalf("only %d tools", len(tools))
	}
	for _, raw := range tools {
		tl := raw.(map[string]any)
		if tl["description"] == "" || tl["inputSchema"] == nil {
			t.Fatalf("tool %v is missing a description or a schema", tl["name"])
		}
	}
}

// An agent does what a member does, through the same handlers.
func TestAnAgentCanRunAnOrdinaryDay(t *testing.T) {
	h := newHarness(t)

	states := h.call("list_work_states", map[string]any{"teamId": h.team.ID})
	var stateList []any
	json.Unmarshal([]byte(states["raw"].(string)), &stateList)
	byRole := map[string]float64{}
	for _, raw := range stateList {
		s := raw.(map[string]any)
		if role, ok := s["role"].(string); ok {
			byRole[role] = s["id"].(float64)
		}
	}

	item := h.call("create_item", map[string]any{
		"kind": "ticket", "teamId": h.team.ID, "title": "cap the pre-fetch buffer",
		"priority": 2, "labels": []string{"stream"},
	})
	if item["identifier"] != "SOL-1" {
		t.Fatalf("created %v", item["identifier"])
	}
	itemID := item["id"].(float64)

	moved := h.call("move_item", map[string]any{"itemId": itemID, "stateId": byRole["accepted"]})
	if moved["stateRole"] != "accepted" {
		t.Fatalf("moved to %v", moved["stateRole"])
	}
	if moved["actualStart"] == nil {
		t.Fatal("the move out of the default state did not start the clock")
	}

	h.call("add_comment", map[string]any{"itemId": itemID, "body": "starting on this"})
	looked := h.call("lookup_item", map[string]any{"identifier": "SOL-1"})
	if looked["id"] != itemID {
		t.Fatal("the permalink did not resolve")
	}

	split := h.call("split_item", map[string]any{
		"itemId": itemID, "titles": []string{"cap it", "fall back to too-far-behind"},
	})
	if len(split["new"].([]any)) != 2 {
		t.Fatalf("split produced %v", split["new"])
	}

	// Rejecting needs a reason, and the tool error says so rather than
	// failing the protocol.
	msg := h.callErr("move_item", map[string]any{"itemId": itemID, "stateId": byRole["rejected"]})
	if !strings.Contains(msg, "reason") {
		t.Fatalf("a reasonless rejection reported %q", msg)
	}
	h.call("move_item", map[string]any{
		"itemId": itemID, "stateId": byRole["rejected"], "reason": "cancelled"})

	// Every mutation records the token it arrived on (SPEC.md §14.4).
	logPage := h.call("get_log", map[string]any{"teamId": h.team.ID, "agentOnly": true})
	events := logPage["events"].([]any)
	if len(events) == 0 {
		t.Fatal("nothing in the log is attributed to the agent")
	}
	for _, raw := range events {
		e := raw.(map[string]any)
		if e["agentName"] != "laptop" || e["actorName"] != "greg" {
			t.Fatalf("event attributed to %v / %v", e["actorName"], e["agentName"])
		}
	}
}

// The agent holds exactly its user's permissions, resolved at the time of the
// call — not the ones the token was issued with.
func TestAnAgentIsBoundedByItsUsersPermissions(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	// A second team the agent's user does not belong to. Their agent can
	// still write there, because an instance administrator acts as a team
	// administrator anywhere — the agent holds the user's permissions, not
	// fewer and not more.
	other, err := h.svc.CreateTeam(ctx, h.admin, "Terra", "TER")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.svc.RemoveMember(ctx, h.admin, other.ID, h.admin.UserID); err != nil {
		t.Fatal(err)
	}
	h.call("create_item", map[string]any{"kind": "ticket", "teamId": other.ID, "title": "as admin"})

	// A plain member of no team cannot write anywhere.
	marta, err := h.svc.CreateUser(ctx, h.admin, "marta", "correct horse battery", "", false)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := h.svc.IssueAgentToken(ctx,
		core.Actor{UserID: marta.ID, Username: marta.Username}, "marta's agent")
	if err != nil {
		t.Fatal(err)
	}
	out := h.rpc("tools/call", map[string]any{
		"name":      "create_item",
		"arguments": map[string]any{"kind": "ticket", "teamId": other.ID, "title": "as an outsider"},
	}, issued.Token)
	result := out["result"].(map[string]any)
	if result["isError"] != true || !strings.Contains(textOf(result), "forbidden") {
		t.Fatalf("an outsider's agent was allowed to write: %s", textOf(result))
	}
	// But reading is instance-wide, for the agent as for the person.
	out = h.rpc("tools/call", map[string]any{
		"name": "list_teams", "arguments": map[string]any{},
	}, issued.Token)
	if out["result"].(map[string]any)["isError"] == true {
		t.Fatal("an outsider's agent could not read")
	}
}

// Every argument a tool advertises is one it reads, and one it does not
// advertise is refused: arguments are decoded strictly.
func TestToolsReadExactlyWhatTheyAdvertise(t *testing.T) {
	h := newHarness(t)
	zero := map[string]any{"integer": 0, "string": "", "boolean": false, "array": []any{}}
	for name, tl := range New(h.svc, "test").tools {
		args := map[string]any{}
		for prop, schema := range tl.Schema["properties"].(map[string]any) {
			args[prop] = zero[schema.(map[string]any)["type"].(string)]
		}
		raw, _ := json.Marshal(args)
		if _, err := tl.Call(context.Background(), core.Actor{}, raw); err != nil &&
			strings.Contains(err.Error(), "the arguments are not valid") {
			t.Errorf("%s does not read what it advertises: %v", name, err)
		}
		args["notAnArgument"] = 1
		raw, _ = json.Marshal(args)
		if _, err := tl.Call(context.Background(), core.Actor{}, raw); err == nil ||
			!strings.Contains(err.Error(), "notAnArgument") {
			t.Errorf("%s accepted an argument it does not have: %v", name, err)
		}
	}
}

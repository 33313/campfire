// Package mcp serves the agent interface: a Model Context Protocol server over
// streamable HTTP, in the same process and on the same origin as the client
// and the HTTP API (SPEC.md §14.2).
//
// It is a second interface over the same handlers, not a separate surface.
// Every tool dispatches into internal/core behind the same validation and the
// same permission checks; none of them reaches storage directly. Reuse of the
// authorisation path is the entire point: a second implementation would be a
// second set of rules to keep in step, and the permission bug would live on
// the interface fewer people look at.
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/33313/campfire/internal/core"
)

// The protocol version this server implements.
const protocolVersion = "2025-06-18"

type Server struct {
	svc     *core.Service
	version string
	tools   map[string]tool
}

// New serves the tools over svc, reporting version as the server's own.
func New(svc *core.Service, version string) *Server {
	s := &Server{svc: svc, version: version, tools: map[string]tool{}}
	for _, t := range s.toolset() {
		s.tools[t.Name] = t
	}
	return s
}

// tool is one operation. Operation is the operationId of its counterpart in
// api/openapi.yaml: a tool without an operation must not exist, because that
// would be behaviour with no written contract (SPEC.md §14.1).
type tool struct {
	Name        string
	Operation   string
	Description string
	Schema      map[string]any
	Call        func(ctx context.Context, a core.Actor, args json.RawMessage) (any, error)
}

// Operations returns the operationId each tool corresponds to, which is what
// holds the tool list to the document.
func (s *Server) Operations() []string {
	out := make([]string, 0, len(s.tools))
	for _, t := range s.tools {
		out = append(out, t.Operation)
	}
	sort.Strings(out)
	return out
}

/* JSON-RPC ---------------------------------------------------------------- */

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

const (
	codeParse          = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternal       = -32603
)

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		// Request and response only. The server never initiates: an agent that
		// wants to know what changed reads the audit log from its last
		// sequence number (SPEC.md §14.5).
		w.Header().Set("Allow", "POST")
		http.Error(w, "the agent interface is request and response only", http.StatusMethodNotAllowed)
		return
	}

	// Authentication is by agent token. Cookies are not used, because an agent
	// has no browser and cannot perform an interactive login (SPEC.md §14.2).
	a, err := s.actor(r)
	if err != nil || a.UserID == 0 {
		w.Header().Set("WWW-Authenticate", `Bearer realm="campfire"`)
		http.Error(w, "an agent token is required", http.StatusUnauthorized)
		return
	}

	var req request
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeRPC(w, response{JSONRPC: "2.0", Error: &rpcError{codeParse, "the body is not valid JSON-RPC"}})
		return
	}
	if req.JSONRPC != "2.0" {
		writeRPC(w, response{JSONRPC: "2.0", ID: req.ID,
			Error: &rpcError{codeInvalidRequest, "jsonrpc must be \"2.0\""}})
		return
	}
	// A notification carries no id and expects no body.
	if len(req.ID) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	result, rpcErr := s.dispatch(r.Context(), a, req)
	writeRPC(w, response{JSONRPC: "2.0", ID: req.ID, Result: result, Error: rpcErr})
}

// actor resolves the bearer token to the user it names.
//
// The token carries no permissions of its own: this resolves the user's
// current permissions at the time of the call, never the ones they had when
// the token was issued, and a revoked token stops working on the next call
// because nothing about it is held in the process.
func (s *Server) actor(r *http.Request) (core.Actor, error) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return core.Actor{}, nil
	}
	return s.svc.ResolveAgentToken(r.Context(), strings.TrimSpace(header[len("Bearer "):]))
}

func (s *Server) dispatch(ctx context.Context, a core.Actor, req request) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		return map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "campfire", "version": s.version},
			"instructions": "Campfire is a kanban work tracker. You act as " + a.Username +
				" with exactly that user's permissions. Items are tickets and buckets in one " +
				"namespace: SOL-42 may be either. Any item may move to any work state at any " +
				"time; rejecting one needs a reason. To find what changed, read the log from " +
				"your last sequence number.",
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": s.list()}, nil
	case "tools/call":
		return s.callTool(ctx, a, req.Params)
	}
	return nil, &rpcError{codeMethodNotFound, "no such method: " + req.Method}
}

func (s *Server) list() []map[string]any {
	names := make([]string, 0, len(s.tools))
	for name := range s.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]map[string]any, 0, len(names))
	for _, name := range names {
		t := s.tools[name]
		out = append(out, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": t.Schema,
		})
	}
	return out
}

func (s *Server) callTool(ctx context.Context, a core.Actor, params json.RawMessage) (any, *rpcError) {
	var req struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &rpcError{codeInvalidParams, "the call is malformed"}
	}
	t, ok := s.tools[req.Name]
	if !ok {
		// A tool that does not exist needs no rule: the operations of
		// SPEC.md §14.3 are excluded by not being here at all, rather than by
		// a second permission model.
		return nil, &rpcError{codeMethodNotFound, "no such tool: " + req.Name}
	}
	if len(req.Arguments) == 0 {
		req.Arguments = json.RawMessage("{}")
	}
	result, err := t.Call(ctx, a, req.Arguments)
	if err != nil {
		// A refused call is a tool error the model can read and act on, not a
		// protocol error.
		return toolResult(errorText(err), true), nil
	}
	body, err := withoutAvatars(result)
	if err != nil {
		return nil, &rpcError{codeInternal, "the result could not be encoded"}
	}
	return toolResult(string(body), false), nil
}

// withoutAvatars encodes a result with every "avatar" key removed, at any
// depth. Team and user pictures are inline data URIs of up to 256KB that an
// agent can't use and that crowd out its context. Only teams and users have
// that key; pictures in descriptions and comments are string values and stay.
func withoutAvatars(result any) ([]byte, error) {
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	dropAvatars(v)
	return json.MarshalIndent(v, "", "  ")
}

func dropAvatars(v any) {
	switch v := v.(type) {
	case map[string]any:
		delete(v, "avatar")
		for _, child := range v {
			dropAvatars(child)
		}
	case []any:
		for _, child := range v {
			dropAvatars(child)
		}
	}
}

func toolResult(text string, isError bool) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"isError": isError,
	}
}

func errorText(err error) string {
	var e *core.Error
	if errors.As(err, &e) {
		if e.Field != "" {
			return string(e.Code) + " (" + e.Field + "): " + e.Message
		}
		return string(e.Code) + ": " + e.Message
	}
	return "internal: something went wrong"
}

func writeRPC(w http.ResponseWriter, res response) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(res)
}

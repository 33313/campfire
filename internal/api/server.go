// Package api serves the HTTP interface described by api/openapi.yaml.
//
// The document is the definition of the interface, not documentation
// generated from it (SPEC.md §11). Every route here has an operation there,
// and every MCP tool has the same operation (§14.1).
package api

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/33313/campfire/internal/core"
	"github.com/33313/campfire/internal/forge"
	"github.com/33313/campfire/internal/stream"
	"github.com/33313/campfire/internal/updates"
)

const sessionCookie = "campfire_session"

type Server struct {
	svc *core.Service
	bus *stream.Bus
	mux *http.ServeMux
	// patterns records every route registered, so that a test can hold the
	// API and api/openapi.yaml to each other. The document is the definition
	// of the interface (SPEC.md §11); a route it does not describe is
	// behaviour with no written contract.
	patterns []string
	// HiddenTabGrace is how long a hidden tab keeps its stream, for the client
	// to read from getInstance (SPEC.md §12.4).
	HiddenTabGrace time.Duration
	Snapshots      Snapshots
	// MaxImage bounds a pasted picture, in bytes; 0 takes none (DEV-40).
	MaxImage int64
	Updates  *updates.Checker
	Forges   *forge.Client
}

func New(svc *core.Service, bus *stream.Bus) *Server {
	s := &Server{svc: svc, bus: bus, mux: http.NewServeMux(), Forges: forge.New()}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Patterns lists every registered route as "METHOD /path".
func (s *Server) Patterns() []string { return s.patterns }

// actor resolves the caller from the session cookie.
//
// Agent tokens are deliberately not accepted here. They authenticate the MCP
// path and nothing else, which is what makes the exclusions of SPEC.md §14.3 a
// property of the interface rather than a second permission model: an
// operation with no tool is unreachable to an agent because it has no route to
// call, not because a rule says no.
func (s *Server) actor(r *http.Request) (core.Actor, error) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return core.Actor{}, nil
	}
	return s.svc.ResolveSession(r.Context(), c.Value)
}

// handler is a route that needs the caller and returns a body or an error.
type handler func(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error)

func (s *Server) handle(pattern string, h handler) {
	s.patterns = append(s.patterns, pattern)
	s.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if !sameOrigin(r) {
			fail(w, &core.Error{Code: core.CodeForbidden,
				Message: "request blocked: it came from another site"})
			return
		}
		a, err := s.actor(r)
		if err != nil {
			fail(w, err)
			return
		}
		body, err := h(w, r, a)
		if err != nil {
			fail(w, err)
			return
		}
		if f, ok := body.(file); ok {
			sendFile(w, f)
			return
		}
		if body == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		reply(w, r, body)
	})
}

// file is a body sent as it is rather than as JSON: a picture.
type file struct {
	contentType string
	data        []byte
}

// sendFile sends a picture. One never changes under its id, so the browser
// keeps it. nosniff and the sandbox keep a browser from taking one for
// anything but a picture, even opened on its own.
func sendFile(w http.ResponseWriter, f file) {
	h := w.Header()
	h.Set("Content-Type", f.contentType)
	h.Set("Cache-Control", "private, max-age=31536000, immutable")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Write(f.data)
}

// secure reports whether the request reached the instance over HTTPS, here or
// at the proxy in front, which is when the session cookie is marked secure. A
// plain-HTTP loopback instance is the individual profile's whole deployment.
func secure(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

// sameOrigin rejects a state-changing request that did not come from a page on
// this instance.
//
// Authentication is a cookie, so a form on another site could otherwise make
// one on the user's behalf. Browsers send Origin on every cross-site request
// with a side effect, so comparing it against Host is the whole check.
func sameOrigin(r *http.Request) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return true
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		// A non-browser client (curl, an agent's proxy) sends no Origin. It
		// also carries no ambient cookie, so there is nothing to forge.
		return true
	}
	i := strings.Index(origin, "://")
	return i >= 0 && origin[i+3:] == r.Host
}

func respond(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		return // the client hung up; there is nowhere left to report it
	}
}

// reply sends a successful body, gzipped when the client takes it.
//
// A GET also carries an ETag, a fingerprint of the body. A client that names
// it again gets 304 and no body, so a refetch that finds nothing new costs a
// few hundred bytes (SPEC.md §12.4). The fingerprint is of the content alone:
// two callers who would be sent the same bytes may share it, and neither
// learns anything the body would not have told them.
func reply(w http.ResponseWriter, r *http.Request, body any) {
	buf, err := json.Marshal(body)
	if err != nil {
		fail(w, err)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("Vary", "Accept-Encoding")
	if r.Method == http.MethodGet {
		sum := sha256.Sum256(buf)
		tag := `W/"` + base64.RawURLEncoding.EncodeToString(sum[:15]) + `"`
		h.Set("ETag", tag)
		if strings.Contains(r.Header.Get("If-None-Match"), tag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	if len(buf) < 1024 || !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Write(buf)
		return
	}
	h.Set("Content-Encoding", "gzip")
	zw := gzips.Get().(*gzip.Writer)
	defer gzips.Put(zw)
	zw.Reset(w)
	zw.Write(buf)
	zw.Close()
}

// A gzip writer carries a few hundred kilobytes of tables; reusing them keeps
// a busy board from allocating them on every refetch.
var gzips = sync.Pool{New: func() any { return gzip.NewWriter(nil) }}

// statusFor maps a core error code to an HTTP status. The mapping lives here
// and nowhere else, so that core stays free of HTTP and MCP can map the same
// codes its own way.
func statusFor(code core.Code) int {
	switch code {
	case core.CodeInvalid:
		return http.StatusBadRequest
	case core.CodeUnauthenticated:
		return http.StatusUnauthorized
	case core.CodeForbidden:
		return http.StatusForbidden
	case core.CodeNotFound:
		return http.StatusNotFound
	case core.CodeConflict:
		return http.StatusConflict
	case core.CodeRateLimited:
		return http.StatusTooManyRequests
	case core.CodeTooFarBehind:
		return http.StatusGone
	}
	return http.StatusInternalServerError
}

func fail(w http.ResponseWriter, err error) {
	var e *core.Error
	if errors.As(err, &e) {
		respond(w, statusFor(e.Code), map[string]any{"error": e})
		return
	}
	respond(w, http.StatusInternalServerError, map[string]any{
		"error": &core.Error{Code: "internal", Message: "something went wrong"}})
}

// decode reads a JSON body, refusing unknown fields so that a typo in a client
// is an error rather than a silently ignored value.
func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return &core.Error{Code: core.CodeInvalid, Message: "the request body is not valid: " + err.Error()}
	}
	return nil
}

// pathID reads an integer path segment.
func pathID(r *http.Request, name string) (int64, error) {
	v, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || v <= 0 {
		return 0, &core.Error{Code: core.CodeNotFound, Message: "not found"}
	}
	return v, nil
}

func queryInt(r *http.Request, name string) int64 {
	v, _ := strconv.ParseInt(r.URL.Query().Get(name), 10, 64)
	return v
}

func queryInts(r *http.Request, name string) []int64 {
	var out []int64
	for _, raw := range r.URL.Query()[name] {
		for _, part := range strings.Split(raw, ",") {
			if v, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64); err == nil {
				out = append(out, v)
			}
		}
	}
	return out
}

func queryBool(r *http.Request, name string) bool {
	v := r.URL.Query().Get(name)
	return v == "1" || v == "true"
}

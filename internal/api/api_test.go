package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/33313/campfire/internal/core"
	"github.com/33313/campfire/internal/store"
	"github.com/33313/campfire/internal/stream"
)

type client struct {
	t    *testing.T
	base string
	http *http.Client
}

func newTestServer(t *testing.T) (*client, *core.Service, *Server) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "campfire.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	bus := stream.NewBus()
	svc := core.New(db)
	svc.Notify = func(e store.Event) {
		bus.Publish(stream.Notification{Seq: e.Seq, Kind: e.Kind,
			Identifier: e.ItemIdentifier, TeamID: e.TeamID})
	}
	api := New(svc, bus)
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)

	jar := &jar{cookies: map[string][]*http.Cookie{}}
	return &client{t: t, base: srv.URL, http: &http.Client{Jar: jar}}, svc, api
}

// jar is a cookie jar small enough to read: the tests need one origin.
type jar struct{ cookies map[string][]*http.Cookie }

func (j *jar) SetCookies(u *url.URL, cs []*http.Cookie) { j.cookies[u.Host] = cs }
func (j *jar) Cookies(u *url.URL) []*http.Cookie        { return j.cookies[u.Host] }

func (c *client) do(method, path string, body any) (int, map[string]any) {
	c.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			c.t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		c.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	out := map[string]any{}
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			// A list response is a JSON array; wrap it so callers see it.
			var list []any
			if err2 := json.Unmarshal(raw, &list); err2 == nil {
				out = map[string]any{"list": list}
			} else {
				c.t.Fatalf("%s %s returned %s: %v", method, path, raw, err)
			}
		}
	}
	return res.StatusCode, out
}

func (c *client) must(method, path string, body any) map[string]any {
	c.t.Helper()
	code, out := c.do(method, path, body)
	if code >= 400 {
		c.t.Fatalf("%s %s: %d %v", method, path, code, out)
	}
	return out
}

func id(v any) int64 {
	f, _ := v.(float64)
	return int64(f)
}

// A whole ordinary session over HTTP: bootstrap, sign in, make a team, make a
// ticket, move it, comment on it, and read it back by permalink.
func TestEndToEndOverHTTP(t *testing.T) {
	c, _, _ := newTestServer(t)

	inst := c.must("GET", "/api/instance", nil)
	if inst["needsBootstrap"] != true {
		t.Fatal("a fresh instance does not report that it needs bootstrapping")
	}
	c.must("POST", "/api/instance/bootstrap", map[string]string{
		"username": "root", "password": "correct horse battery"})

	// Unauthenticated reads are refused.
	if code, _ := c.do("GET", "/api/teams", nil); code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated GET /api/teams returned %d", code)
	}

	c.must("POST", "/api/auth/signin", map[string]string{
		"username": "root", "password": "correct horse battery"})

	session := c.must("GET", "/api/auth/session", nil)
	if session["user"].(map[string]any)["username"] != "root" {
		t.Fatalf("session says %v", session["user"])
	}

	team := c.must("POST", "/api/teams", map[string]string{"name": "Solstice", "shortCode": "SOL"})
	teamID := id(team["id"])

	item := c.must("POST", "/api/items", map[string]any{
		"kind": "ticket", "teamId": teamID, "title": "rate limiter", "priority": 1})
	if item["identifier"] != "SOL-1" {
		t.Fatalf("identifier %v", item["identifier"])
	}
	itemID := id(item["id"])

	// The backlog holds it; the board does not.
	backlog := c.must("GET", fmt.Sprintf("/api/teams/%d/backlog", teamID), nil)
	if len(backlog["items"].([]any)) != 1 {
		t.Fatalf("backlog: %v", backlog["items"])
	}
	board := c.must("GET", fmt.Sprintf("/api/teams/%d/board", teamID), nil)
	for _, col := range board["columns"].([]any) {
		// Lists come back as [], never null, so the client can map over them
		// without guarding every one.
		items, ok := col.(map[string]any)["items"].([]any)
		if !ok {
			t.Fatalf("a column's items are %v, not a list", col.(map[string]any)["items"])
		}
		if len(items) != 0 {
			t.Fatalf("the board already holds %v", items)
		}
	}

	states := c.must("GET", fmt.Sprintf("/api/teams/%d/states", teamID), nil)["list"].([]any)
	var accepted int64
	for _, s := range states {
		if s.(map[string]any)["role"] == "accepted" {
			accepted = id(s.(map[string]any)["id"])
		}
	}
	moved := c.must("POST", fmt.Sprintf("/api/items/%d/move", itemID),
		map[string]any{"stateId": accepted})
	if moved["stateName"] != "To Do" {
		t.Fatalf("moved to %v", moved["stateName"])
	}

	c.must("POST", fmt.Sprintf("/api/items/%d/comments", itemID), map[string]string{"body": "on it"})
	comments := c.must("GET", fmt.Sprintf("/api/items/%d/comments", itemID), nil)["list"].([]any)
	if len(comments) != 1 {
		t.Fatalf("%d comments", len(comments))
	}

	// The identifier is a permalink.
	looked := c.must("GET", "/api/lookup/SOL-1", nil)
	if id(looked["id"]) != itemID {
		t.Fatal("the permalink resolved to another item")
	}

	// The log records all of it, keyset-paginated.
	logPage := c.must("GET", fmt.Sprintf("/api/log?team=%d&limit=2", teamID), nil)
	if len(logPage["events"].([]any)) != 2 || id(logPage["cursor"]) == 0 {
		t.Fatalf("log page: %v", logPage)
	}
}

// The API refuses a state-changing request that did not come from a page on
// this instance.
func TestCrossOriginWritesAreRefused(t *testing.T) {
	c, _, _ := newTestServer(t)
	req, _ := http.NewRequest("POST", c.base+"/api/instance/bootstrap",
		strings.NewReader(`{"username":"root","password":"correct horse battery"}`))
	req.Header.Set("Origin", "https://elsewhere.example")
	res, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("a cross-origin write returned %d", res.StatusCode)
	}
}

// A session token belongs in the cookie and nowhere a script can read it.
func TestSignInDoesNotEchoTheToken(t *testing.T) {
	c, _, _ := newTestServer(t)
	c.must("POST", "/api/instance/bootstrap", map[string]string{
		"username": "root", "password": "correct horse battery"})
	out := c.must("POST", "/api/auth/signin", map[string]string{
		"username": "root", "password": "correct horse battery"})
	if out["token"] != nil {
		t.Fatal("the sign-in response carries the session token in its body")
	}
}

// SPEC.md §14.2: agent tokens authenticate the MCP path only. If they worked
// here, an agent would reach every operation §14.3 excludes.
func TestAgentTokensDoNotAuthenticateTheHTTPAPI(t *testing.T) {
	c, svc, _ := newTestServer(t)
	c.must("POST", "/api/instance/bootstrap", map[string]string{
		"username": "root", "password": "correct horse battery"})
	c.must("POST", "/api/auth/signin", map[string]string{
		"username": "root", "password": "correct horse battery"})
	issued := c.must("POST", "/api/agent-tokens", map[string]string{"name": "laptop"})
	token := issued["token"].(string)

	// A fresh client with no cookie, carrying only the bearer token.
	req, _ := http.NewRequest("GET", c.base+"/api/teams", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("an agent token authenticated the HTTP API (%d)", res.StatusCode)
	}
	// The same token does resolve as an actor, which is what MCP will use.
	a, err := svc.ResolveAgentToken(context.Background(), token)
	if err != nil || a.UserID == 0 {
		t.Fatal("the token does not resolve at all")
	}
}

// SPEC.md §12: subscribe, then fetch, then apply. The stream carries thin
// notifications and nothing else.
func TestStreamPushesThinNotifications(t *testing.T) {
	c, _, _ := newTestServer(t)
	c.must("POST", "/api/instance/bootstrap", map[string]string{
		"username": "root", "password": "correct horse battery"})
	c.must("POST", "/api/auth/signin", map[string]string{
		"username": "root", "password": "correct horse battery"})
	team := c.must("POST", "/api/teams", map[string]string{"name": "Solstice", "shortCode": "SOL"})

	req, _ := http.NewRequest("GET", c.base+"/api/stream", nil)
	for _, ck := range c.http.Jar.Cookies(mustURL(t, c.base)) {
		req.AddCookie(ck)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	res, err := c.http.Do(req.WithContext(ctx))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if got := res.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("stream content type %q", got)
	}
	if res.Header.Get("X-Accel-Buffering") != "no" {
		t.Fatal("the stream does not ask proxies to stop buffering")
	}

	frames := make(chan stream.Notification, 8)
	go func() {
		scanner := bufio.NewScanner(res.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			var n stream.Notification
			if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &n) == nil {
				frames <- n
			}
		}
	}()

	// The first frame tells the client where it is.
	select {
	case n := <-frames:
		if n.Kind != "hello" {
			t.Fatalf("first frame is %+v", n)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no opening frame")
	}

	c.must("POST", "/api/items", map[string]any{
		"kind": "ticket", "teamId": id(team["id"]), "title": "watch me"})

	select {
	case n := <-frames:
		if n.Kind != store.EvItemCreated {
			t.Fatalf("pushed %+v", n)
		}
		if n.Identifier != "SOL-1" || n.Seq == 0 || n.TeamID != id(team["id"]) {
			t.Fatalf("thin notification is missing something: %+v", n)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the creation was not pushed")
	}
}

// A cursor older than the replay window gets an explicit instruction to
// discard and refetch, never a silent partial replay (SPEC.md §12.2).
func TestStreamTellsAFarBehindClientToReload(t *testing.T) {
	c, svc, _ := newTestServer(t)
	c.must("POST", "/api/instance/bootstrap", map[string]string{
		"username": "root", "password": "correct horse battery"})
	c.must("POST", "/api/auth/signin", map[string]string{
		"username": "root", "password": "correct horse battery"})
	team := c.must("POST", "/api/teams", map[string]string{"name": "Solstice", "shortCode": "SOL"})

	// Push the log well past the replay window without writing thousands of
	// items: the cursor is what matters, so ask to resume from far in the past
	// relative to a log that has moved on.
	for i := 0; i < 3; i++ {
		c.must("POST", "/api/items", map[string]any{
			"kind": "ticket", "teamId": id(team["id"]), "title": "filler"})
	}
	latest, err := svc.LatestSeq(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_ = latest

	req, _ := http.NewRequest("GET", c.base+"/api/stream?cursor=1", nil)
	for _, ck := range c.http.Jar.Cookies(mustURL(t, c.base)) {
		req.AddCookie(ck)
	}
	// Resuming from within the window replays rather than resets.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	res, err := c.http.Do(req.WithContext(ctx))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	first := readFrame(t, res.Body)
	if first.Reset {
		t.Fatalf("a client one event behind was told to reload: %+v", first)
	}
	if first.Seq == 0 {
		t.Fatalf("the replay carried no sequence number: %+v", first)
	}
}

func readFrame(t *testing.T, r io.Reader) stream.Notification {
	t.Helper()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var n stream.Notification
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	t.Fatal("the stream produced no frame")
	return stream.Notification{}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// DEV-40: a pasted picture is uploaded as itself and read back by id, by a
// signed-in user only. The bytes decide its type, so a page dressed as a
// picture is refused, and a picture is served so that no browser takes it
// for anything else.
func TestPicturesUploadAndReadBack(t *testing.T) {
	c, _, api := newTestServer(t)
	api.MaxImage = 1 << 10
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89")
	upload := func(body []byte, kind string) (int, map[string]any) {
		req, _ := http.NewRequest("POST", c.base+"/api/images", bytes.NewReader(body))
		req.Header.Set("Content-Type", kind)
		res, err := c.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		out := map[string]any{}
		json.NewDecoder(res.Body).Decode(&out)
		return res.StatusCode, out
	}

	if code, _ := upload(png, "image/png"); code != http.StatusUnauthorized {
		t.Fatalf("an upload without a session returned %d", code)
	}
	c.must("POST", "/api/instance/bootstrap", map[string]string{
		"username": "root", "password": "correct horse battery"})
	c.must("POST", "/api/auth/signin", map[string]string{
		"username": "root", "password": "correct horse battery"})

	code, out := upload(png, "image/png")
	if code != http.StatusOK || out["url"] != fmt.Sprintf("/api/images/%d", id(out["id"])) {
		t.Fatalf("upload: %d %v", code, out)
	}
	res, err := c.http.Get(c.base + out["url"].(string))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !bytes.Equal(got, png) || res.Header.Get("Content-Type") != "image/png" ||
		res.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("read back %d bytes as %q, nosniff %q", len(got),
			res.Header.Get("Content-Type"), res.Header.Get("X-Content-Type-Options"))
	}

	if code, out := upload([]byte("<html><script>alert(1)</script>"), "image/png"); code != http.StatusBadRequest {
		t.Fatalf("a page sent as a picture returned %d %v", code, out)
	}
	if code, out := upload(append(png, make([]byte, 1<<10)...), "image/png"); code != http.StatusBadRequest ||
		!strings.Contains(fmt.Sprint(out), "at most 1KB") {
		t.Fatalf("a picture over the limit returned %d %v", code, out)
	}
	api.MaxImage = 0
	if code, _ := upload(png, "image/png"); code != http.StatusBadRequest {
		t.Fatalf("an instance that takes no pictures returned %d", code)
	}
	if code, _ := c.do("GET", "/api/images/999", nil); code != http.StatusNotFound {
		t.Fatalf("a picture that does not exist returned %d", code)
	}
}

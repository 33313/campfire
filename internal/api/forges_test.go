package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeForgejo plays a Forgejo whose token "tok" (or "tok2") is marta's and
// sees one repository with one pull request. It answers as GitHub Enterprise
// too, under /api/v3.
func fakeForgejo(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a := r.Header.Get("Authorization"); a != "token tok" && a != "token tok2" && a != "Bearer tok" {
			w.WriteHeader(401)
			fmt.Fprint(w, `{"message":"user does not exist"}`)
			return
		}
		switch strings.Replace(r.URL.Path, "/api/v3/", "/api/v1/", 1) {
		case "/api/v1/user":
			fmt.Fprint(w, `{"login":"marta"}`)
		case "/api/v1/user/repos":
			fmt.Fprint(w, `[{"full_name":"acme/web"}]`)
		case "/api/v1/repos/acme/web/pulls":
			fmt.Fprint(w, `[{"id":501,"number":9,"title":"[SOL-1] from a sync","state":"open",
				"html_url":"https://forge.test/acme/web/pulls/9","updated_at":"2099-01-01T00:00:00Z","user":{"login":"marta"}}]`)
		default:
			w.WriteHeader(404)
			fmt.Fprint(w, `{"message":"Not Found"}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func (c *client) deliver(path, secret, body string) int {
	c.t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	req, _ := http.NewRequest("POST", c.base+path, bytes.NewReader([]byte(body)))
	req.Header.Set("X-Forgejo-Event", "pull_request")
	req.Header.Set("X-Forgejo-Signature", hex.EncodeToString(mac.Sum(nil)))
	res, err := http.DefaultClient.Do(req) // a forge carries no session
	if err != nil {
		c.t.Fatal(err)
	}
	res.Body.Close()
	return res.StatusCode
}

func TestForgeToPullRequests(t *testing.T) {
	c, _, srv := newTestServer(t)
	fj := fakeForgejo(t)
	srv.Forges.HTTP = fj.Client()

	c.must("POST", "/api/instance/bootstrap", map[string]string{"username": "root", "password": "correct horse battery"})
	c.must("POST", "/api/auth/signin", map[string]string{"username": "root", "password": "correct horse battery"})
	team := c.must("POST", "/api/teams", map[string]string{"name": "Solstice", "shortCode": "SOL"})
	ticket := c.must("POST", "/api/items", map[string]any{"kind": "ticket", "teamId": id(team["id"]), "title": "one"})
	item := func() []any {
		prs, _ := c.must("GET", "/api/items/"+fmt.Sprint(id(ticket["id"])), nil)["pullRequests"].([]any)
		return prs
	}

	// A token the forge refuses is not kept.
	if code, out := c.do("POST", "/api/forges", map[string]string{"kind": "forgejo", "mode": "webhook", "url": fj.URL, "token": "nope"}); code != 400 {
		t.Fatalf("a bad token got %d %v", code, out)
	}
	f := c.must("POST", "/api/forges", map[string]string{"kind": "forgejo", "mode": "webhook", "url": fj.URL + "/", "token": "tok"})
	if f["account"] != "marta" || f["url"] != fj.URL || f["token"] != nil {
		t.Fatalf("forge %v", f)
	}
	hook, secret := fmt.Sprintf("/api/forges/%d/webhook", id(f["id"])), f["webhookSecret"].(string)

	// A delivery signed with the forge's secret is taken; one that is not is
	// refused.
	pr := `{"action":"opened","repository":{"full_name":"acme/web"},
		"pull_request":{"id":500,"number":1,"title":"SOL-1: the work","state":"open","draft":true,
		"html_url":"https://forge.test/acme/web/pulls/1","updated_at":"2026-10-06T00:00:00Z","user":{"login":"marta"}}}`
	if code := c.deliver(hook, "wrong", pr); code != http.StatusUnauthorized {
		t.Fatalf("a badly signed delivery got %d", code)
	}
	if code := c.deliver(hook, secret, pr); code != http.StatusNoContent {
		t.Fatalf("a delivery got %d", code)
	}
	if prs := item(); len(prs) != 1 || prs[0].(map[string]any)["state"] != "draft" {
		t.Fatalf("the ticket lists %v", prs)
	}

	// A sync finds what no delivery brought.
	synced := c.must("POST", fmt.Sprintf("/api/forges/%d/sync", id(f["id"])), nil)
	if synced["syncError"] != nil || synced["syncedAt"] == nil {
		t.Fatalf("sync %v", synced)
	}
	if len(item()) != 2 {
		t.Fatalf("after a sync the ticket lists %v", item())
	}

	// The secret is shown once, never again.
	if list := c.must("GET", "/api/forges", nil)["list"].([]any); list[0].(map[string]any)["webhookSecret"] != nil {
		t.Fatalf("the list shows the secret: %v", list)
	}
	if code, _ := c.do("PATCH", fmt.Sprintf("/api/forges/%d", id(f["id"])), map[string]string{"token": "nope"}); code != 400 {
		t.Fatal("a bad token replaced a good one")
	}
	if again := c.must("PATCH", fmt.Sprintf("/api/forges/%d", id(f["id"])), map[string]string{"token": "tok2"}); again["webhookSecret"] != nil {
		t.Fatalf("a new token showed the secret: %v", again)
	}

	// Rotated: the old secret stops working, the new one is shown once.
	rotated := c.must("POST", fmt.Sprintf("/api/forges/%d/secret", id(f["id"])), nil)["webhookSecret"].(string)
	if rotated == secret || c.deliver(hook, secret, pr) != http.StatusUnauthorized {
		t.Fatal("the old secret still works")
	}
	secret = rotated
	if code := c.deliver(hook, secret, pr); code != http.StatusNoContent {
		t.Fatalf("a delivery with the new secret got %d", code)
	}

	// Removed: its deliveries go nowhere, and what it listed stays.
	c.must("DELETE", fmt.Sprintf("/api/forges/%d", id(f["id"])), nil)
	if list := c.must("GET", "/api/forges", nil)["list"].([]any); len(list) != 0 {
		t.Fatalf("forges %v", list)
	}
	if code := c.deliver(hook, secret, pr); code != http.StatusNotFound {
		t.Fatalf("a removed forge's delivery got %d", code)
	}
	if len(item()) != 2 {
		t.Fatal("removing the forge dropped the pull requests")
	}
}

// A polled forge is read on a timer, as often as it asked; a webhook one is
// left to its webhook.
func TestPollReadsPolledForges(t *testing.T) {
	c, _, srv := newTestServer(t)
	fake := fakeForgejo(t)
	srv.Forges.HTTP = fake.Client()
	c.must("POST", "/api/instance/bootstrap", map[string]string{"username": "root", "password": "correct horse battery"})
	c.must("POST", "/api/auth/signin", map[string]string{"username": "root", "password": "correct horse battery"})
	team := c.must("POST", "/api/teams", map[string]string{"name": "Solstice", "shortCode": "SOL"})
	c.must("POST", "/api/items", map[string]any{"kind": "ticket", "teamId": id(team["id"]), "title": "one"})
	gh := c.must("POST", "/api/forges", map[string]any{"kind": "github", "mode": "poll", "url": fake.URL, "token": "tok", "pollMinutes": 15})
	hooked := c.must("POST", "/api/forges", map[string]string{"kind": "github", "mode": "webhook", "url": fake.URL, "token": "tok"})
	if gh["webhookSecret"] != nil || hooked["webhookSecret"] == nil {
		t.Fatalf("secrets: polled %v, webhook %v", gh["webhookSecret"], hooked["webhookSecret"])
	}
	// A polled forge refuses deliveries: nothing could sign them.
	if code := c.deliver(fmt.Sprintf("/api/forges/%d/webhook", id(gh["id"])), "", `{}`); code != http.StatusUnauthorized {
		t.Fatalf("a delivery to a polled forge got %d", code)
	}

	synced := func(now time.Time) map[int64]any {
		srv.pollOnce(context.Background(), now)
		out := map[int64]any{}
		for _, f := range c.must("GET", "/api/forges", nil)["list"].([]any) {
			f := f.(map[string]any)
			out[id(f["id"])] = f["syncedAt"]
		}
		return out
	}
	first := synced(time.Now())
	if first[id(gh["id"])] == nil || first[id(hooked["id"])] != nil {
		t.Fatalf("after a poll, synced %v", first)
	}
	// Not due again until its 15 minutes are up.
	if again := synced(time.Now().Add(14 * time.Minute)); again[id(gh["id"])] != first[id(gh["id"])] {
		t.Fatal("polled before it was due")
	}
	if again := synced(time.Now().Add(16 * time.Minute)); again[id(gh["id"])] == first[id(gh["id"])] {
		t.Fatal("not polled when it was due")
	}
}

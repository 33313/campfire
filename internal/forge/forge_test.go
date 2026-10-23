package forge

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestIdentifiers(t *testing.T) {
	for title, want := range map[string][]string{
		"DEV-12: fix the thing":          {"DEV-12"},
		"[DEV-12] fix the thing":         {"DEV-12"},
		"fix the thing DEV-12":           {"DEV-12"},
		"fix the thing [DEV-12]":         {"DEV-12"},
		"fix the thing (DEV-12)":         {"DEV-12"},
		"dev-12 lower case":              {"DEV-12"},
		"[DEV-12][DEV-13] two at once":   {"DEV-12", "DEV-13"},
		"DEV-12, DEV-13: two at once":    {"DEV-12", "DEV-13"},
		"DEV-12 both ends DEV-14":        {"DEV-12", "DEV-14"},
		"DEV-12 says DEV-12 twice":       {"DEV-12"},
		"S2D-7 a code with a digit":      {"S2D-7"},
		"fix DEV-12 in the middle":       nil,
		"nothing at all":                 nil,
		"DEV-0 is not a number":          nil,
		"DEV1-2 ends in a digit":         nil,
		"TOOLONGCODE-1 is nine letters":  nil,
		"":                               nil,
		"DEV-12":                         {"DEV-12"},
		"Bump golang.org/x/net to 0.2.0": nil,
	} {
		if got := Identifiers(title); !reflect.DeepEqual(got, want) {
			t.Errorf("Identifiers(%q) = %q, want %q", title, got, want)
		}
	}
}

func TestAPI(t *testing.T) {
	for f, want := range map[Forge]string{
		{Kind: GitHub, URL: "https://github.com"}:     "https://api.github.com",
		{Kind: GitHub, URL: "https://git.acme.test"}:  "https://git.acme.test/api/v3",
		{Kind: Forgejo, URL: "https://codeberg.org"}:  "https://codeberg.org/api/v1",
		{Kind: GitLab, URL: "https://gitlab.acme.io"}: "https://gitlab.acme.io/api/v4",
	} {
		if got := f.api(); got != want {
			t.Errorf("%+v: %s, want %s", f, got, want)
		}
	}
}

func sign(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestVerify(t *testing.T) {
	body := `{"action":"opened"}`
	good := sign("s3cret", body)
	gh := Forge{Kind: GitHub, Secret: "s3cret"}
	fj := Forge{Kind: Forgejo, Secret: "s3cret"}
	gl := Forge{Kind: GitLab, Secret: "s3cret"}
	for _, c := range []struct {
		f    Forge
		h    map[string]string
		want bool
	}{
		{gh, map[string]string{"X-Hub-Signature-256": "sha256=" + good}, true},
		{gh, map[string]string{"X-Hub-Signature-256": good}, false},
		{gh, map[string]string{"X-Hub-Signature-256": "sha256=" + good[:62] + "00"}, false},
		{gh, map[string]string{}, false},
		{fj, map[string]string{"X-Forgejo-Signature": good, "X-Gitea-Signature": good}, true},
		{fj, map[string]string{"X-Gitea-Signature": good}, true},
		{fj, map[string]string{"X-Forgejo-Signature": sign("other", body)}, false},
		{gl, map[string]string{"X-Gitlab-Token": "s3cret"}, true},
		{gl, map[string]string{"X-Gitlab-Token": "s3cre"}, false},
		{Forge{Kind: GitHub}, map[string]string{"X-Hub-Signature-256": "sha256=" + sign("", body)}, false},
		{Forge{Kind: GitLab}, map[string]string{"X-Gitlab-Token": ""}, false},
	} {
		h := http.Header{}
		for k, v := range c.h {
			h.Set(k, v)
		}
		if got := verify(c.f, h, []byte(body)); got != c.want {
			t.Errorf("%s with %v: %v", c.f.Kind, c.h, got)
		}
	}
}

func TestState(t *testing.T) {
	merged := time.Now()
	for want, p := range map[string]pullJSON{
		"open":   {State: "open"},
		"draft":  {State: "open", Draft: true},
		"closed": {State: "closed"},
		"merged": {State: "closed", MergedAt: &merged},
	} {
		if got := p.value(Forge{}, "a/b").State; got != want {
			t.Errorf("%+v reads %s, want %s", p, got, want)
		}
	}
	// Forgejo says merged in a word.
	if got := (pullJSON{State: "closed", Merged: true}).value(Forge{}, "a/b").State; got != "merged" {
		t.Errorf("merged reads %s", got)
	}
	for want, m := range map[string]mrJSON{
		"open":   {State: "opened"},
		"draft":  {State: "opened", Draft: true},
		"closed": {State: "locked"},
		"merged": {State: "merged"},
	} {
		if got := m.value(Forge{}, "a/b").State; got != want {
			t.Errorf("%+v reads %s, want %s", m, got, want)
		}
	}
}

// fake plays one forge: two repositories, pull requests in one of them
// paged two at a time, and a token it takes in the forge's own header.
func fake(t *testing.T, kind string) (*Client, Forge) {
	var srv *httptest.Server
	pulls := map[string]string{
		GitHub: `[{"id":1,"number":3,"title":"DEV-1: new","state":"open","updated_at":"2026-10-05T00:00:00Z","user":{"login":"marta"}},
			{"id":2,"number":2,"title":"DEV-2: mid","state":"open","draft":true,"updated_at":"2026-10-03T00:00:00Z"}]`,
		GitLab: `[{"id":1,"iid":3,"title":"DEV-1: new","state":"opened","web_url":"w","updated_at":"2026-10-05T00:00:00.000Z","author":{"username":"marta"}},
			{"id":2,"iid":2,"title":"DEV-2: mid","state":"opened","draft":true,"updated_at":"2026-10-03T00:00:00.000Z"}]`,
	}
	pulls[Forgejo] = pulls[GitHub]
	auth := map[string]string{GitHub: "Bearer tok", Forgejo: "token tok", GitLab: "Bearer tok"}[kind]
	root := map[string]string{GitHub: "/api/v3", Forgejo: "/api/v1", GitLab: "/api/v4"}[kind]
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != auth {
			w.WriteHeader(401)
			fmt.Fprint(w, `{"message":"Bad credentials"}`)
			return
		}
		switch strings.TrimPrefix(r.URL.Path, root) {
		case "/user":
			fmt.Fprint(w, `{"login":"marta","username":"marta"}`)
		case "/user/repos":
			fmt.Fprint(w, `[{"full_name":"acme/web"},{"full_name":"acme/api"}]`)
		case "/projects":
			if r.URL.Query().Get("membership") != "true" {
				t.Error("GitLab was asked for every project, not the token's")
			}
			fmt.Fprint(w, `[{"id":10,"path_with_namespace":"acme/web"},{"id":11,"path_with_namespace":"acme/api"}]`)
		case "/repos/acme/web/pulls", "/projects/10/merge_requests":
			if r.URL.Query().Get("page") == "" {
				w.Header().Set("Link", `<`+srv.URL+r.URL.Path+`?page=2>; rel="next"`)
				fmt.Fprint(w, pulls[kind])
				return
			}
			fmt.Fprint(w, `[{"id":3,"number":1,"iid":1,"title":"DEV-3: old","state":"closed","updated_at":"2026-01-01T00:00:00Z"}]`)
		case "/repos/acme/api/pulls", "/projects/11/merge_requests":
			fmt.Fprint(w, `[]`)
		case "/projects/10/merge_requests/3":
			fmt.Fprint(w, `{"id":1,"iid":3,"title":"DEV-1: new","state":"merged","web_url":"w","updated_at":"2026-10-06T00:00:00Z","author":{"username":"marta"}}`)
		default:
			w.WriteHeader(404)
			fmt.Fprint(w, `{"message":"404 Not Found"}`)
		}
	}))
	t.Cleanup(srv.Close)
	return &Client{HTTP: srv.Client()}, Forge{Kind: kind, URL: srv.URL, Token: "tok", Secret: "s3cret"}
}

func TestPullRequestsPagesUntilTheWindowEnds(t *testing.T) {
	since := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for _, kind := range []string{GitHub, Forgejo, GitLab} {
		c, f := fake(t, kind)
		if who, err := c.Account(context.Background(), f); err != nil || who != "marta" {
			t.Errorf("%s: account %q, %v", kind, who, err)
		}
		prs, err := c.PullRequests(context.Background(), f, since)
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if len(prs) != 2 || prs[0].Title != "DEV-1: new" || prs[0].Number != 3 || prs[0].Author != "marta" ||
			prs[1].State != "draft" || prs[0].Repo != "acme/web" || prs[0].Remote != f.URL+"#1" {
			t.Errorf("%s: pull requests %+v", kind, prs)
		}
		f.Token = "wrong"
		if _, err := c.Account(context.Background(), f); err == nil || !strings.Contains(err.Error(), "Bad credentials") {
			t.Errorf("%s: a bad token gave %v", kind, err)
		}
	}
}

func TestDelivery(t *testing.T) {
	pr := `{"action":"closed","repository":{"full_name":"acme/web"},
		"pull_request":{"id":99,"number":4,"title":"DEV-1: x","html_url":"https://forge.test/acme/web/pull/4",
		  "state":"closed","merged":true,"updated_at":"2026-10-01T10:00:00Z","user":{"login":"marta"}}}`
	ctx := context.Background()

	c, f := fake(t, GitHub)
	h := http.Header{"X-Github-Event": {"pull_request"}, "X-Hub-Signature-256": {"sha256=" + sign("s3cret", pr)}}
	got, err := c.Delivery(ctx, f, h, []byte(pr))
	if err != nil || got == nil || got.State != "merged" || got.Repo != "acme/web" || got.Number != 4 ||
		got.Author != "marta" || got.Remote != f.URL+"#99" {
		t.Fatalf("GitHub: %+v, %v", got, err)
	}
	h.Set("X-Github-Event", "push")
	if got, err := c.Delivery(ctx, f, h, []byte(pr)); got != nil || err != nil {
		t.Fatalf("a push was read as %+v, %v", got, err)
	}
	if _, err := c.Delivery(ctx, f, http.Header{}, []byte(pr)); err != ErrSignature {
		t.Fatalf("an unsigned delivery gave %v", err)
	}

	// Forgejo, posting a form, as either forge may be told to.
	c, f = fake(t, Forgejo)
	form := "payload=" + url.QueryEscape(pr)
	h = http.Header{"X-Forgejo-Event": {"pull_request"}, "X-Forgejo-Signature": {sign("s3cret", form)},
		"Content-Type": {"application/x-www-form-urlencoded"}}
	if got, err := c.Delivery(ctx, f, h, []byte(form)); err != nil || got == nil || got.State != "merged" {
		t.Fatalf("Forgejo: %+v, %v", got, err)
	}

	// GitLab names the merge request; it is read back for its author.
	c, f = fake(t, GitLab)
	hook := `{"object_kind":"merge_request","user":{"username":"someone else"},
		"project":{"id":10,"path_with_namespace":"acme/web"},"object_attributes":{"id":1,"iid":3}}`
	h = http.Header{"X-Gitlab-Event": {"Merge Request Hook"}, "X-Gitlab-Token": {"s3cret"}}
	got, err = c.Delivery(ctx, f, h, []byte(hook))
	if err != nil || got == nil || got.State != "merged" || got.Author != "marta" || got.Repo != "acme/web" || got.Number != 3 {
		t.Fatalf("GitLab: %+v, %v", got, err)
	}
	if got, err := c.Delivery(ctx, f, h, []byte(`{"object_kind":"push"}`)); got != nil || err != nil {
		t.Fatalf("a GitLab push was read as %+v, %v", got, err)
	}
}

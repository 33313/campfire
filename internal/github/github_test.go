package github

import (
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestVerify(t *testing.T) {
	body := []byte(`{"action":"opened"}`)
	mac := hmac.New(sha256.New, []byte("s3cret"))
	mac.Write(body)
	good := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if !Verify("s3cret", body, good) {
		t.Fatal("a good signature was refused")
	}
	for _, bad := range []string{"", "sha256=", good[:len(good)-2] + "00", strings.TrimPrefix(good, "sha256=")} {
		if Verify("s3cret", body, bad) {
			t.Errorf("signature %q was accepted", bad)
		}
	}
	if Verify("other", body, good) {
		t.Error("a signature under another secret was accepted")
	}
}

func TestParseEvent(t *testing.T) {
	e, err := ParseEvent("pull_request", []byte(`{"action":"closed","installation":{"id":7},
		"repository":{"full_name":"acme/web"},
		"pull_request":{"id":99,"number":4,"title":"DEV-1: x","html_url":"https://github.com/acme/web/pull/4",
		  "state":"closed","draft":false,"merged_at":"2026-10-01T10:00:00Z","updated_at":"2026-10-01T10:00:00Z",
		  "user":{"login":"marta"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	pr := e.PullRequest
	if e.InstallationID != 7 || pr == nil || pr.State != "merged" || pr.Repo != "acme/web" ||
		pr.Number != 4 || pr.Author != "marta" || pr.InstallationID != 7 {
		t.Fatalf("event %+v, pull request %+v", e, pr)
	}
	e, _ = ParseEvent("installation", []byte(`{"action":"deleted","installation":{"id":7}}`))
	if !e.Removed || e.InstallationID != 7 {
		t.Fatalf("uninstall %+v", e)
	}
	e, _ = ParseEvent("push", []byte(`{"installation":{"id":7}}`))
	if e.PullRequest != nil || e.Removed {
		t.Fatalf("a push was read as %+v", e)
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
		if got := p.value(1, "a/b").State; got != want {
			t.Errorf("%+v reads %s, want %s", p, got, want)
		}
	}
}

func testKey(t *testing.T) (*rsa.PrivateKey, string) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key, string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
}

// The JWT is what GitHub checks; a wrong one fails every call with a 401.
func TestAppJWTIsSignedByTheAppsKey(t *testing.T) {
	key, p := testKey(t)
	jwt, err := appJWT(App{ID: 42, PEM: p})
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		t.Fatalf("jwt %q", jwt)
	}
	sig, _ := base64.RawURLEncoding.DecodeString(parts[2])
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, sum[:], sig); err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var claims struct {
		Iss      string
		Iat, Exp int64
	}
	json.Unmarshal(raw, &claims)
	if claims.Iss != "42" || claims.Exp-claims.Iat > 600 {
		t.Fatalf("claims %+v", claims)
	}
	if _, err := appJWT(App{ID: 1, PEM: "not a key"}); err == nil {
		t.Fatal("a missing key signed a token")
	}
}

// A fake GitHub: one installation on one organisation with two repositories,
// pull requests paged two at a time.
func fakeGitHub(t *testing.T) *Client {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		switch {
		case r.Method == "POST" && r.URL.Path == "/app-manifests/c0de/conversions":
			fmt.Fprint(w, `{"id":42,"slug":"campfire-acme","html_url":"https://github.com/apps/campfire-acme",
				"pem":"KEY","webhook_secret":"s3cret","owner":{"login":"acme"}}`)
		case r.Method == "POST" && r.URL.Path == "/app/installations/7/access_tokens":
			if !strings.HasPrefix(auth, "Bearer ") {
				w.WriteHeader(401)
				return
			}
			fmt.Fprint(w, `{"token":"ghs_x"}`)
		case r.URL.Path == "/app/installations/7":
			fmt.Fprint(w, `{"id":7,"account":{"login":"acme","type":"Organization"}}`)
		case r.URL.Path == "/installation/repositories" && auth == "token ghs_x":
			fmt.Fprint(w, `{"repositories":[{"full_name":"acme/web"},{"full_name":"acme/api"}]}`)
		case r.URL.Path == "/repos/acme/web/pulls" && auth == "token ghs_x":
			if r.URL.Query().Get("page") == "" {
				w.Header().Set("Link", `<`+srv.URL+`/repos/acme/web/pulls?page=2>; rel="next"`)
				fmt.Fprint(w, `[{"id":1,"number":3,"title":"DEV-1: new","state":"open","updated_at":"2026-10-05T00:00:00Z"},
					{"id":2,"number":2,"title":"DEV-2: mid","state":"open","draft":true,"updated_at":"2026-10-03T00:00:00Z"}]`)
				return
			}
			fmt.Fprint(w, `[{"id":3,"number":1,"title":"DEV-3: old","state":"closed","updated_at":"2026-01-01T00:00:00Z"}]`)
		case r.URL.Path == "/repos/acme/api/pulls" && auth == "token ghs_x":
			fmt.Fprint(w, `[]`)
		default:
			w.WriteHeader(404)
			fmt.Fprint(w, `{"message":"Not Found"}`)
		}
	}))
	t.Cleanup(srv.Close)
	return &Client{API: srv.URL, Web: "https://github.example", HTTP: srv.Client()}
}

func TestConvertManifest(t *testing.T) {
	app, err := fakeGitHub(t).ConvertManifest(context.Background(), "c0de")
	if err != nil {
		t.Fatal(err)
	}
	if app.ID != 42 || app.Slug != "campfire-acme" || app.Owner != "acme" || app.PEM != "KEY" || app.WebhookSecret != "s3cret" {
		t.Fatalf("app %+v", app)
	}
	if _, err := fakeGitHub(t).ConvertManifest(context.Background(), "spent"); err == nil ||
		!strings.Contains(err.Error(), "Not Found") {
		t.Fatalf("a bad code gave %v", err)
	}
}

func TestPullRequestsPagesUntilTheWindowEnds(t *testing.T) {
	c := fakeGitHub(t)
	_, p := testKey(t)
	since := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	prs, err := c.PullRequests(context.Background(), App{ID: 42, PEM: p}, 7, since)
	if err != nil {
		t.Fatal(err)
	}
	if len(prs) != 2 || prs[0].Title != "DEV-1: new" || prs[1].State != "draft" || prs[0].Repo != "acme/web" {
		t.Fatalf("pull requests %+v", prs)
	}
	inst, err := c.Installation(context.Background(), App{ID: 42, PEM: p}, 7)
	if err != nil || inst.Account != "acme" || inst.AccountType != "Organization" {
		t.Fatalf("installation %+v, %v", inst, err)
	}
}

func TestURLs(t *testing.T) {
	c := New()
	if got := c.NewAppURL("", "s"); got != "https://github.com/settings/apps/new?state=s" {
		t.Error(got)
	}
	if got := c.NewAppURL("acme", "s"); got != "https://github.com/organizations/acme/settings/apps/new?state=s" {
		t.Error(got)
	}
	if got := c.InstallURL("campfire-acme", "s"); got != "https://github.com/apps/campfire-acme/installations/new?state=s" {
		t.Error(got)
	}
}

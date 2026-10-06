package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/33313/campfire/internal/github"
)

// fakeGitHub plays GitHub for one app installed on two organisations, acme
// (7) and other (8), acme having one repository with one pull request.
func fakeGitHub(t *testing.T) *httptest.Server {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	pemKey := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "POST /app-manifests/c0de/conversions":
			json.NewEncoder(w).Encode(map[string]any{"id": 42, "slug": "campfire-test",
				"html_url": "https://github.com/apps/campfire-test", "pem": pemKey,
				"webhook_secret": "s3cret", "owner": map[string]string{"login": "acme"}})
		case "GET /app/installations":
			fmt.Fprint(w, `[{"id":7,"account":{"login":"acme","type":"Organization"}},
				{"id":8,"account":{"login":"other","type":"Organization"}}]`)
		case "GET /app/installations/7":
			fmt.Fprint(w, `{"id":7,"account":{"login":"acme","type":"Organization"}}`)
		case "POST /app/installations/7/access_tokens":
			fmt.Fprint(w, `{"token":"ghs_x"}`)
		case "GET /installation/repositories":
			fmt.Fprint(w, `{"repositories":[{"full_name":"acme/web"}]}`)
		case "GET /repos/acme/web/pulls":
			fmt.Fprint(w, `[{"id":501,"number":9,"title":"[SOL-1] from a sync","state":"open",
				"html_url":"https://github.com/acme/web/pull/9","updated_at":"2099-01-01T00:00:00Z","user":{"login":"marta"}}]`)
		default:
			w.WriteHeader(404)
			fmt.Fprint(w, `{"message":"Not Found"}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// visit follows nothing: it returns where a navigation was sent.
func (c *client) visit(path string) *url.URL {
	c.t.Helper()
	hc := *c.http
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := hc.Get(c.base + path)
	if err != nil {
		c.t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusSeeOther {
		c.t.Fatalf("GET %s: %d", path, res.StatusCode)
	}
	return mustURL(c.t, res.Header.Get("Location"))
}

func (c *client) deliver(kind, secret, body string) int {
	c.t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	req, _ := http.NewRequest("POST", c.base+"/api/github/webhook", bytes.NewReader([]byte(body)))
	req.Header.Set("X-GitHub-Event", kind)
	req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	res, err := http.DefaultClient.Do(req) // GitHub carries no session
	if err != nil {
		c.t.Fatal(err)
	}
	res.Body.Close()
	return res.StatusCode
}

func TestGitHubFromAppToPullRequest(t *testing.T) {
	c, _, srv := newTestServer(t)
	gh := fakeGitHub(t)
	srv.GitHub = &github.Client{API: gh.URL, Web: "https://github.test", HTTP: gh.Client()}

	c.must("POST", "/api/instance/bootstrap", map[string]string{"username": "root", "password": "correct horse battery"})
	c.must("POST", "/api/auth/signin", map[string]string{"username": "root", "password": "correct horse battery"})
	team := c.must("POST", "/api/teams", map[string]string{"name": "Solstice", "shortCode": "SOL"})
	ticket := c.must("POST", "/api/items", map[string]any{"kind": "ticket", "teamId": id(team["id"]), "title": "one"})

	// The form goes to GitHub with a manifest whose hooks point back here.
	form := c.must("POST", "/api/github/app", map[string]string{"organization": "acme"})
	action := mustURL(t, form["action"].(string))
	if action.Path != "/organizations/acme/settings/apps/new" || !strings.Contains(form["manifest"].(string), c.base+"/api/github/webhook") {
		t.Fatalf("form %v", form)
	}

	// A code without the state this administrator was issued is refused.
	if to := c.visit("/api/github/manifest?code=c0de&state=forged"); to.Query().Get("error") == "" {
		t.Fatalf("a forged state was accepted: %s", to)
	}
	if to := c.visit("/api/github/manifest?code=c0de&state=" + action.Query().Get("state")); to.Query().Get("error") != "" {
		t.Fatalf("creating the app: %s", to.Query().Get("error"))
	}
	status := c.must("GET", "/api/github", nil)
	if status["app"].(map[string]any)["slug"] != "campfire-test" || len(status["available"].([]any)) != 2 {
		t.Fatalf("status %v", status)
	}

	// Installing comes back to setup, which links it.
	install := c.visit("/api/github/install")
	if install.Host != "github.test" || install.Path != "/apps/campfire-test/installations/new" {
		t.Fatalf("install goes to %s", install)
	}
	c.visit("/api/github/setup?installation_id=7&setup_action=install&state=" + install.Query().Get("state"))
	// One arriving without a state links nothing: it may be anyone.
	c.visit("/api/github/setup?installation_id=8&setup_action=install")
	status = c.must("GET", "/api/github", nil)
	if inst := status["installations"].([]any); len(inst) != 1 || inst[0].(map[string]any)["account"] != "acme" {
		t.Fatalf("linked %v", status["installations"])
	}

	// A signed delivery naming the ticket lists the pull request on it; an
	// unsigned one is refused.
	pr := `{"action":"opened","installation":{"id":7},"repository":{"full_name":"acme/web"},
		"pull_request":{"id":500,"number":8,"title":"SOL-1: the work","state":"open","draft":true,
		"html_url":"https://github.com/acme/web/pull/8","updated_at":"2026-10-06T00:00:00Z","user":{"login":"marta"}}}`
	if code := c.deliver("pull_request", "wrong", pr); code != http.StatusUnauthorized {
		t.Fatalf("a badly signed delivery got %d", code)
	}
	if code := c.deliver("pull_request", "s3cret", pr); code != http.StatusNoContent {
		t.Fatalf("a delivery got %d", code)
	}
	item := c.must("GET", "/api/items/"+fmt.Sprint(id(ticket["id"])), nil)
	prs, _ := item["pullRequests"].([]any)
	if len(prs) != 1 || prs[0].(map[string]any)["state"] != "draft" || prs[0].(map[string]any)["repo"] != "acme/web" {
		t.Fatalf("the ticket lists %v", item["pullRequests"])
	}

	// A sync finds what no delivery brought.
	synced := c.must("POST", "/api/github/installations/7/sync", nil)
	if synced["syncError"] != nil || synced["syncedAt"] == nil {
		t.Fatalf("sync %v", synced)
	}
	item = c.must("GET", "/api/items/"+fmt.Sprint(id(ticket["id"])), nil)
	if prs, _ := item["pullRequests"].([]any); len(prs) != 2 {
		t.Fatalf("after a sync the ticket lists %v", item["pullRequests"])
	}

	// Uninstalled on GitHub: unlinked here, and what was listed stays.
	if code := c.deliver("installation", "s3cret", `{"action":"deleted","installation":{"id":7}}`); code != http.StatusNoContent {
		t.Fatalf("an uninstall got %d", code)
	}
	if inst := c.must("GET", "/api/github", nil)["installations"].([]any); len(inst) != 0 {
		t.Fatalf("still linked: %v", inst)
	}
	item = c.must("GET", "/api/items/"+fmt.Sprint(id(ticket["id"])), nil)
	if prs, _ := item["pullRequests"].([]any); len(prs) != 2 {
		t.Fatal("unlinking dropped the pull requests")
	}
}

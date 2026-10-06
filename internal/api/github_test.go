package api

import (
	"bytes"
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
	"net/url"
	"strings"
	"testing"

	"github.com/33313/campfire/internal/github"
)

// fakeGitHub plays GitHub for two private apps: 42 on the organisation
// acme (installation 7, one repository with one pull request), and 43 on the
// account marta (installation 8). It tells the apps apart by their JWT.
func fakeGitHub(t *testing.T) *httptest.Server {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	pemKey := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	app := func(r *http.Request) string {
		parts := strings.Split(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), ".")
		if len(parts) != 3 {
			return ""
		}
		raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
		var claims struct{ Iss string }
		json.Unmarshal(raw, &claims)
		return claims.Iss
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path + " " + app(r) {
		case "POST /app-manifests/acme/conversions ":
			json.NewEncoder(w).Encode(map[string]any{"id": 42, "slug": "campfire-acme",
				"html_url": "https://github.com/apps/campfire-acme", "pem": pemKey,
				"webhook_secret": "s3cret", "owner": map[string]string{"login": "acme"}})
		case "POST /app-manifests/marta/conversions ":
			json.NewEncoder(w).Encode(map[string]any{"id": 43, "slug": "campfire-marta",
				"html_url": "https://github.com/apps/campfire-marta", "pem": pemKey,
				"webhook_secret": "other", "owner": map[string]string{"login": "marta"}})
		case "GET /app/installations 42":
			fmt.Fprint(w, `[{"id":7,"account":{"login":"acme","type":"Organization"}}]`)
		case "GET /app/installations/7 42":
			fmt.Fprint(w, `{"id":7,"account":{"login":"acme","type":"Organization"}}`)
		case "GET /app/installations 43":
			fmt.Fprint(w, `[{"id":8,"account":{"login":"marta","type":"User"}}]`)
		case "GET /app/installations/8 43":
			fmt.Fprint(w, `{"id":8,"account":{"login":"marta","type":"User"}}`)
		case "POST /app/installations/7/access_tokens 42":
			fmt.Fprint(w, `{"token":"ghs_x"}`)
		case "GET /installation/repositories ":
			fmt.Fprint(w, `{"repositories":[{"full_name":"acme/web"}]}`)
		case "GET /repos/acme/web/pulls ":
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

func TestGitHubFromAppsToPullRequests(t *testing.T) {
	c, _, srv := newTestServer(t)
	gh := fakeGitHub(t)
	srv.GitHub = &github.Client{API: gh.URL, Web: "https://github.test", HTTP: gh.Client()}

	c.must("POST", "/api/instance/bootstrap", map[string]string{"username": "root", "password": "correct horse battery"})
	c.must("POST", "/api/auth/signin", map[string]string{"username": "root", "password": "correct horse battery"})
	team := c.must("POST", "/api/teams", map[string]string{"name": "Solstice", "shortCode": "SOL"})
	ticket := c.must("POST", "/api/items", map[string]any{"kind": "ticket", "teamId": id(team["id"]), "title": "one"})
	item := func() []any {
		prs, _ := c.must("GET", "/api/items/"+fmt.Sprint(id(ticket["id"])), nil)["pullRequests"].([]any)
		return prs
	}

	// The form goes to GitHub with a private manifest whose hooks point back
	// here, and whose homepage is the source, not this instance.
	form := c.must("POST", "/api/github/apps", map[string]any{"organization": "acme"})
	action := mustURL(t, form["action"].(string))
	var manifest map[string]any
	json.Unmarshal([]byte(form["manifest"].(string)), &manifest)
	if action.Path != "/organizations/acme/settings/apps/new" || manifest["public"] != false ||
		manifest["url"] != github.Homepage ||
		manifest["hook_attributes"].(map[string]any)["url"] != c.base+"/api/github/webhook" {
		t.Fatalf("form %v", form)
	}

	// A code without the state this administrator was issued is refused.
	if to := c.visit("/api/github/manifest?code=acme&state=forged"); to.Query().Get("error") == "" {
		t.Fatalf("a forged state was accepted: %s", to)
	}
	// With it, the app is kept and installing it is next; installing comes
	// back to setup, which links it.
	install := c.visit("/api/github/manifest?code=acme&state=" + action.Query().Get("state"))
	if install.Host != "github.test" || install.Path != "/apps/campfire-acme/installations/new" {
		t.Fatalf("after creating, sent to %s", install)
	}
	c.visit("/api/github/setup?installation_id=7&setup_action=install&state=" + install.Query().Get("state"))

	// A second private app, on a personal account.
	form = c.must("POST", "/api/github/apps", map[string]any{})
	if mustURL(t, form["action"].(string)).Path != "/settings/apps/new" {
		t.Fatalf("form %v", form)
	}
	install = c.visit("/api/github/manifest?code=marta&state=" + mustURL(t, form["action"].(string)).Query().Get("state"))
	// One arriving without a state links nothing: it may be anyone.
	c.visit("/api/github/setup?installation_id=8&setup_action=install")
	apps := c.must("GET", "/api/github", nil)["apps"].([]any)
	if len(apps) != 2 {
		t.Fatalf("apps %v", apps)
	}
	acme, marta := apps[0].(map[string]any), apps[1].(map[string]any)
	if len(acme["installations"].([]any)) != 1 || len(marta["installations"].([]any)) != 0 ||
		len(marta["available"].([]any)) != 1 {
		t.Fatalf("acme %v, marta %v", acme, marta)
	}
	// Linking it from the list asks each app whose it is.
	linked := c.must("PUT", "/api/github/installations/8", nil)
	if id(linked["appId"]) != 43 || linked["account"] != "marta" {
		t.Fatalf("linked %v", linked)
	}
	if code, _ := c.do("PUT", "/api/github/installations/999", nil); code < 400 {
		t.Fatal("an installation no app here has was linked")
	}

	// A delivery signed by either app's secret is taken; one signed by
	// neither is refused.
	pr := func(id, inst int) string {
		return fmt.Sprintf(`{"action":"opened","installation":{"id":%d},"repository":{"full_name":"x/y"},
			"pull_request":{"id":%d,"number":%d,"title":"SOL-1: the work","state":"open","draft":true,
			"html_url":"https://github.com/x/y/pull/1","updated_at":"2026-10-06T00:00:00Z","user":{"login":"marta"}}}`, inst, id, id)
	}
	if code := c.deliver("pull_request", "wrong", pr(500, 7)); code != http.StatusUnauthorized {
		t.Fatalf("a badly signed delivery got %d", code)
	}
	if code := c.deliver("pull_request", "s3cret", pr(500, 7)); code != http.StatusNoContent {
		t.Fatalf("acme's delivery got %d", code)
	}
	if code := c.deliver("pull_request", "other", pr(510, 8)); code != http.StatusNoContent {
		t.Fatalf("marta's delivery got %d", code)
	}
	if prs := item(); len(prs) != 2 || prs[0].(map[string]any)["state"] != "draft" {
		t.Fatalf("the ticket lists %v", prs)
	}

	// A sync finds what no delivery brought.
	synced := c.must("POST", "/api/github/installations/7/sync", nil)
	if synced["syncError"] != nil || synced["syncedAt"] == nil {
		t.Fatalf("sync %v", synced)
	}
	if len(item()) != 3 {
		t.Fatalf("after a sync the ticket lists %v", item())
	}

	// Uninstalled on GitHub: unlinked here. Forgetting an app unlinks its
	// accounts. What was listed stays either way.
	if code := c.deliver("installation", "s3cret", `{"action":"deleted","installation":{"id":7}}`); code != http.StatusNoContent {
		t.Fatalf("an uninstall got %d", code)
	}
	c.must("DELETE", "/api/github/apps/43", nil)
	apps = c.must("GET", "/api/github", nil)["apps"].([]any)
	if len(apps) != 1 || len(apps[0].(map[string]any)["installations"].([]any)) != 0 {
		t.Fatalf("apps %v", apps)
	}
	if len(item()) != 3 {
		t.Fatal("unlinking dropped the pull requests")
	}
}

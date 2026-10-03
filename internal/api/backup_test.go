package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/campfire/campfire/internal/backup"
	"github.com/campfire/campfire/internal/store"
)

func signedInAdmin(t *testing.T) (*client, *Server) {
	t.Helper()
	c, _, api := newTestServer(t)
	c.must("POST", "/api/instance/bootstrap", map[string]string{
		"username": "root", "password": "correct horse battery"})
	c.must("POST", "/api/auth/signin", map[string]string{
		"username": "root", "password": "correct horse battery"})
	return c, api
}

// The snapshot routes go through the same door as every other route: a write
// from another site is refused before anything is taken.
func TestSnapshotsRefuseCrossOriginWrites(t *testing.T) {
	c, api := signedInAdmin(t)
	taken := 0
	api.Snapshots.Take = func(context.Context) (backup.Snapshot, error) {
		taken++
		return backup.Snapshot{Path: "s.db"}, nil
	}

	req, _ := http.NewRequest("POST", c.base+"/api/snapshots", nil)
	req.Header.Set("Origin", "https://elsewhere.example")
	res, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden || taken != 0 {
		t.Fatalf("a cross-origin snapshot returned %d and took %d", res.StatusCode, taken)
	}
	c.must("POST", "/api/snapshots", nil)
	if taken != 1 {
		t.Fatalf("a same-origin snapshot took %d", taken)
	}
}

// A restore names a snapshot in the snapshot directory and nothing else: a
// path elsewhere is read as the name of one there.
func TestRestoreOnlyNamesASnapshotInTheDirectory(t *testing.T) {
	c, api := signedInAdmin(t)
	api.Snapshots.Dir = t.TempDir()
	elsewhere := filepath.Join(t.TempDir(), "s.db")
	db, err := store.Open(elsewhere)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	if code, _ := c.do("POST", "/api/restore", map[string]string{"path": elsewhere}); code != http.StatusBadRequest {
		t.Fatalf("a snapshot outside the directory returned %d", code)
	}
	raw, _ := os.ReadFile(elsewhere)
	os.WriteFile(filepath.Join(api.Snapshots.Dir, "s.db"), raw, 0o600)
	out := c.must("POST", "/api/restore", map[string]string{"path": elsewhere})
	if want := "campfire restore " + filepath.Join(api.Snapshots.Dir, "s.db"); out["command"] != want {
		t.Fatalf("command %q, want %q", out["command"], want)
	}
}

// The session cookie is secure whenever the request arrived over HTTPS, here
// or at the proxy in front.
func TestSessionCookieIsSecureOverHTTPS(t *testing.T) {
	c, _, _ := newTestServer(t)
	c.must("POST", "/api/instance/bootstrap", map[string]string{
		"username": "root", "password": "correct horse battery"})
	for proto, want := range map[string]bool{"": false, "https": true} {
		req, _ := http.NewRequest("POST", c.base+"/api/auth/signin",
			strings.NewReader(`{"username":"root","password":"correct horse battery"}`))
		if proto != "" {
			req.Header.Set("X-Forwarded-Proto", proto)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		cookies := res.Cookies()
		if len(cookies) != 1 || cookies[0].Secure != want {
			t.Fatalf("X-Forwarded-Proto %q: cookies %v, want secure %v", proto, cookies, want)
		}
	}
}

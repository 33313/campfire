package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/33313/campfire/internal/backup"
	"github.com/33313/campfire/internal/store"
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

// The list says which schema version each snapshot holds, beside the
// binary's own, so a snapshot the binary would refuse is visible before a
// restore is tried.
func TestSnapshotListCarriesSchemaVersions(t *testing.T) {
	c, api := signedInAdmin(t)
	api.Snapshots.Dir = t.TempDir()
	// The newer file is this binary's; the older one predates recorded
	// versions, holds 0, and has the v1 schema.
	for name, recorded := range map[string]int{"campfire-20260102-000000.db": -1, "campfire-20260101-000000.db": 0} {
		db, err := store.Open(filepath.Join(api.Snapshots.Dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if recorded >= 0 {
			if _, err := db.W.Exec(fmt.Sprintf("PRAGMA user_version = %d", recorded)); err != nil {
				t.Fatal(err)
			}
		}
		db.Close()
	}

	out := c.must("GET", "/api/snapshots", nil)
	snaps, _ := out["snapshots"].([]any)
	if len(snaps) != 2 {
		t.Fatalf("snapshots %v", out["snapshots"])
	}
	newest, oldest := id(snaps[0].(map[string]any)["version"]), id(snaps[1].(map[string]any)["version"])
	if newest != int64(store.Version()) || id(out["version"]) != int64(store.Version()) || oldest != 1 {
		t.Fatalf("snapshot versions %d and %d, binary %v; want %d, 1 and %d",
			newest, oldest, out["version"], store.Version(), store.Version())
	}
}

// A delete names a snapshot the list shows, and nothing else: not a path out
// of the directory, and not another file in it.
func TestDeleteSnapshotOnlyRemovesAListedSnapshot(t *testing.T) {
	c, api := signedInAdmin(t)
	api.Snapshots.Dir = t.TempDir()
	snap := filepath.Join(api.Snapshots.Dir, "campfire-20260101-000000.db")
	kept := filepath.Join(api.Snapshots.Dir, "before-migration-1.db")
	for _, path := range []string{snap, kept} {
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	for _, name := range []string{"before-migration-1.db", "..%2Fcampfire.db", "nothing.db"} {
		if code, _ := c.do("DELETE", "/api/snapshots/"+name, nil); code != http.StatusNotFound {
			t.Errorf("deleting %s returned %d, want 404", name, code)
		}
	}
	if _, err := os.Stat(kept); err != nil {
		t.Fatalf("a file the list does not show was removed: %v", err)
	}

	c.must("DELETE", "/api/snapshots/"+filepath.Base(snap), nil)
	if _, err := os.Stat(snap); !os.IsNotExist(err) {
		t.Fatalf("the snapshot is still there: %v", err)
	}
}

// Deleting a snapshot is for an instance administrator only.
func TestDeleteSnapshotNeedsAnInstanceAdmin(t *testing.T) {
	c, api := signedInAdmin(t)
	api.Snapshots.Dir = t.TempDir()
	snap := filepath.Join(api.Snapshots.Dir, "campfire-20260101-000000.db")
	if err := os.WriteFile(snap, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	c.must("POST", "/api/users", map[string]any{"username": "marta", "password": "correct horse battery"})
	c.must("POST", "/api/auth/signout", nil)
	c.must("POST", "/api/auth/signin", map[string]string{"username": "marta", "password": "correct horse battery"})

	if code, _ := c.do("DELETE", "/api/snapshots/"+filepath.Base(snap), nil); code != http.StatusForbidden {
		t.Fatalf("a member's delete returned %d, want 403", code)
	}
	if _, err := os.Stat(snap); err != nil {
		t.Fatalf("a member removed a snapshot: %v", err)
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

// A moment is restored to from the replica, which an instance without
// snapshots.replica does not have.
func TestRestoreAtNeedsAReplica(t *testing.T) {
	c, _ := signedInAdmin(t)
	code, out := c.do("POST", "/api/restore", map[string]string{"at": "2026-10-03T12:00:00Z"})
	if code != http.StatusBadRequest || !strings.Contains(fmt.Sprint(out), "replica") {
		t.Fatalf("restore to a moment with no replica: %d %v", code, out)
	}
}

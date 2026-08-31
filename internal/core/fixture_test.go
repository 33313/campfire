package core

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/campfire/campfire/internal/auth"

	"github.com/campfire/campfire/internal/store"
)

type fixture struct {
	*Service
	t      *testing.T
	ctx    context.Context
	admin  Actor
	events []store.Event
}

// newFixture opens a fresh database with one instance administrator signed in.
// Every other actor in a test is created through the ordinary operations, so
// the tests exercise the same path the interfaces do.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "campfire.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	f := &fixture{t: t, ctx: context.Background(), Service: New(db)}
	f.Notify = func(e store.Event) { f.events = append(f.events, e) }

	// The first account is made directly: CreateUser needs an administrator,
	// and an empty instance has none. This is the same bootstrap first run
	// performs (SPEC.md §13.1).
	u, err := f.Bootstrap(f.ctx, "root", "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	f.admin = Actor{UserID: u.ID, Username: u.Username, IsAdmin: true}
	return f
}

func (f *fixture) user(username string) Actor {
	f.t.Helper()
	u, err := f.CreateUser(f.ctx, f.admin, username, "correct horse battery", "", false)
	if err != nil {
		f.t.Fatalf("create user %s: %v", username, err)
	}
	return Actor{UserID: u.ID, Username: u.Username}
}

func (f *fixture) team(name, code string, admin Actor) *store.Team {
	f.t.Helper()
	// CreateTeam makes its creator a team administrator, so a team is created
	// by the instance administrator and then handed over.
	team, err := f.CreateTeam(f.ctx, f.admin, name, code)
	if err != nil {
		f.t.Fatalf("create team %s: %v", name, err)
	}
	if admin.UserID != f.admin.UserID {
		if err := f.SetMember(f.ctx, f.admin, team.ID, admin.UserID, "admin"); err != nil {
			f.t.Fatal(err)
		}
	}
	return team
}

func (f *fixture) join(team *store.Team, a Actor, role string) {
	f.t.Helper()
	if err := f.SetMember(f.ctx, f.admin, team.ID, a.UserID, role); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) state(team *store.Team, role string) store.WorkState {
	f.t.Helper()
	st, err := store.WorkStateByRole(f.ctx, f.DB.R, team.ID, role)
	if err != nil {
		f.t.Fatalf("state %s of %s: %v", role, team.ShortCode, err)
	}
	return st
}

// code asserts the error is a core.Error with the given code.
func requireCode(t *testing.T, err error, want Code) {
	t.Helper()
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("error %v is not a core.Error", err)
	}
	if e.Code != want {
		t.Fatalf("error code %q, want %q (%s)", e.Code, want, e.Message)
	}
}

func totpNow(t *testing.T, secret string, f *fixture) string {
	t.Helper()
	code, err := auth.TOTPCode(secret, time.UnixMicro(f.now()))
	if err != nil {
		t.Fatal(err)
	}
	return code
}

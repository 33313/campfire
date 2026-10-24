package core

import (
	"context"
	"testing"

	"github.com/33313/campfire/internal/store"
)

func (f *fixture) ticket(team *store.Team, title string) *Item {
	f.t.Helper()
	it, err := f.CreateItem(f.ctx, f.admin, NewItem{Kind: store.KindTicket, TeamID: team.ID, Title: title})
	if err != nil {
		f.t.Fatal(err)
	}
	return it
}

func (f *fixture) pullRequests(id int64) []store.PullRequest {
	f.t.Helper()
	it, err := f.GetItem(f.ctx, f.admin, id)
	if err != nil {
		f.t.Fatal(err)
	}
	return it.PullRequests
}

func (f *fixture) apply(forge int64, pr store.PullRequest, ids ...string) {
	f.t.Helper()
	if err := f.ApplyPullRequest(f.ctx, forge, pr, ids); err != nil {
		f.t.Fatal(err)
	}
}

// trust takes any token, as acme's.
func trust(context.Context, store.Forge) (string, error) { return "acme", nil }

// forge connects one, and returns its id.
func (f *fixture) forge(kind, url string) int64 {
	f.t.Helper()
	fg, err := f.AddForge(f.ctx, f.admin, store.Forge{Kind: kind, Mode: "webhook", URL: url, Token: "t"}, trust)
	if err != nil {
		f.t.Fatal(err)
	}
	return fg.ID
}

func TestPullRequestsFollowTheirTitles(t *testing.T) {
	f := newFixture(t)
	team := f.team("Development", "DEV", f.admin)
	one, two := f.ticket(team, "one"), f.ticket(team, "two")
	gh := f.forge("github", "https://github.com")
	f.events = nil
	// changed counts the items told to refetch since it was last asked.
	changed := func() int {
		n := 0
		for _, e := range f.events {
			if e.Kind == store.EvPullRequestChanged {
				n++
			}
		}
		f.events = nil
		return n
	}

	pr := store.PullRequest{Remote: "https://github.com#100", Repo: "acme/web", Number: 4,
		Title: "DEV-1: one", URL: "https://github.com/acme/web/pull/4", State: "open", Author: "marta", UpdatedAt: 10}
	f.apply(gh, pr, one.Identifier, "NOPE-9")
	if got := f.pullRequests(one.ID); len(got) != 1 || got[0].Ref() != "acme/web#4" || got[0].State != "open" {
		t.Fatalf("one lists %+v", got)
	}
	if n := changed(); n != 1 {
		t.Fatalf("%d items told of a new pull request", n)
	}

	// Retitled to the other ticket: it moves.
	pr.Title, pr.UpdatedAt = "DEV-2: two", 20
	f.apply(gh, pr, two.Identifier)
	if len(f.pullRequests(one.ID)) != 0 || len(f.pullRequests(two.ID)) != 1 {
		t.Fatal("the pull request did not follow its title")
	}
	if n := changed(); n != 2 {
		t.Fatalf("%d items told of a move", n)
	}

	// Merged: the state changes in place.
	pr.State, pr.UpdatedAt = "merged", 30
	f.apply(gh, pr, two.Identifier)
	if got := f.pullRequests(two.ID); got[0].State != "merged" {
		t.Fatalf("two lists %+v", got)
	}

	// An older delivery arriving late changes nothing.
	stale := pr
	stale.State, stale.Title, stale.UpdatedAt = "open", "DEV-1: one", 15
	f.apply(gh, stale, one.Identifier)
	if got := f.pullRequests(two.ID); len(got) != 1 || got[0].State != "merged" || len(f.pullRequests(one.ID)) != 0 {
		t.Fatalf("a stale delivery was applied: %+v", got)
	}

	// The same id on another forge is another pull request.
	other := pr
	other.Remote = "https://codeberg.org#100"
	f.apply(f.forge("forgejo", "https://codeberg.org"), other, two.Identifier)
	if len(f.pullRequests(two.ID)) != 2 {
		t.Fatal("two forges' pull requests with one id were taken for one")
	}

	// Renamed to name nothing: unlinked, and no longer kept.
	pr.Title, pr.UpdatedAt = "no ticket", 40
	f.apply(gh, pr)
	if len(f.pullRequests(two.ID)) != 1 {
		t.Fatal("a pull request naming nothing is still listed")
	}
	var n int
	f.DB.R.QueryRow(`SELECT count(*) FROM pull_requests`).Scan(&n)
	if n != 1 {
		t.Fatalf("%d pull requests kept", n)
	}
	// A poll reading it again tells nobody: nothing changed.
	changed()
	f.apply(gh, other, two.Identifier)
	if n := changed(); n != 0 {
		t.Fatalf("an unchanged pull request told %d items", n)
	}
	// None of it is logged: the forge is the record.
	f.DB.R.QueryRow(`SELECT count(*) FROM events WHERE kind = ?`, store.EvPullRequestChanged).Scan(&n)
	if n != 0 {
		t.Fatalf("%d pull request changes logged", n)
	}
}

func TestForgesAreForInstanceAdministrators(t *testing.T) {
	f := newFixture(t)
	marta := f.user("marta")
	_, err := f.Forges(f.ctx, marta)
	requireCode(t, err, CodeForbidden)
	_, err = f.AddForge(f.ctx, marta, store.Forge{Kind: "github", Mode: "poll", URL: "https://github.com", Token: "t"}, trust)
	requireCode(t, err, CodeForbidden)
	requireCode(t, f.SetForgeToken(f.ctx, marta, 1, "t", trust), CodeForbidden)
}

func TestAddForge(t *testing.T) {
	f := newFixture(t)
	for _, bad := range []store.Forge{
		{Kind: "svn", Mode: "poll", URL: "https://x.test", Token: "t"},
		{Kind: "github", Mode: "poll", URL: "https://github.com", Token: ""},
		{Kind: "github", Mode: "poll", URL: "github.com", Token: "t"},
		{Kind: "github", Mode: "poll", URL: "ftp://github.com", Token: "t"},
		{Kind: "github", Mode: "poll", URL: "https://github.com", Token: "t", PollMinutes: -1},
		{Kind: "github", Mode: "poll", URL: "https://github.com", Token: "t", PollMinutes: 1441},
		{Kind: "github", Mode: "carrier pigeon", URL: "https://github.com", Token: "t"},
	} {
		// Refused before the token is tried.
		_, err := f.AddForge(f.ctx, f.admin, bad, func(context.Context, store.Forge) (string, error) {
			t.Errorf("%+v: tried the token", bad)
			return "", nil
		})
		requireCode(t, err, CodeInvalid)
	}
	a, err := f.AddForge(f.ctx, f.admin, store.Forge{Kind: "gitlab", Mode: "webhook", URL: " https://git.acme.test/gitlab/ ", Token: "t"}, trust)
	if err != nil || a.URL != "https://git.acme.test/gitlab" || len(a.WebhookSecret) != 48 || a.PollMinutes != 5 {
		t.Fatalf("forge %+v, %v", a, err)
	}
	// No address is the kind's hosted forge.
	b, _ := f.AddForge(f.ctx, f.admin, store.Forge{Kind: "gitlab", Mode: "webhook", Token: "t"}, trust)
	if b.URL != "https://gitlab.com" || a.WebhookSecret == b.WebhookSecret {
		t.Fatalf("hosted forge %+v", b)
	}
	// A polled forge takes no deliveries: no secret, and none to rotate.
	p, err := f.AddForge(f.ctx, f.admin, store.Forge{Kind: "github", Mode: "poll", URL: "https://github.com", Token: "t"}, trust)
	if err != nil || p.WebhookSecret != "" {
		t.Fatalf("polled forge %+v, %v", p, err)
	}
	_, err = f.RotateForgeSecret(f.ctx, f.admin, p.ID)
	requireCode(t, err, CodeNotFound)
	bot := func(_ context.Context, g store.Forge) (string, error) {
		if g.Token != "t2" {
			t.Errorf("tried %q", g.Token)
		}
		return "bot", nil
	}
	if err := f.SetForgeToken(f.ctx, f.admin, a.ID, "t2", bot); err != nil {
		t.Fatal(err)
	}
	got, _ := f.LoadForge(f.ctx, a.ID)
	if got.Token != "t2" || got.Account != "bot" || got.WebhookSecret != a.WebhookSecret {
		t.Fatalf("after a new token %+v", got)
	}
}

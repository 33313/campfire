package core

import (
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

// forge connects one, and returns its id.
func (f *fixture) forge(kind, url string) int64 {
	f.t.Helper()
	fg, err := f.AddForge(f.ctx, f.admin, store.Forge{Kind: kind, Mode: "webhook", URL: url, Account: "acme", Token: "t"})
	if err != nil {
		f.t.Fatal(err)
	}
	return fg.ID
}

func (f *fixture) kinds() map[string]int {
	out := map[string]int{}
	for _, e := range f.events {
		out[e.Kind]++
	}
	f.events = nil
	return out
}

func TestPullRequestsFollowTheirTitles(t *testing.T) {
	f := newFixture(t)
	team := f.team("Development", "DEV", f.admin)
	one, two := f.ticket(team, "one"), f.ticket(team, "two")
	gh := f.forge("github", "https://github.com")
	f.events = nil

	pr := store.PullRequest{Remote: "https://github.com#100", Repo: "acme/web", Number: 4,
		Title: "DEV-1: one", URL: "https://github.com/acme/web/pull/4", State: "open", Author: "marta", UpdatedAt: 10}
	f.apply(gh, pr, one.Identifier, "NOPE-9")
	if got := f.pullRequests(one.ID); len(got) != 1 || got[0].Ref() != "acme/web#4" || got[0].State != "open" {
		t.Fatalf("one lists %+v", got)
	}
	if k := f.kinds(); k[store.EvPullRequestLinked] != 1 {
		t.Fatalf("events %v", k)
	}

	// Retitled to the other ticket: it moves.
	pr.Title, pr.UpdatedAt = "DEV-2: two", 20
	f.apply(gh, pr, two.Identifier)
	if len(f.pullRequests(one.ID)) != 0 || len(f.pullRequests(two.ID)) != 1 {
		t.Fatal("the pull request did not follow its title")
	}
	if k := f.kinds(); k[store.EvPullRequestLinked] != 1 || k[store.EvPullRequestUnlinked] != 1 {
		t.Fatalf("events %v", k)
	}

	// Merged: the state changes in place, and the item's history says so.
	pr.State, pr.UpdatedAt = "merged", 30
	f.apply(gh, pr, two.Identifier)
	if got := f.pullRequests(two.ID); got[0].State != "merged" {
		t.Fatalf("two lists %+v", got)
	}
	if k := f.kinds(); k[store.EvPullRequestUpdated] != 1 || len(k) != 1 {
		t.Fatalf("events %v", k)
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
}

// Removing a forge keeps what it listed, the record of the work, and drops
// later deliveries for it.
func TestRemovingAForgeKeepsItsPullRequests(t *testing.T) {
	f := newFixture(t)
	team := f.team("Development", "DEV", f.admin)
	one := f.ticket(team, "one")
	f.apply(666, store.PullRequest{Remote: "x#1", Repo: "evil/x", Number: 1,
		Title: "DEV-1", URL: "u", State: "open", UpdatedAt: 1}, one.Identifier)
	if len(f.pullRequests(one.ID)) != 0 {
		t.Fatal("a delivery for no forge linked a pull request")
	}

	gh := f.forge("github", "https://github.com")
	pr := store.PullRequest{Remote: "https://github.com#2", Repo: "acme/web", Number: 1,
		Title: "DEV-1", URL: "u", State: "open", UpdatedAt: 1}
	f.apply(gh, pr, one.Identifier)
	if err := f.DeleteForge(f.ctx, f.admin, gh); err != nil {
		t.Fatal(err)
	}
	if fs, err := f.Forges(f.ctx, f.admin); err != nil || len(fs) != 0 {
		t.Fatalf("forges %v, %v", fs, err)
	}
	if len(f.pullRequests(one.ID)) != 1 {
		t.Fatal("removing the forge dropped the pull request")
	}
	pr.State, pr.UpdatedAt = "merged", 2
	f.apply(gh, pr, one.Identifier)
	if f.pullRequests(one.ID)[0].State != "open" {
		t.Fatal("a removed forge's delivery was applied")
	}
	requireCode(t, f.DeleteForge(f.ctx, f.admin, gh), CodeNotFound)
}

func TestForgesAreForInstanceAdministrators(t *testing.T) {
	f := newFixture(t)
	marta := f.user("marta")
	_, err := f.Forges(f.ctx, marta)
	requireCode(t, err, CodeForbidden)
	_, err = f.AddForge(f.ctx, marta, store.Forge{Kind: "github", Mode: "poll", URL: "https://github.com", Token: "t"})
	requireCode(t, err, CodeForbidden)
	requireCode(t, f.SetForgeToken(f.ctx, marta, 1, "a", "t"), CodeForbidden)
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
		_, err := f.AddForge(f.ctx, f.admin, bad)
		requireCode(t, err, CodeInvalid)
	}
	a, err := f.AddForge(f.ctx, f.admin, store.Forge{Kind: "gitlab", Mode: "webhook", URL: " https://git.acme.test/gitlab/ ", Token: "t"})
	if err != nil || a.URL != "https://git.acme.test/gitlab" || len(a.WebhookSecret) != 48 || a.PollMinutes != 5 {
		t.Fatalf("forge %+v, %v", a, err)
	}
	b, _ := f.AddForge(f.ctx, f.admin, store.Forge{Kind: "gitlab", Mode: "webhook", URL: "https://gitlab.com", Token: "t"})
	if a.WebhookSecret == b.WebhookSecret {
		t.Fatal("two forges share a webhook secret")
	}
	// A polled forge takes no deliveries: no secret, and none to rotate.
	p, err := f.AddForge(f.ctx, f.admin, store.Forge{Kind: "github", Mode: "poll", URL: "https://github.com", Token: "t"})
	if err != nil || p.WebhookSecret != "" {
		t.Fatalf("polled forge %+v, %v", p, err)
	}
	_, err = f.RotateForgeSecret(f.ctx, f.admin, p.ID)
	requireCode(t, err, CodeNotFound)
	if err := f.SetForgeToken(f.ctx, f.admin, a.ID, "bot", "t2"); err != nil {
		t.Fatal(err)
	}
	got, _ := f.LoadForge(f.ctx, a.ID)
	if got.Token != "t2" || got.Account != "bot" || got.WebhookSecret != a.WebhookSecret {
		t.Fatalf("after a new token %+v", got)
	}
}

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

func (f *fixture) apply(pr store.PullRequest, ids ...string) {
	f.t.Helper()
	if err := f.ApplyPullRequest(f.ctx, pr, ids); err != nil {
		f.t.Fatal(err)
	}
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
	if err := f.LinkInstallation(f.ctx, f.admin, 7, "acme", "Organization"); err != nil {
		t.Fatal(err)
	}
	f.events = nil

	pr := store.PullRequest{ID: 100, InstallationID: 7, Repo: "acme/web", Number: 4,
		Title: "DEV-1: one", URL: "https://github.com/acme/web/pull/4", State: "open", Author: "marta", UpdatedAt: 10}
	f.apply(pr, one.Identifier, "NOPE-9")
	if got := f.pullRequests(one.ID); len(got) != 1 || got[0].Ref() != "acme/web#4" || got[0].State != "open" {
		t.Fatalf("one lists %+v", got)
	}
	if k := f.kinds(); k[store.EvPullRequestLinked] != 1 {
		t.Fatalf("events %v", k)
	}

	// Retitled to the other ticket: it moves.
	pr.Title, pr.UpdatedAt = "DEV-2: two", 20
	f.apply(pr, two.Identifier)
	if len(f.pullRequests(one.ID)) != 0 || len(f.pullRequests(two.ID)) != 1 {
		t.Fatal("the pull request did not follow its title")
	}
	if k := f.kinds(); k[store.EvPullRequestLinked] != 1 || k[store.EvPullRequestUnlinked] != 1 {
		t.Fatalf("events %v", k)
	}

	// Merged: the state changes in place, and the item's history says so.
	pr.State, pr.UpdatedAt = "merged", 30
	f.apply(pr, two.Identifier)
	if got := f.pullRequests(two.ID); got[0].State != "merged" {
		t.Fatalf("two lists %+v", got)
	}
	if k := f.kinds(); k[store.EvPullRequestUpdated] != 1 || len(k) != 1 {
		t.Fatalf("events %v", k)
	}

	// An older delivery arriving late changes nothing.
	stale := pr
	stale.State, stale.Title, stale.UpdatedAt = "open", "DEV-1: one", 15
	f.apply(stale, one.Identifier)
	if got := f.pullRequests(two.ID); len(got) != 1 || got[0].State != "merged" || len(f.pullRequests(one.ID)) != 0 {
		t.Fatalf("a stale delivery was applied: %+v", got)
	}

	// Renamed to name nothing: unlinked, and no longer kept.
	pr.Title, pr.UpdatedAt = "no ticket", 40
	f.apply(pr)
	if len(f.pullRequests(two.ID)) != 0 {
		t.Fatal("a pull request naming nothing is still listed")
	}
	var n int
	f.DB.R.QueryRow(`SELECT count(*) FROM pull_requests`).Scan(&n)
	if n != 0 {
		t.Fatalf("%d pull requests kept", n)
	}
}

// The app is public, so anyone can install it; only a linked installation
// is listened to.
func TestAnUnlinkedInstallationIsIgnored(t *testing.T) {
	f := newFixture(t)
	team := f.team("Development", "DEV", f.admin)
	one := f.ticket(team, "one")
	f.apply(store.PullRequest{ID: 1, InstallationID: 666, Repo: "evil/x", Number: 1,
		Title: "DEV-1", URL: "u", State: "open", UpdatedAt: 1}, one.Identifier)
	if len(f.pullRequests(one.ID)) != 0 {
		t.Fatal("a stranger's installation linked a pull request")
	}

	// Unlinking keeps what was linked: it is the record of the work.
	if err := f.LinkInstallation(f.ctx, f.admin, 7, "acme", "Organization"); err != nil {
		t.Fatal(err)
	}
	f.apply(store.PullRequest{ID: 2, InstallationID: 7, Repo: "acme/web", Number: 1,
		Title: "DEV-1", URL: "u", State: "open", UpdatedAt: 1}, one.Identifier)
	if err := f.InstallationRemoved(f.ctx, 7); err != nil {
		t.Fatal(err)
	}
	if len(f.pullRequests(one.ID)) != 1 {
		t.Fatal("unlinking dropped the pull request")
	}
	// Later deliveries from it are ignored.
	f.apply(store.PullRequest{ID: 2, InstallationID: 7, Repo: "acme/web", Number: 1,
		Title: "DEV-1", URL: "u", State: "merged", UpdatedAt: 2}, one.Identifier)
	if f.pullRequests(one.ID)[0].State != "open" {
		t.Fatal("an unlinked installation's delivery was applied")
	}
	if err := f.InstallationRemoved(f.ctx, 7); err != nil {
		t.Fatalf("removing it twice: %v", err)
	}
}

func TestGitHubIsForInstanceAdministrators(t *testing.T) {
	f := newFixture(t)
	marta := f.user("marta")
	_, err := f.GitHub(f.ctx, marta)
	requireCode(t, err, CodeForbidden)
	requireCode(t, f.LinkInstallation(f.ctx, marta, 7, "acme", "Organization"), CodeForbidden)
	_, err = f.GitHubState(f.ctx, marta)
	requireCode(t, err, CodeForbidden)
}

// A state is spent once, by whoever it was issued to, and never redeems as
// an invitation.
func TestGitHubStateIsSpentOnce(t *testing.T) {
	f := newFixture(t)
	state, err := f.GitHubState(f.ctx, f.admin)
	if err != nil {
		t.Fatal(err)
	}
	other, err := f.CreateUser(f.ctx, f.admin, "other", "correct horse battery", "", true)
	if err != nil {
		t.Fatal(err)
	}
	requireCode(t, f.SpendGitHubState(f.ctx, Actor{UserID: other.ID, IsAdmin: true}, state), CodeForbidden)
	_, err = f.RedeemInvitation(f.ctx, state, "intruder", "correct horse battery")
	requireCode(t, err, CodeNotFound)
	if err := f.SpendGitHubState(f.ctx, f.admin, state); err != nil {
		t.Fatal(err)
	}
	requireCode(t, f.SpendGitHubState(f.ctx, f.admin, state), CodeForbidden)
}

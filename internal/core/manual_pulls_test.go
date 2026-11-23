package core

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/33313/campfire/internal/store"
)

func TestManualPullCanBeRefreshedAndRemoved(t *testing.T) {
	w := newWorld(t)
	it := w.ticket(w.sol, w.marta, "Manual links")
	const target = "https://git.example/a/b/pull/4"
	now, state := store.Now(), "unknown"
	w.Clock = func() int64 { return now }
	lookup := func(_ context.Context, u *url.URL) store.PullRequest {
		return store.PullRequest{URL: u.String(), Repo: "a/b", Number: 4, State: state}
	}
	requireCode(t, w.AddManualPull(w.ctx, w.admin, it.ID, "javascript:alert(1)", lookup), CodeInvalid)
	if err := w.AddManualPull(w.ctx, w.marta, it.ID, target, lookup); err != nil {
		t.Fatal(err)
	}
	read := func(want string) int64 {
		t.Helper()
		got, err := w.GetItem(w.ctx, w.marta, it.ID)
		if err != nil || len(got.PullRequests) != 1 {
			t.Fatalf("manual link not shown: %v, %v", got, err)
		}
		if p := got.PullRequests[0]; !p.Manual || p.State != want || p.URL != target {
			t.Fatalf("manual link %+v; want state %s", p, want)
		}
		return got.PullRequests[0].ID
	}
	read("unknown")
	state = "merged"
	now += int64(time.Hour / time.Microsecond)
	if err := w.CheckManualPulls(w.ctx, lookup); err != nil {
		t.Fatal(err)
	}
	id := read("merged")
	if err := w.RemoveManualPull(w.ctx, w.marta, it.ID, id); err != nil {
		t.Fatal(err)
	}
	got, err := w.GetItem(w.ctx, w.marta, it.ID)
	if err != nil || len(got.PullRequests) != 0 {
		t.Fatalf("removed link still shown: %v, %v", got, err)
	}
}

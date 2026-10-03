package core

import (
	"slices"
	"testing"
	"time"

	"github.com/campfire/campfire/internal/store"
)

// SPEC.md §9.2: a CSQL query composes conditions with and, or and not, and
// means the same wherever it is run.
func TestQueriesComposeWithAndOrNot(t *testing.T) {
	w := newWorld(t)
	mk := func(title string, in NewItem) {
		in.Kind, in.TeamID, in.Title = store.KindTicket, w.sol.ID, title
		if _, err := w.CreateItem(w.ctx, w.marta, in); err != nil {
			t.Fatal(err)
		}
	}
	roses := w.bucket(w.sol, w.marta, "Roses")
	mk("pruning", NewItem{AssigneeID: w.marta.UserID, Priority: 1, Labels: []string{"garden"}, BucketID: roses.ID})
	mk("watering", NewItem{AssigneeID: w.marta.UserID, Priority: 3})
	mk("compost", NewItem{Priority: 2, Labels: []string{"garden", "soil"}, Description: "Turn the heap"})
	mk("fence", NewItem{Priority: 4})

	titles := func(q string) []string {
		t.Helper()
		page, err := w.ListItems(w.ctx, w.marta, Filter{Query: "kind = ticket and (" + q + ")"}, "", 50)
		if err != nil {
			t.Fatalf("%q: %v", q, err)
		}
		var out []string
		for _, it := range page.Items {
			out = append(out, it.Title)
		}
		slices.Sort(out)
		return out
	}
	for q, want := range map[string][]string{
		"assignee = marta":                                        {"pruning", "watering"},
		"assignee = me and label = garden":                        {"pruning"},
		"assignee = marta or label = soil":                        {"compost", "pruning", "watering"},
		"not label = garden":                                      {"fence", "watering"},
		"label = garden and not (bucket = Roses or priority = 1)": {"compost"},
		"assignee != marta":                                       {"compost", "fence"}, // the unassigned are not marta's
		"assignee = none and label = none":                        {"fence"},
		"priority <= P2":                                          {"compost", "pruning"},
		"label in (soil) or bucket != none":                       {"compost", "pruning"},
		`text ~ heap or text ~ "PRUN"`:                            {"compost", "pruning"},
		"label not in (garden, soil) and priority > 3":            {"fence"},
		"kind != bucket and priority = 4":                         {"fence"},
	} {
		if got := titles(q); !slices.Equal(got, want) {
			t.Errorf("%q matched %v, want %v", q, got, want)
		}
	}
}

// SPEC.md §9.2: a date field compares whole days in the reader's time zone.
func TestDaysAreReadInTheReadersZone(t *testing.T) {
	w := newWorld(t)
	at := func(s string) {
		tm, _ := time.Parse(time.RFC3339, s)
		w.Clock = func() int64 { return tm.UnixMicro() }
	}
	mk := func(title, due string) {
		if _, err := w.CreateItem(w.ctx, w.marta, NewItem{Kind: store.KindTicket, TeamID: w.sol.ID,
			Title: title, PlannedEnd: due}); err != nil {
			t.Fatal(err)
		}
	}
	at("2026-09-20T10:00:00Z")
	mk("old", "")
	at("2026-09-30T23:30:00Z") // already 1 October in Warsaw
	mk("late", "2026-09-29")
	at("2026-10-01T12:00:00Z")
	mk("fresh", "2026-10-03")

	titles := func(q, tz string) []string {
		t.Helper()
		page, err := w.ListItems(w.ctx, w.marta, Filter{Query: "kind = ticket and (" + q + ")", TZ: tz}, "", 50)
		if err != nil {
			t.Fatalf("%q in %q: %v", q, tz, err)
		}
		var out []string
		for _, it := range page.Items {
			out = append(out, it.Title)
		}
		slices.Sort(out)
		return out
	}
	for _, tc := range []struct {
		q, tz string
		want  []string
	}{
		{"created = today", "", []string{"fresh"}},
		{"created = today", "Europe/Warsaw", []string{"fresh", "late"}},
		{"created >= -7d", "", []string{"fresh", "late"}},
		{"created < 2026-09-21 or due = none", "", []string{"old"}},
		{"due < today", "", []string{"late"}},
		{"due >= today and due <= +2d", "", []string{"fresh"}},
		{"not due > 2026-09-29", "", []string{"late", "old"}}, // not, over an empty due date, includes it
		{"completed = none and started = none", "", []string{"fresh", "late", "old"}},
	} {
		if got := titles(tc.q, tc.tz); !slices.Equal(got, tc.want) {
			t.Errorf("%q in %q matched %v, want %v", tc.q, tc.tz, got, tc.want)
		}
	}
	_, err := w.ListItems(w.ctx, w.marta, Filter{Query: "created = today", TZ: "Mars/Olympus"}, "", 50)
	requireCode(t, err, CodeInvalid)
}

// SPEC.md §9.2: the words a query gives a meaning of its own are no one's
// name, so assignee = none can never mean a user called none.
func TestReservedWordsAreNoOnesName(t *testing.T) {
	w := newWorld(t)
	it := w.ticket(w.sol, w.marta, "named")
	states, _ := w.ListWorkStates(w.ctx, w.marta, w.sol.ID)
	for _, word := range []string{"me", "None", "TODAY", "and", " or ", "not", "in"} {
		_, err := w.CreateUser(w.ctx, w.admin, word, "a long enough password", "", false)
		requireCode(t, err, CodeInvalid)
		_, err = w.AddLabel(w.ctx, w.marta, it.ID, word)
		requireCode(t, err, CodeInvalid)
		_, err = w.CreateItem(w.ctx, w.marta, NewItem{Kind: store.KindTicket, TeamID: w.sol.ID,
			Title: "labelled", Labels: []string{word}})
		requireCode(t, err, CodeInvalid)
		_, err = w.CreateTeam(w.ctx, w.admin, word, "ZQX")
		requireCode(t, err, CodeInvalid)
		requireCode(t, w.RenameTeam(w.ctx, w.admin, w.sol.ID, word), CodeInvalid)
		_, err = w.CreateWorkState(w.ctx, w.admin, w.sol.ID, word, "")
		requireCode(t, err, CodeInvalid)
		requireCode(t, w.UpdateWorkState(w.ctx, w.admin, states[0].ID, word, "", 0), CodeInvalid)
	}
	_, err := w.CreateTeam(w.ctx, w.admin, "Mirror", "ME")
	requireCode(t, err, CodeInvalid)
	// A word that only contains one is anyone's.
	if _, err := w.AddLabel(w.ctx, w.marta, it.ID, "none-yet"); err != nil {
		t.Fatal(err)
	}
}

package core

import (
	"errors"
	"strings"
	"testing"

	"github.com/campfire/campfire/internal/store"
)

// SPEC.md §9.4: a saved view stores its query canonical, naming what it
// references by #id, and hands it out written with today's names.
func TestSavedViewStoresItsQueryByIdentity(t *testing.T) {
	w := newWorld(t)
	w.CreateItem(w.ctx, w.marta, NewItem{Kind: store.KindTicket, TeamID: w.sol.ID,
		Title: "labelled", Labels: []string{"api"}})

	v, err := w.CreateSavedView(w.ctx, w.marta, ViewInput{
		TeamID: w.sol.ID, Name: "API work", PinBoard: true,
		Query: "LABEL = API and (assignee = marta or reporter = me)"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "label = api and (assignee = marta or reporter = me)"; v.Query != want {
		t.Fatalf("query = %q, want %q", v.Query, want)
	}
	stored, _ := store.SavedViewByID(w.ctx, w.DB.R, v.ID)
	if strings.Contains(stored.Query, "api") || strings.Contains(stored.Query, "marta") ||
		!strings.Contains(stored.Query, "reporter = me") {
		t.Fatalf("stored %q names by name, or pinned me to its author", stored.Query)
	}
}

// A referent MUST NOT go when a view names it: a label is not reaped, and a
// deleted bucket is named as missing and matches nothing (SPEC.md §9.4).
func TestSavedViewKeepsAReferentThatIsGone(t *testing.T) {
	w := newWorld(t)
	it, _ := w.CreateItem(w.ctx, w.marta, NewItem{Kind: store.KindTicket, TeamID: w.sol.ID,
		Title: "labelled", Labels: []string{"api"}})
	labels, _ := w.ListLabels(w.ctx, w.marta)
	labelID := labels[0].ID

	v, err := w.CreateSavedView(w.ctx, w.marta, ViewInput{TeamID: w.sol.ID, Name: "API work",
		Query: "label = api"})
	if err != nil {
		t.Fatal(err)
	}
	// Taking the last use of the label off the item would normally reap it.
	if _, err := w.RemoveLabel(w.ctx, w.marta, it.ID, "api"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LabelByID(w.ctx, w.DB.R, labelID); err != nil {
		t.Fatal("the label was reaped despite being named by a saved view")
	}

	bucket := w.bucket(w.sol, w.marta, "Release 1.0")
	v2, err := w.UpdateSavedView(w.ctx, w.marta, v.ID, ViewInput{
		TeamID: w.sol.ID, Name: "API work", Query: `bucket = "release 1.0" or not label = api`})
	if err != nil {
		t.Fatal(err)
	}
	if len(v2.Missing) != 0 || !strings.HasPrefix(v2.Query, "bucket = "+bucket.Identifier) {
		t.Fatalf("before anything was deleted: %+v", v2)
	}
	if err := w.DeleteItem(w.ctx, w.admin, bucket.ID); err != nil {
		t.Fatal(err)
	}
	views, _ := w.ListSavedViews(w.ctx, w.marta, w.sol.ID)
	if len(views) != 1 || len(views[0].Missing) != 1 || !strings.Contains(views[0].Query, "#") {
		t.Fatalf("the deleted bucket is not named as missing: %+v", views)
	}

	// The bucket half now matches nothing; the not-api half still matches.
	f, _ := w.SavedViewFilter(w.ctx, w.marta, v.ID)
	page, err := w.ListItems(w.ctx, w.marta, f, "", 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range page.Items {
		if it.Kind == store.KindBucket && it.ID == bucket.ID {
			t.Fatal("a deleted bucket was matched")
		}
	}
}

// Any member or administrator of the owning team may create, edit and delete.
// Every authenticated user may read and apply any view (SPEC.md §9.4).
func TestSavedViewPermissions(t *testing.T) {
	w := newWorld(t)
	v, err := w.CreateSavedView(w.ctx, w.marta, ViewInput{TeamID: w.sol.ID, Name: "Triage"})
	if err != nil {
		t.Fatalf("a member could not create a view in their own team: %v", err)
	}
	// Sam is in TERRA, not SOL.
	_, err = w.CreateSavedView(w.ctx, w.sam, ViewInput{TeamID: w.sol.ID, Name: "Theirs"})
	requireCode(t, err, CodeForbidden)
	_, err = w.UpdateSavedView(w.ctx, w.sam, v.ID, ViewInput{TeamID: w.sol.ID, Name: "Renamed"})
	requireCode(t, err, CodeForbidden)
	requireCode(t, w.DeleteSavedView(w.ctx, w.sam, v.ID), CodeForbidden)

	// But reading and applying are open to everyone.
	if _, err := w.ListSavedViews(w.ctx, w.sam, w.sol.ID); err != nil {
		t.Fatalf("reading another team's views was refused: %v", err)
	}
	if _, err := w.SavedViewFilter(w.ctx, w.sam, v.ID); err != nil {
		t.Fatalf("applying another team's view was refused: %v", err)
	}
}

func TestSavedViewNamesAreUniqueWithinATeamAndCaseFolded(t *testing.T) {
	w := newWorld(t)
	if _, err := w.CreateSavedView(w.ctx, w.marta, ViewInput{TeamID: w.sol.ID, Name: "Triage"}); err != nil {
		t.Fatal(err)
	}
	_, err := w.CreateSavedView(w.ctx, w.marta, ViewInput{TeamID: w.sol.ID, Name: "triage"})
	requireCode(t, err, CodeConflict)
	// Another team may use the same name.
	if _, err := w.CreateSavedView(w.ctx, w.sam, ViewInput{TeamID: w.ter.ID, Name: "Triage"}); err != nil {
		t.Fatalf("the same name in another team was refused: %v", err)
	}
}

// A view owned by SOL may filter on TERRA's tickets: the owning team governs
// where it is listed, not what it may match (SPEC.md §9.4).
func TestAViewMayFilterOnAnotherTeam(t *testing.T) {
	w := newWorld(t)
	theirs := w.ticket(w.ter, w.sam, "theirs")
	v, err := w.CreateSavedView(w.ctx, w.marta, ViewInput{TeamID: w.sol.ID, Name: "Watching TERRA",
		Query: "team = TER"})
	if err != nil {
		t.Fatal(err)
	}
	f, _ := w.SavedViewFilter(w.ctx, w.marta, v.ID)
	page, err := w.ListItems(w.ctx, w.marta, f, "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].Identifier != theirs.Identifier {
		t.Fatalf("the view matched %+v", page.Items)
	}
}

// Every mutation of a saved view writes an event (SPEC.md §9.4).
func TestSavedViewMutationsAreLogged(t *testing.T) {
	w := newWorld(t)
	v, _ := w.CreateSavedView(w.ctx, w.marta, ViewInput{TeamID: w.sol.ID, Name: "Triage"})
	if _, err := w.UpdateSavedView(w.ctx, w.marta, v.ID, ViewInput{TeamID: w.sol.ID,
		Name: "Triage renamed", PinBoard: true, Query: "priority = 1"}); err != nil {
		t.Fatal(err)
	}
	if err := w.DeleteSavedView(w.ctx, w.marta, v.ID); err != nil {
		t.Fatal(err)
	}
	page, _ := w.Log(w.ctx, w.marta, store.EventFilter{TeamID: w.sol.ID}, 0, 100)
	kinds := map[string]bool{}
	for _, e := range page.Events {
		kinds[e.Kind+"/"+e.Field] = true
	}
	for _, want := range []string{
		store.EvViewCreated + "/", store.EvViewUpdated + "/name",
		store.EvViewUpdated + "/pinning", store.EvViewUpdated + "/query",
		store.EvViewDeleted + "/"} {
		if !kinds[want] {
			t.Fatalf("no event for %q; got %v", want, kinds)
		}
	}
}

// A query that does not read, or names nothing, is refused where it is wrong.
func TestSavedViewRejectsABadQueryAtThePlace(t *testing.T) {
	w := newWorld(t)
	_, err := w.CreateSavedView(w.ctx, w.marta, ViewInput{TeamID: w.sol.ID, Name: "Bad",
		Query: "assignee = marty"})
	requireCode(t, err, CodeInvalid)
	var e *Error
	if !errors.As(err, &e) || e.Field != "query" || len(e.Span) != 2 || e.Span[0] != 11 || e.Span[1] != 16 {
		t.Fatalf("got %+v", err)
	}
}

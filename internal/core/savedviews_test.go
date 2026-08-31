package core

import (
	"testing"

	"github.com/campfire/campfire/internal/store"
)

// SPEC.md §9.4: a saved view is stored as its filter's own structured
// representation — one row per clause, referencing by identity — never as a
// serialised query string.
func TestSavedViewStoresClausesByIdentity(t *testing.T) {
	w := newWorld(t)
	it, _ := w.CreateItem(w.ctx, w.marta, NewItem{Kind: store.KindTicket, TeamID: w.sol.ID,
		Title: "labelled", Labels: []string{"api"}})
	labels, _ := w.ListLabels(w.ctx, w.marta)

	v, err := w.CreateSavedView(w.ctx, w.marta, ViewInput{
		TeamID: w.sol.ID, Name: "API work", PinBoard: true,
		Clauses: []store.Clause{
			{Dimension: store.DimLabel, RefID: labels[0].ID},
			{Dimension: store.DimAssignee, RefID: w.marta.UserID},
		}})
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Clauses) != 2 {
		t.Fatalf("%d clauses", len(v.Clauses))
	}
	for _, c := range v.Clauses {
		if c.RefID == 0 {
			t.Fatalf("clause %+v does not reference anything by identity", c)
		}
		if c.Missing {
			t.Fatalf("clause %+v is missing its referent already", c)
		}
	}
	_ = it
}

// A clause MUST NOT be removed by cascade when its referent is deleted, and a
// label referenced by a clause MUST NOT be reaped by lazy cleanup. The view
// names the missing reference and matches nothing (SPEC.md §9.4).
func TestSavedViewKeepsAClauseWhoseReferentIsGone(t *testing.T) {
	w := newWorld(t)
	it, _ := w.CreateItem(w.ctx, w.marta, NewItem{Kind: store.KindTicket, TeamID: w.sol.ID,
		Title: "labelled", Labels: []string{"api"}})
	labels, _ := w.ListLabels(w.ctx, w.marta)
	labelID := labels[0].ID

	v, err := w.CreateSavedView(w.ctx, w.marta, ViewInput{TeamID: w.sol.ID, Name: "API work",
		Clauses: []store.Clause{{Dimension: store.DimLabel, RefID: labelID}}})
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

	// Now delete a different referent — a bucket — and check the clause stays
	// and reports itself missing.
	bucket := w.bucket(w.sol, w.marta, "Release 1.0")
	v2, err := w.UpdateSavedView(w.ctx, w.marta, v.ID, ViewInput{
		TeamID: w.sol.ID, Name: "API work",
		Clauses: []store.Clause{{Dimension: store.DimBucket, RefID: bucket.ID}}})
	if err != nil {
		t.Fatal(err)
	}
	if v2.Clauses[0].Missing {
		t.Fatal("the clause is missing before anything was deleted")
	}
	if err := w.DeleteItem(w.ctx, w.admin, bucket.ID); err != nil {
		t.Fatal(err)
	}
	views, err := w.ListSavedViews(w.ctx, w.marta, w.sol.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 1 || len(views[0].Clauses) != 1 {
		t.Fatalf("the clause was dropped: %+v", views)
	}
	if !views[0].Clauses[0].Missing {
		t.Fatal("the clause does not report its referent as missing")
	}

	// And the view now matches nothing rather than silently widening.
	f, err := w.SavedViewFilter(w.ctx, w.marta, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	page, err := w.ListItems(w.ctx, w.marta, f, "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 0 {
		t.Fatalf("a view naming a deleted bucket matched %d items", len(page.Items))
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
		Clauses: []store.Clause{{Dimension: store.DimTeam, RefID: w.ter.ID}}})
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
		Name: "Triage renamed", PinBoard: true}); err != nil {
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
		store.EvViewUpdated + "/pinning", store.EvViewUpdated + "/clauses",
		store.EvViewDeleted + "/"} {
		if !kinds[want] {
			t.Fatalf("no event for %q; got %v", want, kinds)
		}
	}
}

func TestSavedViewRejectsUnknownDimensions(t *testing.T) {
	w := newWorld(t)
	_, err := w.CreateSavedView(w.ctx, w.marta, ViewInput{TeamID: w.sol.ID, Name: "Bad",
		Clauses: []store.Clause{{Dimension: "type", RefID: 1}}})
	requireCode(t, err, CodeInvalid)
}

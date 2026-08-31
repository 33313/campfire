package core

import (
	"testing"

	"github.com/campfire/campfire/internal/store"
)

// SPEC.md §6.5: a clone copies title, description, priority, assignee, bucket
// and labels; enters the default state; takes the cloning user as reporter;
// and carries a "relates to" relation to the ticket it came from.
func TestCloneCopiesTheRightFieldsAndNothingElse(t *testing.T) {
	w := newWorld(t)
	bucket := w.bucket(w.sol, w.marta, "Release 1.0")
	src, err := w.CreateItem(w.ctx, w.marta, NewItem{
		Kind: store.KindTicket, TeamID: w.sol.ID, Title: "source", Description: "why",
		Priority: 1, AssigneeID: w.marta.UserID, BucketID: bucket.ID,
		Labels: []string{"api"}, Accepted: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.AddComment(w.ctx, w.marta, src.ID, "a comment on the source"); err != nil {
		t.Fatal(err)
	}
	other := w.ticket(w.sol, w.marta, "other")
	if _, err := w.AddRelation(w.ctx, w.marta, src.ID, other.ID, store.RelBlocks); err != nil {
		t.Fatal(err)
	}

	w.join(w.sol, w.sam, "member")
	clone, err := w.Clone(w.ctx, w.sam, src.ID)
	if err != nil {
		t.Fatal(err)
	}

	if clone.Title != src.Title || clone.Description != src.Description ||
		clone.Priority != src.Priority || clone.AssigneeID != src.AssigneeID ||
		clone.BucketID != src.BucketID {
		t.Fatalf("clone did not copy the fields: %+v", clone.Item)
	}
	if len(clone.Labels) != 1 || clone.Labels[0] != "api" {
		t.Fatalf("clone labels %v", clone.Labels)
	}
	if clone.Identifier == src.Identifier {
		t.Fatal("the clone reused the source's identifier")
	}
	if clone.StateRole != store.RoleDefault {
		t.Fatalf("the clone entered the %q state", clone.StateRole)
	}
	if clone.ReporterID != w.sam.UserID {
		t.Fatal("the clone's reporter is not the cloning user")
	}
	if clone.ActualStart != 0 || clone.ActualEnd != 0 {
		t.Fatal("the clone copied the source's clock")
	}
	comments, _ := w.ListComments(w.ctx, w.sam, clone.ID, 0, 50)
	if len(comments) != 0 {
		t.Fatal("the clone copied the source's comments")
	}
	// The edge back to the source is the only relation it is created with.
	if len(clone.Relations) != 1 {
		t.Fatalf("clone relations %+v", clone.Relations)
	}
	if clone.Relations[0].Kind != store.RelRelatesTo || clone.Relations[0].ItemID != src.ID {
		t.Fatalf("clone relation is %+v", clone.Relations[0])
	}
}

// SPEC.md §6.5: a split gives a title per new ticket, each carrying a "follows
// up" relation to the source, and never deletes or absorbs the source.
func TestSplitLeavesTheSourceReachable(t *testing.T) {
	w := newWorld(t)
	src := w.ticket(w.sol, w.marta, "too big")
	res, err := w.Split(w.ctx, w.marta, src.ID, []string{"part one", "part two"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.New) != 2 {
		t.Fatalf("%d new tickets", len(res.New))
	}
	for _, n := range res.New {
		if len(n.Relations) != 1 || n.Relations[0].Kind != store.RelFollowsUp ||
			n.Relations[0].ItemID != src.ID {
			t.Fatalf("new ticket %s relations %+v", n.Identifier, n.Relations)
		}
	}
	if res.Source.StateRole != store.RoleDefault {
		t.Fatal("the source's state changed without being asked")
	}
	// Each new ticket allocated its own identifier, one at a time.
	if res.New[0].Identifier == res.New[1].Identifier {
		t.Fatal("two new tickets share an identifier")
	}
	// The tickets that replaced it are reachable from it.
	source, _ := w.GetItem(w.ctx, w.marta, src.ID)
	if len(source.Relations) != 2 {
		t.Fatalf("the source shows %d relations", len(source.Relations))
	}
}

func TestSplitCanRejectItsSourceAsCancelled(t *testing.T) {
	w := newWorld(t)
	src := w.ticket(w.sol, w.marta, "too big")
	res, err := w.Split(w.ctx, w.marta, src.ID, []string{"part one"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Source.StateRole != store.RoleRejected {
		t.Fatalf("the source is in the %q state", res.Source.StateRole)
	}
	if res.Source.RejectReason != store.ReasonCancelled {
		t.Fatalf("the source was rejected as %q, want cancelled", res.Source.RejectReason)
	}
}

func TestOnlyTicketsAreClonedOrSplit(t *testing.T) {
	w := newWorld(t)
	b := w.bucket(w.sol, w.marta, "Release 1.0")
	_, err := w.Clone(w.ctx, w.marta, b.ID)
	requireCode(t, err, CodeInvalid)
	_, err = w.Split(w.ctx, w.marta, b.ID, []string{"x"}, false)
	requireCode(t, err, CodeInvalid)
}

// SPEC.md §6.5: an item in a bulk edit that the actor may not edit MUST be
// refused on its own and MUST NOT abort the rest.
func TestBulkEditRefusesPerItemAndContinues(t *testing.T) {
	w := newWorld(t)
	mine := w.ticket(w.sol, w.marta, "mine")
	theirs := w.ticket(w.ter, w.sam, "theirs")
	alsoMine := w.ticket(w.sol, w.marta, "also mine")

	res, err := w.ApplyBulkEdit(w.ctx, w.marta, BulkEdit{
		ItemIDs: []int64{mine.ID, theirs.ID, alsoMine.ID},
		Field:   BulkPriority, Value: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Changed) != 2 || len(res.Refused) != 1 {
		t.Fatalf("%d changed, %d refused", len(res.Changed), len(res.Refused))
	}
	if res.Refused[0].ItemID != theirs.ID {
		t.Fatalf("the wrong item was refused: %+v", res.Refused)
	}
	for _, it := range res.Changed {
		if it.Priority != 1 {
			t.Fatalf("%s is P%d", it.Identifier, it.Priority)
		}
	}
}

// Every item touched by a bulk edit writes its own event. A bulk operation
// writes no summary event (SPEC.md §6.5).
func TestBulkEditWritesOneEventPerItem(t *testing.T) {
	w := newWorld(t)
	a := w.ticket(w.sol, w.marta, "one")
	b := w.ticket(w.sol, w.marta, "two")
	before, _ := w.Log(w.ctx, w.marta, store.EventFilter{TeamID: w.sol.ID}, 0, 200)

	if _, err := w.ApplyBulkEdit(w.ctx, w.marta, BulkEdit{
		ItemIDs: []int64{a.ID, b.ID}, Field: BulkPriority, Value: 2}); err != nil {
		t.Fatal(err)
	}
	after, _ := w.Log(w.ctx, w.marta, store.EventFilter{TeamID: w.sol.ID}, 0, 200)
	if len(after.Events)-len(before.Events) != 2 {
		t.Fatalf("a two-item bulk edit wrote %d events", len(after.Events)-len(before.Events))
	}
	for _, id := range []int64{a.ID, b.ID} {
		page, _ := w.Log(w.ctx, w.marta, store.EventFilter{ItemID: id}, 0, 50)
		var found bool
		for _, e := range page.Events {
			if e.Field == "priority" {
				found = true
			}
		}
		if !found {
			t.Fatalf("item %d has no event for the bulk edit", id)
		}
	}
}

// A bulk move goes through the same transition as a single move, rejection
// reason included (SPEC.md §6.5).
func TestBulkMoveAppliesTheOrdinaryTransition(t *testing.T) {
	w := newWorld(t)
	a := w.ticket(w.sol, w.marta, "one")
	b := w.ticket(w.sol, w.marta, "two")
	rejected := w.state(w.sol, store.RoleRejected)

	res, err := w.ApplyBulkEdit(w.ctx, w.marta, BulkEdit{
		ItemIDs: []int64{a.ID, b.ID}, Field: BulkState, Value: rejected.ID,
		Reason: store.ReasonCancelled})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Changed) != 2 {
		t.Fatalf("%d changed", len(res.Changed))
	}
	for _, it := range res.Changed {
		if it.RejectReason != store.ReasonCancelled || it.ActualEnd == 0 {
			t.Fatalf("%s: reason %q, actual end %d", it.Identifier, it.RejectReason, it.ActualEnd)
		}
	}
	// Without a reason the whole selection is refused, item by item.
	c := w.ticket(w.sol, w.marta, "three")
	res, _ = w.ApplyBulkEdit(w.ctx, w.marta, BulkEdit{
		ItemIDs: []int64{c.ID}, Field: BulkState, Value: rejected.ID})
	if len(res.Refused) != 1 {
		t.Fatal("a reasonless bulk rejection was allowed")
	}
}

func TestBulkEditRejectsUnknownFields(t *testing.T) {
	w := newWorld(t)
	it := w.ticket(w.sol, w.marta, "one")
	// Title, description, planned end and labels are deferred (SPEC.md §3.3).
	for _, field := range []string{"title", "description", "plannedEnd", "labels"} {
		_, err := w.ApplyBulkEdit(w.ctx, w.marta, BulkEdit{
			ItemIDs: []int64{it.ID}, Field: field, Value: 1})
		requireCode(t, err, CodeInvalid)
	}
}

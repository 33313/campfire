package core

import (
	"testing"

	"github.com/campfire/campfire/internal/store"
)

// SPEC.md §10: the log is ordered by sequence number, never by timestamp, and
// paginated by keyset, never by offset.
func TestLogIsOrderedBySequenceAndPagedByKeyset(t *testing.T) {
	w := newWorld(t)
	// Freeze the clock so timestamps tie and only the sequence can order them.
	w.Clock = func() int64 { return 1_700_000_000_000_000 }
	for i := 0; i < 25; i++ {
		w.ticket(w.sol, w.marta, "item")
	}
	first, err := w.Log(w.ctx, w.marta, store.EventFilter{TeamID: w.sol.ID}, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Events) != 10 || first.Cursor == 0 {
		t.Fatalf("first page: %d events, cursor %d", len(first.Events), first.Cursor)
	}
	for i := 1; i < len(first.Events); i++ {
		if first.Events[i].Seq >= first.Events[i-1].Seq {
			t.Fatal("the log is not in descending sequence order")
		}
	}
	second, err := w.Log(w.ctx, w.marta, store.EventFilter{TeamID: w.sol.ID}, first.Cursor, 10)
	if err != nil {
		t.Fatal(err)
	}
	if second.Events[0].Seq >= first.Events[len(first.Events)-1].Seq {
		t.Fatal("the second page overlaps the first")
	}
}

// The global log is for instance administrators only; an item's history and a
// team's log are not (SPEC.md §10).
func TestGlobalLogIsAdministratorsOnly(t *testing.T) {
	w := newWorld(t)
	it := w.ticket(w.sol, w.marta, "work")
	_, err := w.Log(w.ctx, w.marta, store.EventFilter{}, 0, 50)
	requireCode(t, err, CodeForbidden)
	if _, err := w.Log(w.ctx, w.admin, store.EventFilter{}, 0, 50); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Log(w.ctx, w.marta, store.EventFilter{ItemID: it.ID}, 0, 50); err != nil {
		t.Fatalf("item history was refused: %v", err)
	}
	if _, err := w.Log(w.ctx, w.sam, store.EventFilter{TeamID: w.sol.ID}, 0, 50); err != nil {
		t.Fatalf("another team's log was refused: %v", err)
	}
}

// SPEC.md §14.4: every mutation through the agent interface records the token
// the call arrived on, as a real column, null for anything done in the client.
func TestAgentActionsCarryTheirToken(t *testing.T) {
	w := newWorld(t)
	issued, err := w.IssueAgentToken(w.ctx, w.marta, "laptop")
	if err != nil {
		t.Fatal(err)
	}
	agent, _ := w.ResolveAgentToken(w.ctx, issued.Token)
	byAgent, err := w.CreateItem(w.ctx, agent, NewItem{Kind: store.KindTicket,
		TeamID: w.sol.ID, Title: "made by an agent"})
	if err != nil {
		t.Fatal(err)
	}
	byHand := w.ticket(w.sol, w.marta, "made in the client")

	page, _ := w.Log(w.ctx, w.marta, store.EventFilter{ItemID: byAgent.ID}, 0, 50)
	e := page.Events[len(page.Events)-1]
	if e.AgentTokenID != issued.Meta.ID {
		t.Fatalf("the agent's event carries token %d", e.AgentTokenID)
	}
	if e.ActorID != w.marta.UserID {
		t.Fatal("the actor is not the user the agent acts as")
	}
	// History surfaces name the token beside the actor.
	if e.ActorName != "marta" || e.AgentName != "laptop" {
		t.Fatalf("the log renders %q / %q", e.ActorName, e.AgentName)
	}

	page, _ = w.Log(w.ctx, w.marta, store.EventFilter{ItemID: byHand.ID}, 0, 50)
	if page.Events[len(page.Events)-1].AgentTokenID != 0 {
		t.Fatal("a client action carries an agent token")
	}

	// "What did agents do here" is a real query, not a scan.
	agentOnly, err := w.Log(w.ctx, w.admin, store.EventFilter{AgentOnly: true}, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(agentOnly.Events) == 0 {
		t.Fatal("filtering the log by agent found nothing")
	}
	for _, e := range agentOnly.Events {
		if e.AgentTokenID == 0 {
			t.Fatal("the agent filter returned a client action")
		}
	}
}

// Events are append-only: nothing in the application edits or deletes one
// (SPEC.md §10). Deleting an item leaves its history behind.
func TestDeletingAnItemLeavesItsHistory(t *testing.T) {
	w := newWorld(t)
	it := w.ticket(w.sol, w.marta, "doomed")
	before, _ := w.Log(w.ctx, w.marta, store.EventFilter{TeamID: w.sol.ID}, 0, 100)
	if err := w.DeleteItem(w.ctx, w.admin, it.ID); err != nil {
		t.Fatal(err)
	}
	after, _ := w.Log(w.ctx, w.marta, store.EventFilter{TeamID: w.sol.ID}, 0, 100)
	if len(after.Events) <= len(before.Events) {
		t.Fatal("deleting an item removed log entries")
	}
	var found bool
	for _, e := range after.Events {
		if e.Kind == store.EvItemDeleted {
			found = true
		}
	}
	if !found {
		t.Fatal("the deletion wrote no event")
	}
}

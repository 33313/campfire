package core

import (
	"testing"

	"github.com/campfire/campfire/internal/store"
)

// SPEC.md §7.2: the board shows every state except the default and the
// rejected one; the backlog shows the default state; rejected items appear on
// no board.
func TestBoardAndBacklogSplitTheStateModel(t *testing.T) {
	w := newWorld(t)
	inBacklog := w.ticket(w.sol, w.marta, "waiting")
	accepted, err := w.CreateItem(w.ctx, w.marta, NewItem{Kind: store.KindTicket,
		TeamID: w.sol.ID, Title: "agreed", Accepted: true})
	if err != nil {
		t.Fatal(err)
	}
	rejected := w.ticket(w.sol, w.marta, "not doing this")
	if _, err := w.MoveItem(w.ctx, w.marta, rejected.ID,
		w.state(w.sol, store.RoleRejected).ID, store.ReasonCancelled, ""); err != nil {
		t.Fatal(err)
	}

	board, err := w.Board(w.ctx, w.marta, w.sol.ID, Filter{}, false, false, 50)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, col := range board.Columns {
		if col.State.Role == store.RoleDefault || col.State.Role == store.RoleRejected {
			t.Fatalf("the board has a %q column", col.State.Role)
		}
		for _, it := range col.Items {
			seen[it.Identifier] = true
		}
	}
	if seen[inBacklog.Identifier] {
		t.Fatal("a backlog item appeared on the board")
	}
	if seen[rejected.Identifier] {
		t.Fatal("a rejected item appeared on the board")
	}
	if !seen[accepted.Identifier] {
		t.Fatal("an accepted item is missing from the board")
	}

	backlog, err := w.Backlog(w.ctx, w.marta, w.sol.ID, Filter{}, false, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(backlog.Items) != 1 || backlog.Items[0].Identifier != inBacklog.Identifier {
		t.Fatalf("backlog holds %+v", backlog.Items)
	}
}

// SPEC.md §6.2: buckets are hidden from board columns by default and shown by
// a per-board toggle.
func TestBucketsAreHiddenFromBoardsUnlessAskedFor(t *testing.T) {
	w := newWorld(t)
	bucket := w.bucket(w.sol, w.marta, "Release 1.0")
	if _, err := w.MoveItem(w.ctx, w.marta, bucket.ID,
		w.state(w.sol, store.RoleAccepted).ID, "", ""); err != nil {
		t.Fatal(err)
	}
	count := func(showBuckets bool) int {
		board, err := w.Board(w.ctx, w.marta, w.sol.ID, Filter{}, showBuckets, false, 50)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, col := range board.Columns {
			n += len(col.Items)
		}
		return n
	}
	if count(false) != 0 {
		t.Fatal("a bucket showed on the board with the toggle off")
	}
	if count(true) != 1 {
		t.Fatal("a bucket did not show on the board with the toggle on")
	}
}

// SPEC.md §9.3: dropping an item between two neighbours gives it a rank
// sorting between theirs, and a move writes one row.
func TestRankingOrdersAColumn(t *testing.T) {
	w := newWorld(t)
	a := w.ticket(w.sol, w.marta, "a")
	b := w.ticket(w.sol, w.marta, "b")
	c := w.ticket(w.sol, w.marta, "c")

	order := func() []string {
		col, err := w.Backlog(w.ctx, w.marta, w.sol.ID, Filter{}, false, 50)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, it := range col.Items {
			out = append(out, it.Title)
		}
		return out
	}
	if got := order(); got[0] != "a" || got[2] != "c" {
		t.Fatalf("initial order %v", got)
	}
	// Drag c to the top.
	if _, err := w.RankItem(w.ctx, w.marta, c.ID, 0, 0, a.ID); err != nil {
		t.Fatal(err)
	}
	if got := order(); got[0] != "c" || got[1] != "a" || got[2] != "b" {
		t.Fatalf("after dragging c to the top: %v", got)
	}
	// And between a and b.
	if _, err := w.RankItem(w.ctx, w.marta, c.ID, 0, a.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	if got := order(); got[0] != "a" || got[1] != "c" || got[2] != "b" {
		t.Fatalf("after dragging c into the middle: %v", got)
	}
	_ = b
}

func TestRankingCanAlsoMoveBetweenColumns(t *testing.T) {
	w := newWorld(t)
	it := w.ticket(w.sol, w.marta, "a")
	accepted := w.state(w.sol, store.RoleAccepted)
	moved, err := w.RankItem(w.ctx, w.marta, it.ID, accepted.ID, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if moved.StateID != accepted.ID {
		t.Fatal("the drag across columns did not change state")
	}
	if moved.ActualStart == 0 {
		t.Fatal("a drag out of the default state did not start the clock")
	}
}

// SPEC.md §9.2: structured filters, composable across the dimensions.
func TestFilteringAcrossDimensions(t *testing.T) {
	w := newWorld(t)
	bucket := w.bucket(w.sol, w.marta, "Release 1.0")
	hit, err := w.CreateItem(w.ctx, w.marta, NewItem{Kind: store.KindTicket, TeamID: w.sol.ID,
		Title: "rate limiter for the api", Priority: 1, AssigneeID: w.marta.UserID,
		BucketID: bucket.ID, Labels: []string{"api"}})
	if err != nil {
		t.Fatal(err)
	}
	w.ticket(w.sol, w.marta, "something else entirely")

	labels, _ := w.ListLabels(w.ctx, w.marta)
	var apiLabel int64
	for _, l := range labels {
		if l.Name == "api" {
			apiLabel = l.ID
		}
	}
	page, err := w.ListItems(w.ctx, w.marta, Filter{
		TeamIDs: []int64{w.sol.ID}, LabelIDs: []int64{apiLabel},
		AssigneeIDs: []int64{w.marta.UserID}, Priorities: []int64{1},
		BucketIDs: []int64{bucket.ID}, Text: "rate limiter"}, "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].Identifier != hit.Identifier {
		t.Fatalf("composed filter returned %+v", page.Items)
	}

	// Free text searches title and description, and matches case-insensitively
	// enough to be useful.
	page, _ = w.ListItems(w.ctx, w.marta, Filter{Text: "RATE LIMITER"}, "", 50)
	if len(page.Items) != 1 {
		t.Fatalf("case-insensitive text search returned %d items", len(page.Items))
	}
	// Unassigned is its own dimension, not an assignee of nobody.
	page, _ = w.ListItems(w.ctx, w.marta, Filter{TeamIDs: []int64{w.sol.ID}, Unassigned: true}, "", 50)
	for _, it := range page.Items {
		if it.AssigneeID != 0 {
			t.Fatal("the unassigned filter returned an assigned item")
		}
	}
}

// SPEC.md §9.1: the bucket index defaults to participation — buckets my team
// owns, plus buckets containing at least one of my team's tickets.
func TestBucketIndexScopes(t *testing.T) {
	w := newWorld(t)
	ours := w.bucket(w.sol, w.marta, "Ours")
	theirs := w.bucket(w.ter, w.sam, "Theirs")
	unrelated := w.bucket(w.ter, w.sam, "Unrelated")

	// One of SOL's tickets goes into TERRA's bucket.
	ticket := w.ticket(w.sol, w.marta, "our work")
	id := theirs.ID
	if _, err := w.UpdateItem(w.ctx, w.marta, ticket.ID, ItemPatch{BucketID: &id}); err != nil {
		t.Fatal(err)
	}

	names := func(scope string) map[string]bool {
		cards, _, err := w.BucketIndex(w.ctx, w.marta, w.sol.ID, scope, "", 50)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]bool{}
		for _, c := range cards {
			out[c.Title] = true
		}
		return out
	}
	got := names(ScopeParticipation)
	if !got["Ours"] || !got["Theirs"] {
		t.Fatalf("participation scope shows %v", got)
	}
	if got["Unrelated"] {
		t.Fatal("participation scope showed a bucket the team has nothing in")
	}
	if got := names(ScopeOwned); !got["Ours"] || got["Theirs"] {
		t.Fatalf("owned scope shows %v", got)
	}
	if got := names(ScopeInstance); !got["Unrelated"] {
		t.Fatalf("instance scope shows %v", got)
	}
	_ = ours
	_ = unrelated
}

func TestBucketContentsAreGroupedByOwningTeam(t *testing.T) {
	w := newWorld(t)
	bucket := w.bucket(w.sol, w.marta, "Release 1.0")
	mine := w.ticket(w.sol, w.marta, "mine")
	theirs := w.ticket(w.ter, w.sam, "theirs")
	id := bucket.ID
	for a, it := range map[Actor]*Item{w.marta: mine, w.sam: theirs} {
		if _, err := w.UpdateItem(w.ctx, a, it.ID, ItemPatch{BucketID: &id}); err != nil {
			t.Fatal(err)
		}
	}
	groups, err := w.BucketContents(w.ctx, w.marta, bucket.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 {
		t.Fatalf("%d groups", len(groups))
	}
	for _, g := range groups {
		for _, it := range g.Items {
			if it.TeamID != g.Team.ID {
				t.Fatal("an item is grouped under the wrong team")
			}
		}
	}
}

// SPEC.md §9.3: UNIQUE (state_id, rank) is enforced with retry on conflict,
// because two clients can compute the same midpoint from the same read.
//
// The collision is forced here by planting an item on the midpoint the drop is
// about to compute, which is exactly what the losing client finds.
func TestRankRetriesWhenTheMidpointIsTaken(t *testing.T) {
	w := newWorld(t)
	a := w.ticket(w.sol, w.marta, "a")
	b := w.ticket(w.sol, w.marta, "b")
	mover := w.ticket(w.sol, w.marta, "mover")

	first, _ := w.GetItem(w.ctx, w.marta, a.ID)
	second, _ := w.GetItem(w.ctx, w.marta, b.ID)
	taken, err := store.RankBetween(first.Rank, second.Rank)
	if err != nil {
		t.Fatal(err)
	}
	// The winner of the race: an item already sitting on that exact rank.
	winner := w.ticket(w.sol, w.marta, "winner")
	if _, err := w.DB.W.Exec(`UPDATE items SET rank = ? WHERE id = ?`, taken, winner.ID); err != nil {
		t.Fatal(err)
	}

	moved, err := w.RankItem(w.ctx, w.marta, mover.ID, 0, a.ID, b.ID)
	if err != nil {
		t.Fatalf("the losing drop was refused instead of retried: %v", err)
	}
	if moved.Rank == taken {
		t.Fatal("the retry produced the rank that was already taken")
	}
	// It landed in the gap immediately after a — still between the two
	// neighbours the drop named, and in a gap nothing else occupies, so a
	// second retry could not lose either.
	if !(moved.Rank > first.Rank && moved.Rank < second.Rank) {
		t.Fatalf("retried rank %q is not between %q and %q", moved.Rank, first.Rank, second.Rank)
	}

	col, err := w.Backlog(w.ctx, w.marta, w.sol.ID, Filter{}, false, 50)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(col.Items); i++ {
		if col.Items[i].Rank <= col.Items[i-1].Rank {
			t.Fatal("the column is no longer strictly ordered")
		}
	}
}

// A drop past the last card names no neighbour on either side, which is the
// end of the column — not the fixed midpoint the first card already holds.
func TestRankingWithNoNeighbourLandsAtTheEnd(t *testing.T) {
	w := newWorld(t)
	a := w.ticket(w.sol, w.marta, "a")
	w.ticket(w.sol, w.marta, "b")
	if _, err := w.RankItem(w.ctx, w.marta, a.ID, 0, 0, 0); err != nil {
		t.Fatal(err)
	}
	col, err := w.Backlog(w.ctx, w.marta, w.sol.ID, Filter{}, false, 50)
	if err != nil {
		t.Fatal(err)
	}
	if last := col.Items[len(col.Items)-1].Title; last != "a" {
		t.Fatalf("a dropped past the last card sits at %q, not the end", last)
	}
}

// SPEC.md §9.4: a saved view applies as an ordinary filter. A state clause it
// carries narrows the board rather than being silently dropped.
func TestBoardHonoursAStateClauseFromAView(t *testing.T) {
	w := newWorld(t)
	accepted := w.state(w.sol, store.RoleAccepted)
	it := w.ticket(w.sol, w.marta, "moved")
	if _, err := w.RankItem(w.ctx, w.marta, it.ID, accepted.ID, 0, 0); err != nil {
		t.Fatal(err)
	}
	w.ticket(w.sol, w.marta, "stays")

	board, err := w.Board(w.ctx, w.marta, w.sol.ID, Filter{StateIDs: []int64{accepted.ID}}, false, false, 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, col := range board.Columns {
		if col.State.ID != accepted.ID && len(col.Items) > 0 {
			t.Fatalf("column %q ignored the view's state clause", col.State.Name)
		}
	}
}

// A bucket's progress separates what is finished from what has been picked up.
// Both are views over its contents, counted the same way as the "37 / 50"
// (SPEC.md §6.2): nothing is stored on the bucket.
func TestBucketProgressCountsStartedWorkSeparately(t *testing.T) {
	w := newWorld(t)
	milestone := w.bucket(w.sol, w.marta, "Winter release")

	// One left in the backlog, one accepted, one in a state the team invented,
	// one done and one rejected.
	inBacklog := w.ticket(w.sol, w.marta, "not started")
	accepted := w.ticket(w.sol, w.marta, "picked up")
	underway := w.ticket(w.sol, w.marta, "waiting on a part")
	done := w.ticket(w.sol, w.marta, "finished")
	dropped := w.ticket(w.sol, w.marta, "not doing this")
	for _, it := range []*Item{inBacklog, accepted, underway, done, dropped} {
		if _, err := w.UpdateItem(w.ctx, w.marta, it.ID,
			ItemPatch{BucketID: &milestone.ID}); err != nil {
			t.Fatal(err)
		}
	}

	// An intermediate state carries no role at all, and still counts as work
	// somebody is holding.
	intermediate, err := w.CreateWorkState(w.ctx, w.admin, w.sol.ID, "Awaiting parts", "")
	if err != nil {
		t.Fatal(err)
	}
	move := func(it *Item, stateID int64, reason string) {
		t.Helper()
		fresh, err := w.GetItem(w.ctx, w.marta, it.ID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.MoveItem(w.ctx, w.marta, fresh.ID, stateID, reason, ""); err != nil {
			t.Fatal(err)
		}
	}
	move(accepted, w.state(w.sol, store.RoleAccepted).ID, "")
	move(underway, intermediate.ID, "")
	move(done, w.state(w.sol, store.RoleFinal).ID, "")
	move(dropped, w.state(w.sol, store.RoleRejected).ID, store.ReasonCancelled)

	p, err := progress(w.ctx, w.DB.R, milestone.ID)
	if err != nil {
		t.Fatal(err)
	}
	if p.Total != 5 {
		t.Fatalf("total %d, want 5", p.Total)
	}
	// Done is terminal, either way it got there.
	if p.Done != 2 {
		t.Fatalf("done %d, want 2 (the finished one and the rejected one)", p.Done)
	}
	// Accepted and the intermediate state; never the backlog, never a
	// terminal state.
	if p.Accepted != 2 {
		t.Fatalf("accepted %d, want 2 (the accepted one and the one in progress)", p.Accepted)
	}
}

// SPEC.md §7.2: an item done for longer than DoneFor leaves the board's final
// column unless the board asks for all of them.
func TestOldDoneItemsAreHiddenFromTheBoard(t *testing.T) {
	w := newWorld(t)
	now := int64(1_700_000_000_000_000)
	w.Clock = func() int64 { return now }
	done := w.state(w.sol, store.RoleFinal)
	it := w.ticket(w.sol, w.marta, "Sweep the chimney")
	if _, err := w.MoveItem(w.ctx, w.marta, it.ID, done.ID, "", ""); err != nil {
		t.Fatal(err)
	}
	final := func(allDone bool) Column {
		board, err := w.Board(w.ctx, w.marta, w.sol.ID, Filter{}, false, allDone, 50)
		if err != nil {
			t.Fatal(err)
		}
		for _, col := range board.Columns {
			if col.State.ID == done.ID {
				return col
			}
		}
		t.Fatal("no final column")
		return Column{}
	}

	now += DoneFor
	if col := final(false); len(col.Items) != 1 || col.Count != 1 {
		t.Fatalf("done for exactly DoneFor: %d items, count %d", len(col.Items), col.Count)
	}
	now++
	if col := final(false); len(col.Items) != 0 || col.Count != 0 {
		t.Fatalf("done for longer than DoneFor: %d items, count %d", len(col.Items), col.Count)
	}
	if col := final(true); len(col.Items) != 1 || col.Count != 1 {
		t.Fatalf("allDone: %d items, count %d", len(col.Items), col.Count)
	}
}

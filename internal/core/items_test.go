package core

import (
	"strings"
	"testing"

	"github.com/campfire/campfire/internal/store"
)

// world sets up two teams with a member each, which is what most of the
// interesting rules need.
type world struct {
	*fixture
	sol, ter   *store.Team
	marta, sam Actor
}

func newWorld(t *testing.T) *world {
	f := newFixture(t)
	w := &world{fixture: f}
	w.marta = f.user("marta")
	w.sam = f.user("sam")
	w.sol = f.team("Solstice", "SOL", f.admin)
	w.ter = f.team("Terra", "TER", f.admin)
	f.join(w.sol, w.marta, "member")
	f.join(w.ter, w.sam, "member")
	return w
}

func (w *world) ticket(team *store.Team, a Actor, title string) *Item {
	w.t.Helper()
	it, err := w.CreateItem(w.ctx, a, NewItem{Kind: store.KindTicket, TeamID: team.ID, Title: title})
	if err != nil {
		w.t.Fatalf("create ticket: %v", err)
	}
	return it
}

func (w *world) bucket(team *store.Team, a Actor, title string) *Item {
	w.t.Helper()
	it, err := w.CreateItem(w.ctx, a, NewItem{Kind: store.KindBucket, TeamID: team.ID, Title: title})
	if err != nil {
		w.t.Fatalf("create bucket: %v", err)
	}
	return it
}

// SPEC.md §6.3: each team has one counter, shared by tickets and buckets, so
// SOL-42 may be a bucket and SOL-43 a ticket. The identifier says nothing
// about which.
func TestOneCounterServesTicketsAndBuckets(t *testing.T) {
	w := newWorld(t)
	a := w.ticket(w.sol, w.marta, "first")
	b := w.bucket(w.sol, w.marta, "Release 1.0")
	c := w.ticket(w.sol, w.marta, "second")
	if a.Identifier != "SOL-1" || b.Identifier != "SOL-2" || c.Identifier != "SOL-3" {
		t.Fatalf("identifiers %s %s %s", a.Identifier, b.Identifier, c.Identifier)
	}
	// A second team starts its own count.
	d := w.ticket(w.ter, w.sam, "theirs")
	if d.Identifier != "TER-1" {
		t.Fatalf("second team's first item is %s", d.Identifier)
	}
}

func TestIdentifiersAreNeverReused(t *testing.T) {
	w := newWorld(t)
	first := w.ticket(w.sol, w.marta, "gone soon")
	if err := w.DeleteItem(w.ctx, w.admin, first.ID); err != nil {
		t.Fatal(err)
	}
	next := w.ticket(w.sol, w.marta, "after")
	if next.Identifier == first.Identifier {
		t.Fatalf("identifier %s was minted twice", next.Identifier)
	}
}

// SPEC.md §6.1: a ticket has no icon, and a bucket cannot sit inside another.
func TestKindRulesAreEnforced(t *testing.T) {
	w := newWorld(t)
	_, err := w.CreateItem(w.ctx, w.marta, NewItem{Kind: store.KindTicket,
		TeamID: w.sol.ID, Title: "t", Icon: "bug"})
	requireCode(t, err, CodeInvalid)

	bucket := w.bucket(w.sol, w.marta, "Release 1.0")
	_, err = w.CreateItem(w.ctx, w.marta, NewItem{Kind: store.KindBucket,
		TeamID: w.sol.ID, Title: "nested", BucketID: bucket.ID})
	requireCode(t, err, CodeInvalid)

	inner := w.bucket(w.sol, w.marta, "inner")
	id := bucket.ID
	_, err = w.UpdateItem(w.ctx, w.marta, inner.ID, ItemPatch{BucketID: &id})
	requireCode(t, err, CodeInvalid)
}

func TestBucketGetsTheDefaultIconAndOnlyKnownIcons(t *testing.T) {
	w := newWorld(t)
	b := w.bucket(w.sol, w.marta, "Release 1.0")
	if b.Icon != store.DefaultBucketIcon {
		t.Fatalf("a new bucket carries icon %q", b.Icon)
	}
	bad := "rocket"
	_, err := w.UpdateItem(w.ctx, w.marta, b.ID, ItemPatch{Icon: &bad})
	requireCode(t, err, CodeInvalid)
}

// SPEC.md §6.1: the reporter is the creating user and is not editable; the
// assignee is empty by default and never inferred from the reporter.
func TestReporterIsSetAndAssigneeIsNot(t *testing.T) {
	w := newWorld(t)
	it := w.ticket(w.sol, w.marta, "work")
	if it.ReporterID != w.marta.UserID {
		t.Fatalf("reporter is %d", it.ReporterID)
	}
	if it.AssigneeID != 0 {
		t.Fatal("a new ticket arrived with an assignee")
	}
	if it.Priority != 3 {
		t.Fatalf("default priority is %d, want 3", it.Priority)
	}
}

// SPEC.md §7.3: a new item enters the default state, or the accepted state if
// the creator ticks "Mark as accepted" — and accepting starts its clock.
func TestCreationEntersDefaultOrAcceptedState(t *testing.T) {
	w := newWorld(t)
	plain := w.ticket(w.sol, w.marta, "plain")
	if plain.StateRole != store.RoleDefault {
		t.Fatalf("a new ticket is in the %q state", plain.StateRole)
	}
	if plain.ActualStart != 0 {
		t.Fatal("an unaccepted ticket already has an actual start")
	}
	accepted, err := w.CreateItem(w.ctx, w.marta, NewItem{Kind: store.KindTicket,
		TeamID: w.sol.ID, Title: "accepted", Accepted: true})
	if err != nil {
		t.Fatal(err)
	}
	if accepted.StateRole != store.RoleAccepted {
		t.Fatalf("marked as accepted, but in the %q state", accepted.StateRole)
	}
	if accepted.ActualStart == 0 {
		t.Fatal("accepting work did not start its clock")
	}
}

// SPEC.md §6.1, §7.3: actual start is set on the first move out of the default
// state and never again; actual end is set on entering a terminal state and
// cleared on leaving one.
func TestLifecycleTimestamps(t *testing.T) {
	w := newWorld(t)
	it := w.ticket(w.sol, w.marta, "work")
	states, _ := w.ListWorkStates(w.ctx, w.marta, w.sol.ID)
	byRole := map[string]store.WorkState{}
	var intermediate []store.WorkState
	for _, st := range states {
		if st.Role == "" {
			intermediate = append(intermediate, st)
		} else {
			byRole[st.Role] = st
		}
	}

	moved, err := w.MoveItem(w.ctx, w.marta, it.ID, byRole[store.RoleAccepted].ID, "", "")
	if err != nil {
		t.Fatal(err)
	}
	start := moved.ActualStart
	if start == 0 {
		t.Fatal("the first move out of the default state did not set actual start")
	}

	moved, _ = w.MoveItem(w.ctx, w.marta, it.ID, intermediate[0].ID, "", "")
	if moved.ActualStart != start {
		t.Fatal("actual start was rewritten by a later move")
	}
	if moved.ActualEnd != 0 {
		t.Fatal("an intermediate state set actual end")
	}

	done, err := w.MoveItem(w.ctx, w.marta, it.ID, byRole[store.RoleFinal].ID, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if done.ActualEnd == 0 {
		t.Fatal("entering a terminal state did not set actual end")
	}
	back, _ := w.MoveItem(w.ctx, w.marta, it.ID, intermediate[0].ID, "", "")
	if back.ActualEnd != 0 {
		t.Fatal("leaving a terminal state did not clear actual end")
	}
	if back.ActualStart != start {
		t.Fatal("actual start moved on the way back")
	}
}

// SPEC.md §7.3: any item may move to any state at any time. No transition
// graph, no guard rules.
func TestAnyItemMovesToAnyStateAtAnyTime(t *testing.T) {
	w := newWorld(t)
	it := w.ticket(w.sol, w.marta, "work")
	states, _ := w.ListWorkStates(w.ctx, w.marta, w.sol.ID)
	for _, st := range states {
		reason := ""
		if st.Role == store.RoleRejected {
			reason = store.ReasonCancelled
		}
		if _, err := w.MoveItem(w.ctx, w.marta, it.ID, st.ID, reason, ""); err != nil {
			t.Fatalf("move to %q refused: %v", st.Name, err)
		}
	}
}

func TestRejectionNeedsAReason(t *testing.T) {
	w := newWorld(t)
	it := w.ticket(w.sol, w.marta, "work")
	rejected := w.state(w.sol, store.RoleRejected)
	_, err := w.MoveItem(w.ctx, w.marta, it.ID, rejected.ID, "", "")
	requireCode(t, err, CodeInvalid)
	_, err = w.MoveItem(w.ctx, w.marta, it.ID, rejected.ID, "because", "")
	requireCode(t, err, CodeInvalid)
	got, err := w.MoveItem(w.ctx, w.marta, it.ID, rejected.ID, store.ReasonDuplicate, "same as SOL-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.RejectReason != store.ReasonDuplicate || got.RejectNote != "same as SOL-1" {
		t.Fatalf("rejection recorded as %q / %q", got.RejectReason, got.RejectNote)
	}
}

// SPEC.md §8.2: a relation carries no lifecycle meaning. A "blocked by" edge
// MUST NOT prevent any transition.
func TestBlockedByDoesNotPreventATransition(t *testing.T) {
	w := newWorld(t)
	blocker := w.ticket(w.sol, w.marta, "blocker")
	blocked := w.ticket(w.sol, w.marta, "blocked")
	if _, err := w.AddRelation(w.ctx, w.marta, blocker.ID, blocked.ID, store.RelBlocks); err != nil {
		t.Fatal(err)
	}
	final := w.state(w.sol, store.RoleFinal)
	if _, err := w.MoveItem(w.ctx, w.marta, blocked.ID, final.ID, "", ""); err != nil {
		t.Fatalf("a blocked item could not be moved: %v", err)
	}
}

func TestRelationsAreShownFromBothEnds(t *testing.T) {
	w := newWorld(t)
	blocker := w.ticket(w.sol, w.marta, "blocker")
	blocked := w.ticket(w.sol, w.marta, "blocked")
	if _, err := w.AddRelation(w.ctx, w.marta, blocker.ID, blocked.ID, store.RelBlocks); err != nil {
		t.Fatal(err)
	}
	from, _ := w.GetItem(w.ctx, w.marta, blocker.ID)
	if len(from.Relations) != 1 || from.Relations[0].Kind != "blocks" {
		t.Fatalf("forward end shows %+v", from.Relations)
	}
	to, _ := w.GetItem(w.ctx, w.marta, blocked.ID)
	if len(to.Relations) != 1 || to.Relations[0].Kind != "blocked_by" {
		t.Fatalf("far end shows %+v", to.Relations)
	}
	// Relations may cross teams and may link a ticket to a bucket.
	theirs := w.ticket(w.ter, w.sam, "theirs")
	bucket := w.bucket(w.sol, w.marta, "Release 1.0")
	if _, err := w.AddRelation(w.ctx, w.marta, blocker.ID, theirs.ID, store.RelRelatesTo); err != nil {
		t.Fatalf("cross-team relation refused: %v", err)
	}
	if _, err := w.AddRelation(w.ctx, w.marta, blocker.ID, bucket.ID, store.RelRelatesTo); err != nil {
		t.Fatalf("ticket-to-bucket relation refused: %v", err)
	}
}

// SPEC.md §6.4: bucket membership is a field on the ticket, governed by
// ordinary ticket-edit permission.
func TestBucketMembershipFollowsTicketEditPermission(t *testing.T) {
	w := newWorld(t)
	solTicket := w.ticket(w.sol, w.marta, "ours")
	terBucket := w.bucket(w.ter, w.sam, "Release 1.0")

	// A member of SOL may put any SOL ticket into any bucket in the instance.
	id := terBucket.ID
	if _, err := w.UpdateItem(w.ctx, w.marta, solTicket.ID, ItemPatch{BucketID: &id}); err != nil {
		t.Fatalf("cross-team bucket membership refused: %v", err)
	}
	// A user outside SOL may not add or remove SOL tickets — whoever owns the
	// bucket. Sam is only a member of TER.
	zero := int64(0)
	_, err := w.UpdateItem(w.ctx, w.sam, solTicket.ID, ItemPatch{BucketID: &zero})
	requireCode(t, err, CodeForbidden)
}

// SPEC.md §6.4, the one exception: an administrator of the bucket's owning
// team may remove a ticket from that bucket. Removal only.
func TestBucketOwnerAdminMayOnlyRemove(t *testing.T) {
	w := newWorld(t)
	w.join(w.ter, w.sam, "admin")
	solTicket := w.ticket(w.sol, w.marta, "ours")
	terBucket := w.bucket(w.ter, w.sam, "Release 1.0")
	id := terBucket.ID
	if _, err := w.UpdateItem(w.ctx, w.marta, solTicket.ID, ItemPatch{BucketID: &id}); err != nil {
		t.Fatal(err)
	}

	// Sam administers TER, which owns the bucket, so removal is allowed.
	zero := int64(0)
	got, err := w.UpdateItem(w.ctx, w.sam, solTicket.ID, ItemPatch{BucketID: &zero})
	if err != nil {
		t.Fatalf("the bucket owner's administrator could not remove the ticket: %v", err)
	}
	if got.BucketID != 0 {
		t.Fatal("the ticket is still in the bucket")
	}
	// It MUST be logged as an event on the ticket.
	page, err := w.Log(w.ctx, w.sam, store.EventFilter{ItemID: solTicket.ID}, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	var logged bool
	for _, e := range page.Events {
		if e.Field == "bucket" && e.ActorID == w.sam.UserID {
			logged = true
		}
	}
	if !logged {
		t.Fatal("the removal wrote no event on the ticket")
	}

	// Nothing else about the ticket is writable by them.
	title := "renamed by an outsider"
	_, err = w.UpdateItem(w.ctx, w.sam, solTicket.ID, ItemPatch{Title: &title})
	requireCode(t, err, CodeForbidden)
	// Nor is adding it back.
	_, err = w.UpdateItem(w.ctx, w.sam, solTicket.ID, ItemPatch{BucketID: &id})
	requireCode(t, err, CodeForbidden)
}

// SPEC.md §6.4: a bucket's lifecycle is independent of its contents in both
// directions.
func TestBucketLifecycleIsIndependentOfItsContents(t *testing.T) {
	w := newWorld(t)
	bucket := w.bucket(w.sol, w.marta, "Release 1.0")
	ticket := w.ticket(w.sol, w.marta, "inside")
	id := bucket.ID
	if _, err := w.UpdateItem(w.ctx, w.marta, ticket.ID, ItemPatch{BucketID: &id}); err != nil {
		t.Fatal(err)
	}
	final := w.state(w.sol, store.RoleFinal)
	if _, err := w.MoveItem(w.ctx, w.marta, bucket.ID, final.ID, "", ""); err != nil {
		t.Fatal(err)
	}
	after, _ := w.GetItem(w.ctx, w.marta, ticket.ID)
	if after.StateRole == store.RoleFinal {
		t.Fatal("completing the bucket moved its contents")
	}
	// And the other way: completing every ticket does not complete the bucket.
	if _, err := w.MoveItem(w.ctx, w.marta, bucket.ID, w.state(w.sol, store.RoleDefault).ID, "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := w.MoveItem(w.ctx, w.marta, ticket.ID, final.ID, "", ""); err != nil {
		t.Fatal(err)
	}
	b, _ := w.GetItem(w.ctx, w.marta, bucket.ID)
	if b.StateRole == store.RoleFinal {
		t.Fatal("the bucket auto-completed")
	}
	if b.Progress == nil || b.Progress.Done != 1 || b.Progress.Total != 1 {
		t.Fatalf("bucket progress is %+v, want 1 of 1", b.Progress)
	}
}

// SPEC.md §6.3: a reassigned bucket keeps its identifier. The prefix records
// origin, not current ownership.
func TestReassignedBucketKeepsItsIdentifier(t *testing.T) {
	w := newWorld(t)
	bucket := w.bucket(w.sol, w.marta, "Release 1.0")
	before := bucket.Identifier

	_, err := w.ReassignBucket(w.ctx, w.marta, bucket.ID, w.ter.ID)
	requireCode(t, err, CodeForbidden)

	moved, err := w.ReassignBucket(w.ctx, w.admin, bucket.ID, w.ter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if moved.Identifier != before {
		t.Fatalf("identifier changed from %s to %s", before, moved.Identifier)
	}
	if moved.TeamID != w.ter.ID {
		t.Fatal("the bucket did not change owner")
	}
	// It is found among TERRA's own buckets now, by the stored ownership
	// field rather than by its prefix.
	cards, _, err := w.BucketIndex(w.ctx, w.sam, w.ter.ID, ScopeOwned, "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 || cards[0].Identifier != before {
		t.Fatalf("TERRA's owned buckets are %+v", cards)
	}
}

// A ticket's owning team MUST NOT change (SPEC.md §6.3).
func TestATicketsTeamCannotChange(t *testing.T) {
	w := newWorld(t)
	it := w.ticket(w.sol, w.marta, "stays put")
	_, err := w.ReassignBucket(w.ctx, w.admin, it.ID, w.ter.ID)
	requireCode(t, err, CodeInvalid)
}

func TestEditingRequiresMembershipOfTheOwningTeam(t *testing.T) {
	w := newWorld(t)
	it := w.ticket(w.sol, w.marta, "ours")
	title := "theirs now"
	_, err := w.UpdateItem(w.ctx, w.sam, it.ID, ItemPatch{Title: &title})
	requireCode(t, err, CodeForbidden)
	// But reading is instance-wide.
	if _, err := w.GetItem(w.ctx, w.sam, it.ID); err != nil {
		t.Fatalf("reading another team's item was refused: %v", err)
	}
}

// SPEC.md §5.2: any authenticated user may comment on any item.
func TestAnyoneMayCommentAnywhere(t *testing.T) {
	w := newWorld(t)
	it := w.ticket(w.sol, w.marta, "ours")
	c, err := w.AddComment(w.ctx, w.sam, it.ID, "a thought from outside the team")
	if err != nil {
		t.Fatalf("an outsider could not comment: %v", err)
	}
	if _, err := w.UpdateComment(w.ctx, w.marta, c.ID, "rewritten"); err == nil {
		t.Fatal("someone else's comment was rewritten")
	}
}

func TestConcurrentEditIsRefusedOnAStaleVersion(t *testing.T) {
	w := newWorld(t)
	it := w.ticket(w.sol, w.marta, "work")
	first := "first edit"
	if _, err := w.UpdateItem(w.ctx, w.marta, it.ID, ItemPatch{Title: &first, Version: it.Version}); err != nil {
		t.Fatal(err)
	}
	second := "second edit from a stale view"
	_, err := w.UpdateItem(w.ctx, w.marta, it.ID, ItemPatch{Title: &second, Version: it.Version})
	requireCode(t, err, CodeConflict)
}

func TestTitleIsRequiredAndBounded(t *testing.T) {
	w := newWorld(t)
	_, err := w.CreateItem(w.ctx, w.marta, NewItem{Kind: store.KindTicket, TeamID: w.sol.ID, Title: "   "})
	requireCode(t, err, CodeInvalid)
	_, err = w.CreateItem(w.ctx, w.marta, NewItem{Kind: store.KindTicket, TeamID: w.sol.ID,
		Title: strings.Repeat("x", maxTitle+1)})
	requireCode(t, err, CodeInvalid)
}

// The create form offers every field the item has, so creating one is not
// followed by three edits to finish it (DEV-4).
func TestCreateWithAssigneeAndLabels(t *testing.T) {
	w := newWorld(t)
	it, err := w.CreateItem(w.ctx, w.marta, NewItem{Kind: store.KindTicket, TeamID: w.sol.ID,
		Title: "Replace the boiler", AssigneeID: w.sam.UserID, Labels: []string{"heating", "capex"}})
	if err != nil {
		t.Fatal(err)
	}
	if it.AssigneeID != w.sam.UserID || it.AssigneeName != "sam" {
		t.Fatalf("assignee %d/%q, want sam", it.AssigneeID, it.AssigneeName)
	}
	if len(it.Labels) != 2 {
		t.Fatalf("labels %v, want two", it.Labels)
	}
	// An assignee that is not a user is refused rather than reaching the
	// foreign key as a 500.
	_, err = w.CreateItem(w.ctx, w.marta, NewItem{Kind: store.KindTicket, TeamID: w.sol.ID,
		Title: "Nobody", AssigneeID: 9999})
	requireCode(t, err, CodeNotFound)
}

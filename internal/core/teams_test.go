package core

import (
	"testing"

	"github.com/33313/campfire/internal/store"
)

func TestCreateTeamGivesItTheSixDefaultStates(t *testing.T) {
	f := newFixture(t)
	team := f.team("Solstice", "SOL", f.admin)
	states, err := f.ListWorkStates(f.ctx, f.admin, team.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != len(store.DefaultStates) {
		t.Fatalf("%d states, want %d", len(states), len(store.DefaultStates))
	}
	roles := map[string]string{}
	for _, st := range states {
		if st.Role != "" {
			roles[st.Role] = st.Name
		}
	}
	for _, want := range []string{"default", "accepted", "rejected", "final"} {
		if roles[want] == "" {
			t.Fatalf("no state holds the %s role", want)
		}
	}
}

// SPEC.md §5.3: a short code is set at creation and MUST NOT change, and MUST
// NOT be reused after a team is archived — reuse would mint identifiers
// colliding with the archived team's.
func TestShortCodesAreUniqueAndSurviveArchival(t *testing.T) {
	f := newFixture(t)
	team := f.team("Solstice", "SOL", f.admin)
	if err := f.ArchiveTeam(f.ctx, f.admin, team.ID, true); err != nil {
		t.Fatal(err)
	}
	_, err := f.CreateTeam(f.ctx, f.admin, "Solar", "sol")
	requireCode(t, err, CodeConflict)
}

func TestShortCodeValidation(t *testing.T) {
	f := newFixture(t)
	for _, bad := range []string{"S", "TOOLONGCODE", "SO L", "SOL-", "SOL1"} {
		if _, err := f.CreateTeam(f.ctx, f.admin, "T "+bad, bad); err == nil {
			t.Fatalf("short code %q was accepted", bad)
		}
	}
}

// SPEC.md §7.1: a team MUST always have all four roles filled.
func TestATeamAlwaysHasAllFourRoles(t *testing.T) {
	f := newFixture(t)
	team := f.team("Solstice", "SOL", f.admin)
	final := f.state(team, store.RoleFinal)

	err := f.UpdateWorkState(f.ctx, f.admin, final.ID, "Done", "", 0)
	requireCode(t, err, CodeInvalid)

	// The role is still where it was.
	if got := f.state(team, store.RoleFinal); got.ID != final.ID {
		t.Fatal("the rolled-back update took the role away anyway")
	}
}

func TestMovingARoleTakesItFromTheStateThatHadIt(t *testing.T) {
	f := newFixture(t)
	team := f.team("Solstice", "SOL", f.admin)
	old := f.state(team, store.RoleFinal)
	shipped, err := f.CreateWorkState(f.ctx, f.admin, team.ID, "Shipped", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.UpdateWorkState(f.ctx, f.admin, shipped.ID, "Shipped", store.RoleFinal, 0); err != nil {
		t.Fatal(err)
	}
	if got := f.state(team, store.RoleFinal); got.ID != shipped.ID {
		t.Fatalf("the final role is on state %d, want %d", got.ID, shipped.ID)
	}
	after, err := store.WorkStateByID(f.ctx, f.DB.R, old.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Role != "" {
		t.Fatalf("the old state kept the role %q", after.Role)
	}
}

func TestDeletingAStateHoldingARoleIsRefused(t *testing.T) {
	f := newFixture(t)
	team := f.team("Solstice", "SOL", f.admin)
	requireCode(t, f.DeleteWorkState(f.ctx, f.admin, f.state(team, store.RoleDefault).ID), CodeInvalid)
}

func TestOnlyTeamAdministratorsChangeATeam(t *testing.T) {
	f := newFixture(t)
	marta := f.user("marta")
	team := f.team("Solstice", "SOL", f.admin)
	f.join(team, marta, "member")

	requireCode(t, f.RenameTeam(f.ctx, marta, team.ID, "Renamed"), CodeForbidden)
	requireCode(t, f.SetMember(f.ctx, marta, team.ID, marta.UserID, "admin"), CodeForbidden)
	if _, err := f.CreateWorkState(f.ctx, marta, team.ID, "Blocked", ""); err == nil {
		t.Fatal("a member added a work state")
	}
}

// SPEC.md §5.2: an instance administrator acts as a team administrator
// anywhere, without being a member of the team.
func TestInstanceAdministratorActsAsTeamAdministratorAnywhere(t *testing.T) {
	f := newFixture(t)
	marta := f.user("marta")
	team, err := f.CreateTeam(f.ctx, f.admin, "Terra", "TER")
	if err != nil {
		t.Fatal(err)
	}
	f.join(team, marta, "admin")
	if err := f.RemoveMember(f.ctx, f.admin, team.ID, f.admin.UserID); err != nil {
		t.Fatal(err)
	}
	if err := f.RenameTeam(f.ctx, f.admin, team.ID, "Terra Nova"); err != nil {
		t.Fatalf("instance administrator refused on a team they left: %v", err)
	}
}

// SPEC.md §5.2: a user may belong to any number of teams, with a different
// role in each.
func TestRolesAreIndependentPerTeam(t *testing.T) {
	f := newFixture(t)
	marta := f.user("marta")
	sol := f.team("Solstice", "SOL", f.admin)
	ter := f.team("Terra", "TER", f.admin)
	f.join(sol, marta, "admin")
	f.join(ter, marta, "member")

	if err := f.RenameTeam(f.ctx, marta, sol.ID, "Solstice II"); err != nil {
		t.Fatalf("team administrator refused in their own team: %v", err)
	}
	requireCode(t, f.RenameTeam(f.ctx, marta, ter.ID, "Terra II"), CodeForbidden)
}

// A team's picture and description are a team administrator's to write, and
// anyone's to read (DEV-36).
func TestTeamProfileIsTheTeamAdministrators(t *testing.T) {
	w := newWorld(t)
	if err := w.SetTeamProfile(w.ctx, w.marta, w.sol.ID, "", "we fix boilers"); err == nil {
		t.Fatal("a member wrote the team's profile")
	} else {
		requireCode(t, err, CodeForbidden)
	}
	if err := w.SetTeamProfile(w.ctx, w.admin, w.sol.ID, "", "  We fix boilers.  "); err != nil {
		t.Fatal(err)
	}
	team, err := w.Team(w.ctx, w.sam, w.sol.ID)
	if err != nil {
		t.Fatal(err)
	}
	if team.Description != "We fix boilers." {
		t.Fatalf("description %q", team.Description)
	}
	if err := w.SetTeamProfile(w.ctx, w.admin, w.sol.ID, "data:text/html,<b>", ""); err == nil {
		t.Fatal("a non-image picture was accepted")
	}
}

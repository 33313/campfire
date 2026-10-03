package core

import (
	"context"
	"strconv"
	"strings"

	"github.com/campfire/campfire/internal/store"
)

func validateShortCode(code string) error {
	code = strings.TrimSpace(code)
	if len(code) < 2 || len(code) > 8 {
		return invalidField("shortCode", "a short code is between 2 and 8 characters")
	}
	for _, r := range code {
		if !(r >= 'A' && r <= 'Z') && !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') {
			return invalidField("shortCode", "a short code is letters and digits only")
		}
	}
	// The identifier is <code>-<number>, so a code ending in a digit would
	// make SOL1-2 and SOL-12 ambiguous to any eye reading it back.
	if last := code[len(code)-1]; last >= '0' && last <= '9' {
		return invalidField("shortCode", "a short code may not end with a digit")
	}
	return notReserved("shortCode", code)
}

// CreateTeam gives the team the six work states of SPEC.md §7.1 and makes its
// creator an administrator of it.
func (s *Service) CreateTeam(ctx context.Context, a Actor, name, shortCode string) (*store.Team, error) {
	if err := RequireInstanceAdmin(a); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, invalidField("name", "a team needs a name")
	}
	if err := notReserved("name", name); err != nil {
		return nil, err
	}
	if err := validateShortCode(shortCode); err != nil {
		return nil, err
	}
	var out *store.Team
	err := s.write(ctx, a, func(t *wtx) error {
		team := store.Team{Name: name, ShortCode: strings.ToUpper(strings.TrimSpace(shortCode)),
			IconRendering: "mono", CreatedAt: t.now}
		id, err := store.CreateTeam(ctx, t.Tx, team)
		if err != nil {
			return err
		}
		if err := store.CreateDefaultStates(ctx, t.Tx, id, t.now); err != nil {
			return err
		}
		if err := store.SetMember(ctx, t.Tx, id, a.UserID, "admin", t.now); err != nil {
			return err
		}
		team, err = store.TeamByID(ctx, t.Tx, id)
		if err != nil {
			return err
		}
		out = &team
		return t.log(ctx, store.Event{Kind: store.EvTeamCreated, Entity: "team",
			EntityID: id, TeamID: id, NewValue: team.Name})
	})
	return out, err
}

// Reading is instance-wide, so every list here is open to any authenticated
// user (SPEC.md §3.2). Permissions govern writing.

func (s *Service) ListTeams(ctx context.Context, a Actor, includeArchived bool) ([]store.Team, error) {
	if err := requireUser(a); err != nil {
		return nil, err
	}
	teams, err := store.ListTeams(ctx, s.DB.R, includeArchived)
	return list(teams, mapErr(err))
}

func (s *Service) Team(ctx context.Context, a Actor, id int64) (*store.Team, error) {
	if err := requireUser(a); err != nil {
		return nil, err
	}
	team, err := store.TeamByID(ctx, s.DB.R, id)
	if err != nil {
		return nil, mapErr(err)
	}
	return &team, nil
}

// MyTeams is what the sidebar is drawn from.
func (s *Service) MyTeams(ctx context.Context, a Actor) ([]store.Member, error) {
	if err := requireUser(a); err != nil {
		return nil, err
	}
	m, err := store.ListUserTeams(ctx, s.DB.R, a.UserID)
	return list(m, mapErr(err))
}

// RenameTeam changes the name. The short code is set at creation and MUST NOT
// change (SPEC.md §5.3), so there is no operation here that changes it.
func (s *Service) RenameTeam(ctx context.Context, a Actor, teamID int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return invalidField("name", "a team needs a name")
	}
	if err := notReserved("name", name); err != nil {
		return err
	}
	return s.write(ctx, a, func(t *wtx) error {
		if err := s.requireTeamAdmin(ctx, t.Tx, a, teamID); err != nil {
			return err
		}
		old, err := store.TeamByID(ctx, t.Tx, teamID)
		if err != nil {
			return err
		}
		if err := store.RenameTeam(ctx, t.Tx, teamID, name); err != nil {
			return err
		}
		return t.log(ctx, store.Event{Kind: store.EvTeamUpdated, Entity: "team",
			EntityID: teamID, TeamID: teamID, Field: "name", OldValue: old.Name, NewValue: name})
	})
}

// SetIconRendering picks which rendering of the fixed icon set this team's
// surfaces use. It is presentation and MUST NOT change which icon a bucket
// carries (SPEC.md §6.2).
func (s *Service) SetIconRendering(ctx context.Context, a Actor, teamID int64, rendering string) error {
	if rendering != "mono" && rendering != "colour" {
		return invalidField("iconRendering", "icon rendering is mono or colour")
	}
	return s.write(ctx, a, func(t *wtx) error {
		if err := s.requireTeamAdmin(ctx, t.Tx, a, teamID); err != nil {
			return err
		}
		if err := store.SetIconRendering(ctx, t.Tx, teamID, rendering); err != nil {
			return err
		}
		return t.log(ctx, store.Event{Kind: store.EvTeamUpdated, Entity: "team",
			EntityID: teamID, TeamID: teamID, Field: "icon_rendering", NewValue: rendering})
	})
}

// SetTeamProfile sets the picture and the description a team's profile page
// shows. They are held to the same bounds as a user's picture and bio: the
// picture ships with every team list, and the description is a paragraph.
func (s *Service) SetTeamProfile(ctx context.Context, a Actor, teamID int64, avatar, description string) error {
	if err := validateAvatar(avatar); err != nil {
		return err
	}
	description = strings.TrimSpace(description)
	if len([]rune(description)) > maxBio {
		return invalidField("description", "a description is at most %d characters", maxBio)
	}
	return s.write(ctx, a, func(t *wtx) error {
		if err := s.requireTeamAdmin(ctx, t.Tx, a, teamID); err != nil {
			return err
		}
		if err := store.SetTeamProfile(ctx, t.Tx, teamID, avatar, description); err != nil {
			return err
		}
		return t.log(ctx, store.Event{Kind: store.EvTeamUpdated, Entity: "team",
			EntityID: teamID, TeamID: teamID, Field: "profile"})
	})
}

func (s *Service) ArchiveTeam(ctx context.Context, a Actor, teamID int64, archived bool) error {
	if err := RequireInstanceAdmin(a); err != nil {
		return err
	}
	return s.write(ctx, a, func(t *wtx) error {
		at := t.now
		if !archived {
			at = 0
		}
		if err := store.ArchiveTeam(ctx, t.Tx, teamID, at); err != nil {
			return err
		}
		return t.log(ctx, store.Event{Kind: store.EvTeamArchived, Entity: "team",
			EntityID: teamID, TeamID: teamID, Field: "archived",
			NewValue: strconv.FormatBool(archived)})
	})
}

func (s *Service) ListMembers(ctx context.Context, a Actor, teamID int64) ([]store.Member, error) {
	if err := requireUser(a); err != nil {
		return nil, err
	}
	m, err := store.ListMembers(ctx, s.DB.R, teamID)
	return list(m, mapErr(err))
}

func (s *Service) SetMember(ctx context.Context, a Actor, teamID, userID int64, role string) error {
	if role != "admin" && role != "member" {
		return invalidField("role", "a role is admin or member")
	}
	return s.write(ctx, a, func(t *wtx) error {
		if err := s.requireTeamAdmin(ctx, t.Tx, a, teamID); err != nil {
			return err
		}
		u, err := store.UserByID(ctx, t.Tx, userID)
		if err != nil {
			return err
		}
		if err := store.SetMember(ctx, t.Tx, teamID, userID, role, t.now); err != nil {
			return err
		}
		return t.log(ctx, store.Event{Kind: store.EvMemberAdded, Entity: "team",
			EntityID: teamID, TeamID: teamID, Field: "member",
			NewValue: u.Username + " (" + role + ")"})
	})
}

func (s *Service) RemoveMember(ctx context.Context, a Actor, teamID, userID int64) error {
	return s.write(ctx, a, func(t *wtx) error {
		if err := s.requireTeamAdmin(ctx, t.Tx, a, teamID); err != nil {
			return err
		}
		u, err := store.UserByID(ctx, t.Tx, userID)
		if err != nil {
			return err
		}
		if err := store.RemoveMember(ctx, t.Tx, teamID, userID); err != nil {
			return err
		}
		return t.log(ctx, store.Event{Kind: store.EvMemberRemoved, Entity: "team",
			EntityID: teamID, TeamID: teamID, Field: "member", OldValue: u.Username})
	})
}

// Work states. A team MUST always have all four roles filled (SPEC.md §7.1),
// which is enforced here on every change rather than only on creation.

func (s *Service) ListWorkStates(ctx context.Context, a Actor, teamID int64) ([]store.WorkState, error) {
	if err := requireUser(a); err != nil {
		return nil, err
	}
	states, err := store.ListWorkStates(ctx, s.DB.R, teamID)
	return list(states, mapErr(err))
}

func (s *Service) CreateWorkState(ctx context.Context, a Actor, teamID int64, name, role string) (*store.WorkState, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, invalidField("name", "a work state needs a name")
	}
	if err := notReserved("name", name); err != nil {
		return nil, err
	}
	if role != "" && !validRole(role) {
		return nil, invalidField("role", "a role is default, accepted, rejected or final")
	}
	var out *store.WorkState
	err := s.write(ctx, a, func(t *wtx) error {
		if err := s.requireTeamAdmin(ctx, t.Tx, a, teamID); err != nil {
			return err
		}
		existing, err := store.ListWorkStates(ctx, t.Tx, teamID)
		if err != nil {
			return err
		}
		if role != "" {
			// Roles are exclusive. Moving one is an edit of the state that
			// holds it, not a second holder.
			for _, e := range existing {
				if e.Role == role {
					return errf(CodeConflict, "%q already holds the %s role", e.Name, role)
				}
			}
		}
		st := store.WorkState{TeamID: teamID, Name: name, Role: role,
			Position: int64(len(existing) + 1), CreatedAt: t.now}
		id, err := store.CreateWorkState(ctx, t.Tx, st)
		if err != nil {
			return err
		}
		st.ID = id
		out = &st
		return t.log(ctx, store.Event{Kind: store.EvStateCreated, Entity: "work_state",
			EntityID: id, TeamID: teamID, NewValue: name})
	})
	return out, err
}

// UpdateWorkState renames, re-roles or repositions a state, and refuses any
// change that would leave one of the four roles unfilled.
func (s *Service) UpdateWorkState(ctx context.Context, a Actor, stateID int64, name, role string, position int64) error {
	if role != "" && !validRole(role) {
		return invalidField("role", "a role is default, accepted, rejected or final")
	}
	if strings.TrimSpace(name) != "" {
		if err := notReserved("name", name); err != nil {
			return err
		}
	}
	return s.write(ctx, a, func(t *wtx) error {
		st, err := store.WorkStateByID(ctx, t.Tx, stateID)
		if err != nil {
			return err
		}
		if err := s.requireTeamAdmin(ctx, t.Tx, a, st.TeamID); err != nil {
			return err
		}
		before, err := store.ListWorkStates(ctx, t.Tx, st.TeamID)
		if err != nil {
			return err
		}
		old := st
		if name = strings.TrimSpace(name); name != "" {
			st.Name = name
		}
		st.Role = role
		if position > 0 {
			st.Position = position
		}
		// The state currently holding the role being taken loses it, so that
		// one edit is one intent rather than two round trips.
		if role != "" {
			for _, e := range before {
				if e.Role == role && e.ID != st.ID {
					e.Role = ""
					if err := store.UpdateWorkState(ctx, t.Tx, e); err != nil {
						return err
					}
				}
			}
		}
		if err := store.UpdateWorkState(ctx, t.Tx, st); err != nil {
			return err
		}
		if err := s.requireAllRoles(ctx, t, st.TeamID); err != nil {
			return err
		}
		// A reorder changes neither the name nor the role, and logging
		// "In Progress → In Progress" says nothing. Record what moved.
		field, oldValue, newValue := "state", old.Name+" "+old.Role, st.Name+" "+st.Role
		if old.Name == st.Name && old.Role == st.Role {
			field = "order"
			oldValue = st.Name + " " + strconv.FormatInt(old.Position, 10)
			newValue = st.Name + " " + strconv.FormatInt(st.Position, 10)
		}
		return t.log(ctx, store.Event{Kind: store.EvStateUpdated, Entity: "work_state",
			EntityID: st.ID, TeamID: st.TeamID, Field: field,
			OldValue: oldValue, NewValue: newValue})
	})
}

func (s *Service) DeleteWorkState(ctx context.Context, a Actor, stateID int64) error {
	return s.write(ctx, a, func(t *wtx) error {
		st, err := store.WorkStateByID(ctx, t.Tx, stateID)
		if err != nil {
			return err
		}
		if err := s.requireTeamAdmin(ctx, t.Tx, a, st.TeamID); err != nil {
			return err
		}
		if st.Role != "" {
			return errf(CodeInvalid, "%q holds the %s role; give the role to another state first",
				st.Name, st.Role)
		}
		if err := store.DeleteWorkState(ctx, t.Tx, stateID); err != nil {
			return errf(CodeConflict, "%s", err.Error())
		}
		return t.log(ctx, store.Event{Kind: store.EvStateDeleted, Entity: "work_state",
			EntityID: stateID, TeamID: st.TeamID, OldValue: st.Name})
	})
}

// requireAllRoles holds SPEC.md §7.1's standing invariant: a team always has
// all four roles filled. Checked after the write, inside the transaction, so a
// change that would break it rolls back.
func (s *Service) requireAllRoles(ctx context.Context, t *wtx, teamID int64) error {
	states, err := store.ListWorkStates(ctx, t.Tx, teamID)
	if err != nil {
		return err
	}
	held := map[string]bool{}
	for _, st := range states {
		held[st.Role] = true
	}
	for _, role := range []string{store.RoleDefault, store.RoleAccepted, store.RoleRejected, store.RoleFinal} {
		if !held[role] {
			return errf(CodeInvalid, "a team must always have a %s state", role)
		}
	}
	return nil
}

func validRole(role string) bool {
	switch role {
	case store.RoleDefault, store.RoleAccepted, store.RoleRejected, store.RoleFinal:
		return true
	}
	return false
}

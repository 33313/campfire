package core

import (
	"context"

	"github.com/campfire/campfire/internal/store"
)

// The three roles of SPEC.md §5.2 and nothing else. Permissions govern
// writing; reading is instance-wide, so there is no read check anywhere in
// this file (§3.2).

// requireUser rejects an unauthenticated caller. Every mutation starts here.
func requireUser(a Actor) error {
	if !a.authenticated() {
		return &Error{Code: CodeUnauthenticated, Message: "sign in first"}
	}
	return nil
}

// teamRole returns "admin", "member" or "" for the actor in one team.
//
// An instance administrator acts as a team administrator anywhere (§5.2), and
// that is applied here rather than at each call site so that no operation can
// forget it.
func (s *Service) teamRole(ctx context.Context, q store.Q, a Actor, teamID int64) (string, error) {
	if a.IsAdmin {
		return "admin", nil
	}
	return store.MemberRole(ctx, q, teamID, a.UserID)
}

func (s *Service) isTeamMember(ctx context.Context, q store.Q, a Actor, teamID int64) (bool, error) {
	role, err := s.teamRole(ctx, q, a, teamID)
	return role != "", err
}

func (s *Service) isTeamAdmin(ctx context.Context, q store.Q, a Actor, teamID int64) (bool, error) {
	role, err := s.teamRole(ctx, q, a, teamID)
	return role == "admin", err
}

// requireTeamMember is the ordinary write permission: a member may create and
// edit items in teams they belong to.
func (s *Service) requireTeamMember(ctx context.Context, q store.Q, a Actor, teamID int64) error {
	if err := requireUser(a); err != nil {
		return err
	}
	ok, err := s.isTeamMember(ctx, q, a, teamID)
	if err != nil {
		return err
	}
	if !ok {
		return errForbidden
	}
	return nil
}

func (s *Service) requireTeamAdmin(ctx context.Context, q store.Q, a Actor, teamID int64) error {
	if err := requireUser(a); err != nil {
		return err
	}
	ok, err := s.isTeamAdmin(ctx, q, a, teamID)
	if err != nil {
		return err
	}
	if !ok {
		return errForbidden
	}
	return nil
}

// RequireInstanceAdmin refuses anyone but an instance administrator. It is
// exported for the snapshot routes, which need the configuration file and so
// live outside core.
func RequireInstanceAdmin(a Actor) error {
	if err := requireUser(a); err != nil {
		return err
	}
	if !a.IsAdmin {
		return errForbidden
	}
	return nil
}

// requireItemEdit is the permission every write to an item goes through: the
// actor must belong to the team that owns the item.
//
// Ownership is read from the stored field, never parsed out of the identifier
// (SPEC.md §6.3).
func (s *Service) requireItemEdit(ctx context.Context, q store.Q, a Actor, it store.Item) error {
	return s.requireTeamMember(ctx, q, a, it.TeamID)
}

// canRemoveFromBucket implements the single exception of SPEC.md §6.4.
//
// A user outside SOL may not add or remove SOL tickets, whoever owns the
// bucket — with one exception: an administrator of the bucket's owning team
// may *remove* a ticket from that bucket. Removal only. Without it a bucket's
// scope is world-writable; without the rest of the rule a small planning team
// becomes a bottleneck for work it does not do.
//
// It is the only operation that writes to a ticket outside the actor's team,
// and it is logged as an event on the ticket like any other change.
func (s *Service) canRemoveFromBucket(ctx context.Context, q store.Q, a Actor, ticket store.Item) (bool, error) {
	if ticket.BucketID == 0 {
		return false, nil
	}
	bucket, err := store.ItemByID(ctx, q, ticket.BucketID)
	if err != nil {
		return false, err
	}
	return s.isTeamAdmin(ctx, q, a, bucket.TeamID)
}

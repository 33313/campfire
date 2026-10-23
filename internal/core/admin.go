package core

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/33313/campfire/internal/auth"
	"github.com/33313/campfire/internal/store"
)

// Account creation is an administrator's act. Open registration WILL NOT be
// offered (SPEC.md §5.1): an administrator either creates the account or
// issues an invitation link and hands it over.

const linkLifetime = 7 * 24 * time.Hour

func validateUsername(name string) error {
	name = strings.TrimSpace(name)
	if len(name) < 2 || len([]rune(name)) > 40 {
		return invalidField("username", "a username is between 2 and 40 characters")
	}
	for _, r := range name {
		if r <= ' ' || r == '/' || r == '\\' || r == ':' {
			return invalidField("username", "a username may not contain spaces, slashes or colons")
		}
	}
	return notReserved("username", name)
}

// CreateUser is the instance administrator's direct route. The password is
// handed over out of band, the same way an invitation link is.
func (s *Service) CreateUser(ctx context.Context, a Actor, username, password, email string, isAdmin bool) (*User, error) {
	if err := RequireInstanceAdmin(a); err != nil {
		return nil, err
	}
	if err := validateUsername(username); err != nil {
		return nil, err
	}
	if err := validatePassword(password); err != nil {
		return nil, err
	}
	var out *User
	err := s.write(ctx, a, func(t *wtx) error {
		hash, err := auth.HashPassword(password)
		if err != nil {
			return err
		}
		u := store.User{Username: strings.TrimSpace(username), PasswordHash: hash,
			Email: strings.TrimSpace(email), IsAdmin: isAdmin, CreatedAt: t.now}
		id, err := store.CreateUser(ctx, t.Tx, u)
		if err != nil {
			return err
		}
		u.ID = id
		out = publicUser(u)
		return t.log(ctx, store.Event{Kind: store.EvUserCreated, Entity: "user",
			EntityID: id, NewValue: u.Username})
	})
	return out, err
}

// Invitation is a one-time link. It is shown once and only its hash is kept.
type Invitation struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expiresAt"`
	// Username is set for a password reset, naming the account it will act on.
	Username string `json:"username,omitempty"`
}

func (s *Service) CreateInvitation(ctx context.Context, a Actor) (*Invitation, error) {
	if err := RequireInstanceAdmin(a); err != nil {
		return nil, err
	}
	return s.oneTimeLink(ctx, a, "invite", 0, "")
}

// CreatePasswordReset issues a one-time reset for an account.
//
// A flow MUST NOT depend on email delivery (SPEC.md §5.1). Email is optional,
// so the administrator issues the link and hands it over; there is no mailed
// link anywhere in the application.
func (s *Service) CreatePasswordReset(ctx context.Context, a Actor, userID int64) (*Invitation, error) {
	if err := RequireInstanceAdmin(a); err != nil {
		return nil, err
	}
	u, err := store.UserByID(ctx, s.DB.R, userID)
	if err != nil {
		return nil, mapErr(err)
	}
	return s.oneTimeLink(ctx, a, "reset", userID, u.Username)
}

func (s *Service) oneTimeLink(ctx context.Context, a Actor, kind string, userID int64, username string) (*Invitation, error) {
	var out *Invitation
	err := s.write(ctx, a, func(t *wtx) error {
		token, hash, err := auth.NewToken()
		if err != nil {
			return err
		}
		expires := t.now + int64(linkLifetime/time.Microsecond)
		if err := store.ReapOneTimeLinks(ctx, t.Tx, t.now); err != nil {
			return err
		}
		l := store.OneTimeLink{Kind: kind, UserID: userID, CreatedBy: a.UserID,
			CreatedAt: t.now, ExpiresAt: expires}
		if _, err := store.CreateOneTimeLink(ctx, t.Tx, l, hash); err != nil {
			return err
		}
		out = &Invitation{Token: token, ExpiresAt: expires, Username: username}
		return t.log(ctx, store.Event{Kind: store.EvInstance, Entity: "instance",
			Field: kind + "_issued", NewValue: username})
	})
	return out, err
}

// RedeemInvitation creates the account behind an invitation link, or sets a
// new password behind a reset link. It is the one unauthenticated write in
// the application, and the link is spent atomically so two people cannot both
// redeem it.
func (s *Service) RedeemInvitation(ctx context.Context, token, username, password string) (*User, error) {
	if err := validatePassword(password); err != nil {
		return nil, err
	}
	var out *User
	err := s.write(ctx, Actor{}, func(t *wtx) error {
		l, err := store.OneTimeLinkByToken(ctx, t.Tx, auth.HashToken(token), t.now)
		if errors.Is(err, store.ErrNotFound) {
			return errf(CodeNotFound, "that link is not valid any more")
		}
		if err != nil {
			return err
		}
		if err := store.UseOneTimeLink(ctx, t.Tx, l.ID, t.now); err != nil {
			return errf(CodeConflict, "that link has already been used")
		}
		hash, err := auth.HashPassword(password)
		if err != nil {
			return err
		}
		if l.Kind == "reset" {
			if err := store.SetPassword(ctx, t.Tx, l.UserID, hash); err != nil {
				return err
			}
			// Every existing session for the account goes: a reset is issued
			// because access is in doubt.
			if err := store.DeleteUserSessions(ctx, t.Tx, l.UserID); err != nil {
				return err
			}
			u, err := store.UserByID(ctx, t.Tx, l.UserID)
			if err != nil {
				return err
			}
			out = publicUser(u)
			return t.log(ctx, store.Event{Kind: store.EvUserUpdated, Entity: "user",
				EntityID: l.UserID, Field: "password", NewValue: "reset"})
		}
		if err := validateUsername(username); err != nil {
			return err
		}
		u := store.User{Username: strings.TrimSpace(username), PasswordHash: hash, CreatedAt: t.now}
		id, err := store.CreateUser(ctx, t.Tx, u)
		if err != nil {
			return err
		}
		u.ID = id
		out = publicUser(u)
		return t.log(ctx, store.Event{Kind: store.EvUserCreated, Entity: "user",
			EntityID: id, NewValue: u.Username})
	})
	return out, err
}

func (s *Service) ListUsers(ctx context.Context, a Actor, afterID int64, limit int) ([]*User, error) {
	if err := requireUser(a); err != nil {
		return nil, err
	}
	users, err := store.ListUsers(ctx, s.DB.R, afterID, clampLimit(limit))
	if err != nil {
		return nil, mapErr(err)
	}
	badges, err := store.MembershipBadges(ctx, s.DB.R)
	if err != nil {
		return nil, mapErr(err)
	}
	seen, err := store.LastSeen(ctx, s.DB.R)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]*User, 0, len(users))
	for _, u := range users {
		p := publicUser(u)
		p.Teams = badges[u.ID]
		p.LastSeenAt = seen[u.ID]
		out = append(out, p)
	}
	return list(out, nil)
}

// maxAvatar bounds the picture a profile carries. It is stored inline on the
// user row and ships with every user list, so it is a square the client has
// already scaled down — not an upload.
//
// The client scales to 512px and encodes as WebP, which lands in the tens of
// kilobytes; the cap is the headroom over that, not a target to fill.
const maxAvatar = 256 * 1024

var avatarPrefixes = []string{
	"data:image/png;base64,", "data:image/jpeg;base64,", "data:image/webp;base64,",
}

// validateAvatar accepts an inline image and nothing else.
//
// A remote URL would make every screen a user appears on fetch from a third
// party, and any other data: scheme is a script the client would be asked to
// render as a picture.
func validateAvatar(avatar string) error {
	if avatar == "" {
		return nil
	}
	if len(avatar) > maxAvatar {
		return invalidField("avatar", "a picture is at most %dKB once encoded", maxAvatar/1024)
	}
	for _, prefix := range avatarPrefixes {
		if body, ok := strings.CutPrefix(avatar, prefix); ok {
			if _, err := base64.StdEncoding.DecodeString(body); err != nil {
				return invalidField("avatar", "that picture is not readable")
			}
			return nil
		}
	}
	return invalidField("avatar", "a picture is an inline PNG, JPEG or WebP")
}

// maxBio bounds the prose a profile carries. It is a paragraph about a person,
// not a document: long enough to say how to reach them, short enough that a
// profile page is still a profile page.
const maxBio = 1000

func (s *Service) UpdateProfile(ctx context.Context, a Actor, email, avatar, bio string) error {
	if err := requireUser(a); err != nil {
		return err
	}
	if err := validateAvatar(avatar); err != nil {
		return err
	}
	bio = strings.TrimSpace(bio)
	if len([]rune(bio)) > maxBio {
		return invalidField("bio", "a bio is at most %d characters", maxBio)
	}
	return s.write(ctx, a, func(t *wtx) error {
		if err := store.SetProfile(ctx, t.Tx, a.UserID, strings.TrimSpace(email), avatar, bio); err != nil {
			return err
		}
		return t.log(ctx, store.Event{Kind: store.EvUserUpdated, Entity: "user",
			EntityID: a.UserID, Field: "profile"})
	})
}

func (s *Service) SetUserAdmin(ctx context.Context, a Actor, userID int64, isAdmin bool) error {
	if err := RequireInstanceAdmin(a); err != nil {
		return err
	}
	if userID == a.UserID && !isAdmin {
		// An instance with no administrator cannot create one: there is no
		// open registration and no mailed link to fall back on.
		return errf(CodeInvalid, "you cannot remove your own administrator role")
	}
	return s.write(ctx, a, func(t *wtx) error {
		if err := store.SetAdmin(ctx, t.Tx, userID, isAdmin); err != nil {
			return err
		}
		return t.log(ctx, store.Event{Kind: store.EvUserUpdated, Entity: "user",
			EntityID: userID, Field: "is_admin", NewValue: strconv.FormatBool(isAdmin)})
	})
}

// Agent tokens (SPEC.md §14.2). A user issues them to themselves; an instance
// administrator may revoke any of them. Neither operation is exposed over MCP:
// an agent that can mint a credential outlives the one it was given (§14.3).

type IssuedAgentToken struct {
	Token string           `json:"token"`
	Meta  store.AgentToken `json:"meta"`
}

func (s *Service) IssueAgentToken(ctx context.Context, a Actor, name string) (*IssuedAgentToken, error) {
	if err := requireUser(a); err != nil {
		return nil, err
	}
	if a.AgentTokenID != 0 {
		return nil, errForbidden
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, invalidField("name", "give the token a name so you can recognise it later")
	}
	var out *IssuedAgentToken
	err := s.write(ctx, a, func(t *wtx) error {
		token, hash, err := auth.NewToken()
		if err != nil {
			return err
		}
		id, err := store.CreateAgentToken(ctx, t.Tx, a.UserID, name, hash, t.now)
		if err != nil {
			return err
		}
		out = &IssuedAgentToken{Token: token, Meta: store.AgentToken{
			ID: id, UserID: a.UserID, Name: name, CreatedAt: t.now}}
		return t.log(ctx, store.Event{Kind: store.EvInstance, Entity: "instance",
			Field: "agent_token_issued", NewValue: name})
	})
	return out, err
}

func (s *Service) ListAgentTokens(ctx context.Context, a Actor, userID int64) ([]store.AgentToken, error) {
	if err := requireUser(a); err != nil {
		return nil, err
	}
	// userID 0 lists every token in the instance, which is an administrator's
	// view; anything else must be the caller's own.
	if (userID == 0 || userID != a.UserID) && !a.IsAdmin {
		return nil, errForbidden
	}
	toks, err := store.ListAgentTokens(ctx, s.DB.R, userID)
	return list(toks, mapErr(err))
}

func (s *Service) RevokeAgentToken(ctx context.Context, a Actor, id int64) error {
	if err := requireUser(a); err != nil {
		return err
	}
	if a.AgentTokenID != 0 {
		return errForbidden
	}
	tok, err := store.AgentTokenByID(ctx, s.DB.R, id)
	if err != nil {
		return mapErr(err)
	}
	if tok.UserID != a.UserID && !a.IsAdmin {
		return errForbidden
	}
	return s.write(ctx, a, func(t *wtx) error {
		if err := store.RevokeAgentToken(ctx, t.Tx, id, t.now); err != nil {
			return err
		}
		return t.log(ctx, store.Event{Kind: store.EvInstance, Entity: "instance",
			Field: "agent_token_revoked", OldValue: tok.Name})
	})
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 200 {
		return 200
	}
	return limit
}

// Bootstrap creates the first account on an empty instance and makes it an
// instance administrator.
//
// First run creates the account and a personal team, then opens the client:
// there is no setup wizard (SPEC.md §13.1). It refuses once any account
// exists, so it cannot become a back door.
func (s *Service) Bootstrap(ctx context.Context, username, password string) (*User, error) {
	if err := validateUsername(username); err != nil {
		return nil, err
	}
	if err := validatePassword(password); err != nil {
		return nil, err
	}
	var out *User
	err := s.write(ctx, Actor{}, func(t *wtx) error {
		n, err := store.CountUsers(ctx, t.Tx)
		if err != nil {
			return err
		}
		if n > 0 {
			return errForbidden
		}
		hash, err := auth.HashPassword(password)
		if err != nil {
			return err
		}
		u := store.User{Username: strings.TrimSpace(username), PasswordHash: hash,
			IsAdmin: true, CreatedAt: t.now}
		id, err := store.CreateUser(ctx, t.Tx, u)
		if err != nil {
			return err
		}
		u.ID = id
		out = publicUser(u)
		return t.log(ctx, store.Event{Kind: store.EvUserCreated, Entity: "user",
			EntityID: id, NewValue: u.Username})
	})
	return out, err
}

// NeedsBootstrap reports whether the instance has no accounts yet, which is
// what the sign-in screen asks before offering to create the first one.
func (s *Service) NeedsBootstrap(ctx context.Context) (bool, error) {
	n, err := store.CountUsers(ctx, s.DB.R)
	return n == 0, mapErr(err)
}

// Profile is a user as the rest of the instance sees them: who they are, the
// teams they are in, when they were last here, and what they have been doing.
//
// Every authenticated user may read it. Reading is instance-wide and
// permissions govern writing (SPEC.md §3.2), so there is nothing here to hide
// from a colleague that GET /api/users does not already hand over.
type Profile struct {
	User     *User         `json:"user"`
	Activity []store.Event `json:"activity"`
}

// recentActivity is how much of a person's trail a profile shows. It is a
// glance, not the log: the log is where you go to read further (SPEC.md §10).
const recentActivity = 30

func (s *Service) Profile(ctx context.Context, a Actor, username string) (*Profile, error) {
	if err := requireUser(a); err != nil {
		return nil, err
	}
	u, err := store.UserByUsername(ctx, s.DB.R, username)
	if err != nil {
		return nil, mapErr(err)
	}
	out := publicUser(u)

	badges, err := store.MembershipBadges(ctx, s.DB.R)
	if err != nil {
		return nil, mapErr(err)
	}
	out.Teams = badges[u.ID]

	seen, err := store.LastSeen(ctx, s.DB.R)
	if err != nil {
		return nil, mapErr(err)
	}
	out.LastSeenAt = seen[u.ID]

	// A team-scoped event is one every authenticated user can already read.
	// An instance-level event carries no team and belongs to the global log,
	// which is an administrator's to read — so a profile only shows those to
	// an administrator (SPEC.md §10).
	f := store.EventFilter{ActorID: u.ID, InAnyTeam: !a.IsAdmin}
	events, err := store.ListEvents(ctx, s.DB.R, f, 0, recentActivity)
	if err != nil {
		return nil, mapErr(err)
	}
	if events == nil {
		events = []store.Event{}
	}
	return &Profile{User: out, Activity: events}, nil
}

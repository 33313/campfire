package core

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/33313/campfire/internal/auth"
	"github.com/33313/campfire/internal/store"
)

// Sign-in throttling. The counter is a database row, not a map in the process
// (SPEC.md §13.2 rule 5), so a restart does not clear a lockout.
const (
	signInAttempts = 10
	signInWindow   = int64(15 * time.Minute / time.Microsecond)
)

// SignInResult reports either a session or that a second factor is owed.
type SignInResult struct {
	Token string `json:"token,omitempty"`
	// TOTPRequired is set when the password was right but the account carries
	// a second factor. The client asks for the code and calls SignIn again
	// with it; no partial session is issued in between.
	TOTPRequired bool  `json:"totpRequired,omitempty"`
	User         *User `json:"user,omitempty"`
}

// User is the account as any interface renders it.
type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email,omitempty"`
	Avatar   string `json:"avatar,omitempty"`
	Bio      string `json:"bio,omitempty"`
	IsAdmin  bool   `json:"isAdmin"`
	HasTOTP  bool   `json:"hasTotp"`
	// Teams and LastSeenAt are filled by ListUsers for the administration
	// screen and are absent everywhere else.
	Teams      []store.TeamBadge `json:"teams,omitempty"`
	LastSeenAt int64             `json:"lastSeenAt,omitempty"`
}

func publicUser(u store.User) *User {
	return &User{ID: u.ID, Username: u.Username, Email: u.Email, Avatar: u.Avatar,
		Bio: u.Bio, IsAdmin: u.IsAdmin, HasTOTP: u.TOTPSecret != ""}
}

// SignIn verifies a password and, if the account has one, a TOTP code.
//
// The attempt is counted in its own transaction, before the verification, so
// that a failed sign-in still spends an attempt: counting it inside the same
// transaction as the check would roll the counter back with the rejection and
// leave the throttle doing nothing.
func (s *Service) SignIn(ctx context.Context, username, password, code string) (SignInResult, error) {
	now := s.Clock()
	bucket := "signin:" + store.Fold(username)
	var over bool
	if err := s.DB.Write(ctx, func(tx *sql.Tx) error {
		var err error
		over, err = store.RateLimit(ctx, tx, bucket, signInAttempts, now, signInWindow)
		return err
	}); err != nil {
		return SignInResult{}, err
	}
	if over {
		return SignInResult{}, errf(CodeRateLimited, "too many attempts; wait a few minutes")
	}

	u, err := store.UserByUsername(ctx, s.DB.R, username)
	if errors.Is(err, store.ErrNotFound) {
		// Spend the same work as a real verification would, so that the
		// response time does not report whether the account exists.
		auth.VerifyPassword(decoyHash, password)
		return SignInResult{}, errf(CodeUnauthenticated, "wrong username or password")
	}
	if err != nil {
		return SignInResult{}, mapErr(err)
	}
	ok, err := auth.VerifyPassword(u.PasswordHash, password)
	if err != nil || !ok {
		return SignInResult{}, errf(CodeUnauthenticated, "wrong username or password")
	}
	if u.TOTPSecret != "" {
		if strings.TrimSpace(code) == "" {
			return SignInResult{TOTPRequired: true}, nil
		}
		if !auth.VerifyTOTP(u.TOTPSecret, code, time.UnixMicro(now)) {
			return SignInResult{}, errf(CodeUnauthenticated, "that code is not right")
		}
	}

	var out SignInResult
	err = s.write(ctx, Actor{}, func(t *wtx) error {
		token, hash, err := auth.NewToken()
		if err != nil {
			return err
		}
		expires := t.now + int64(sessionLifetime/time.Microsecond)
		if _, err := store.CreateSession(ctx, t.Tx, u.ID, hash, t.now, expires); err != nil {
			return err
		}
		// Lazy cleanup on a write that was happening anyway (rule 6).
		if err := store.ReapExpiredSessions(ctx, t.Tx, t.now); err != nil {
			return err
		}
		if err := store.ClearRateLimit(ctx, t.Tx, bucket); err != nil {
			return err
		}
		out = SignInResult{Token: token, User: publicUser(u)}
		return nil
	})
	return out, err
}

// decoyHash is a real argon2id hash of a value nobody knows, verified against
// when the username does not exist so that both paths cost the same.
var decoyHash = func() string {
	h, err := auth.HashPassword("campfire-decoy")
	if err != nil {
		panic(err)
	}
	return h
}()

// ResolveSession turns a session cookie into an actor, refreshing the
// session's sliding expiry.
func (s *Service) ResolveSession(ctx context.Context, token string) (Actor, error) {
	if token == "" {
		return Actor{}, nil
	}
	now := s.Clock()
	sess, err := store.SessionByToken(ctx, s.DB.R, auth.HashToken(token), now)
	if errors.Is(err, store.ErrNotFound) {
		return Actor{}, nil
	}
	if err != nil {
		return Actor{}, err
	}
	u, err := store.UserByID(ctx, s.DB.R, sess.UserID)
	if err != nil {
		return Actor{}, err
	}
	// Refresh at most once an hour: a write per request would put the single
	// writer in the path of every read.
	if now-sess.LastUsedAt > int64(time.Hour/time.Microsecond) {
		expires := now + int64(sessionLifetime/time.Microsecond)
		_ = s.DB.Write(ctx, func(tx *sql.Tx) error {
			return store.TouchSession(ctx, tx, sess.ID, now, expires)
		})
	}
	return Actor{UserID: u.ID, Username: u.Username, IsAdmin: u.IsAdmin, SessionID: sess.ID}, nil
}

// ResolveAgentToken turns a bearer token into an actor.
//
// The token carries no permissions of its own: it names a user, and this call
// resolves that user's current permissions, never the ones they had when the
// token was issued (SPEC.md §14.2).
func (s *Service) ResolveAgentToken(ctx context.Context, token string) (Actor, error) {
	if token == "" {
		return Actor{}, nil
	}
	tok, err := store.AgentTokenByHash(ctx, s.DB.R, auth.HashToken(token))
	if errors.Is(err, store.ErrNotFound) {
		return Actor{}, nil
	}
	if err != nil {
		return Actor{}, err
	}
	u, err := store.UserByID(ctx, s.DB.R, tok.UserID)
	if err != nil {
		return Actor{}, err
	}
	now := s.Clock()
	if now-tok.LastUsedAt > int64(time.Hour/time.Microsecond) {
		_ = s.DB.Write(ctx, func(tx *sql.Tx) error {
			return store.TouchAgentToken(ctx, tx, tok.ID, now)
		})
	}
	return Actor{UserID: u.ID, Username: u.Username, IsAdmin: u.IsAdmin, AgentTokenID: tok.ID}, nil
}

func (s *Service) SignOut(ctx context.Context, a Actor) error {
	if a.SessionID == 0 {
		return nil
	}
	return s.write(ctx, a, func(t *wtx) error {
		return store.DeleteSession(ctx, t.Tx, a.SessionID)
	})
}

// ChangePassword requires the current one, and drops every other session so
// that a stolen cookie does not outlive the password it was issued under.
func (s *Service) ChangePassword(ctx context.Context, a Actor, current, next string) error {
	if err := requireUser(a); err != nil {
		return err
	}
	if err := validatePassword(next); err != nil {
		return err
	}
	return s.write(ctx, a, func(t *wtx) error {
		u, err := store.UserByID(ctx, t.Tx, a.UserID)
		if err != nil {
			return err
		}
		ok, err := auth.VerifyPassword(u.PasswordHash, current)
		if err != nil || !ok {
			return errf(CodeUnauthenticated, "your current password is not right")
		}
		hash, err := auth.HashPassword(next)
		if err != nil {
			return err
		}
		if err := store.SetPassword(ctx, t.Tx, a.UserID, hash); err != nil {
			return err
		}
		if err := store.DeleteOtherUserSessions(ctx, t.Tx, a.UserID, a.SessionID); err != nil {
			return err
		}
		return t.log(ctx, store.Event{Kind: store.EvUserUpdated, Entity: "user",
			EntityID: a.UserID, Field: "password"})
	})
}

// SetupTOTP returns a secret and its otpauth URI without enabling anything.
// The second factor is only stored once a code proves the app has it.
func (s *Service) SetupTOTP(ctx context.Context, a Actor) (secret, uri string, err error) {
	if err := requireUser(a); err != nil {
		return "", "", err
	}
	secret, err = auth.NewTOTPSecret()
	if err != nil {
		return "", "", err
	}
	return secret, auth.TOTPURI("Campfire", a.Username, secret), nil
}

func (s *Service) EnableTOTP(ctx context.Context, a Actor, secret, code string) error {
	if err := requireUser(a); err != nil {
		return err
	}
	if !auth.VerifyTOTP(secret, code, time.UnixMicro(s.Clock())) {
		return errf(CodeInvalid, "that code is not right")
	}
	return s.write(ctx, a, func(t *wtx) error {
		if err := store.SetTOTPSecret(ctx, t.Tx, a.UserID, secret); err != nil {
			return err
		}
		return t.log(ctx, store.Event{Kind: store.EvUserUpdated, Entity: "user",
			EntityID: a.UserID, Field: "totp", NewValue: "enabled"})
	})
}

// DisableTOTP requires the current password, so that an unattended session
// cannot quietly remove the second factor.
func (s *Service) DisableTOTP(ctx context.Context, a Actor, password string) error {
	if err := requireUser(a); err != nil {
		return err
	}
	return s.write(ctx, a, func(t *wtx) error {
		u, err := store.UserByID(ctx, t.Tx, a.UserID)
		if err != nil {
			return err
		}
		ok, err := auth.VerifyPassword(u.PasswordHash, password)
		if err != nil || !ok {
			return errf(CodeUnauthenticated, "your password is not right")
		}
		if err := store.SetTOTPSecret(ctx, t.Tx, a.UserID, ""); err != nil {
			return err
		}
		return t.log(ctx, store.Event{Kind: store.EvUserUpdated, Entity: "user",
			EntityID: a.UserID, Field: "totp", OldValue: "enabled"})
	})
}

func validatePassword(p string) error {
	if len([]rune(p)) < 10 {
		return invalidField("password", "a password needs at least 10 characters")
	}
	if len(p) > 1024 {
		return invalidField("password", "that password is too long")
	}
	return nil
}

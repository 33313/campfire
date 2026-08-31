package core

import (
	"strings"
	"testing"

	"github.com/campfire/campfire/internal/store"
)

func TestBootstrapOnlyWorksOnAnEmptyInstance(t *testing.T) {
	f := newFixture(t) // already bootstrapped
	if _, err := f.Bootstrap(f.ctx, "second", "correct horse battery"); err == nil {
		t.Fatal("a second bootstrap was allowed")
	} else {
		requireCode(t, err, CodeForbidden)
	}
}

func TestSignInIssuesASessionAndResolvesIt(t *testing.T) {
	f := newFixture(t)
	res, err := f.SignIn(f.ctx, "root", "correct horse battery", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Token == "" {
		t.Fatal("no session token")
	}
	a, err := f.ResolveSession(f.ctx, res.Token)
	if err != nil {
		t.Fatal(err)
	}
	if a.UserID != f.admin.UserID || !a.IsAdmin {
		t.Fatalf("resolved actor %+v", a)
	}
	if a.AgentTokenID != 0 {
		t.Fatal("a browser session must not look like an agent call")
	}
}

func TestSignInIsCaseInsensitiveOnUsername(t *testing.T) {
	f := newFixture(t)
	if _, err := f.SignIn(f.ctx, "ROOT", "correct horse battery", ""); err != nil {
		t.Fatalf("case-folded username rejected: %v", err)
	}
}

func TestSignInRejectsAWrongPassword(t *testing.T) {
	f := newFixture(t)
	_, err := f.SignIn(f.ctx, "root", "wrong password here", "")
	requireCode(t, err, CodeUnauthenticated)
}

// The rate-limit counter is a row, so it survives anything that would clear a
// process-local map (SPEC.md §13.2 rule 5).
func TestSignInThrottlesRepeatedFailures(t *testing.T) {
	f := newFixture(t)
	var last error
	for i := 0; i < signInAttempts+2; i++ {
		_, last = f.SignIn(f.ctx, "root", "wrong password here", "")
	}
	requireCode(t, last, CodeRateLimited)
	// The correct password is refused too: the limit is on the account, not
	// on whether the guess happened to be right.
	_, err := f.SignIn(f.ctx, "root", "correct horse battery", "")
	requireCode(t, err, CodeRateLimited)
}

func TestSignOutInvalidatesTheSession(t *testing.T) {
	f := newFixture(t)
	res, _ := f.SignIn(f.ctx, "root", "correct horse battery", "")
	a, _ := f.ResolveSession(f.ctx, res.Token)
	if err := f.SignOut(f.ctx, a); err != nil {
		t.Fatal(err)
	}
	after, err := f.ResolveSession(f.ctx, res.Token)
	if err != nil {
		t.Fatal(err)
	}
	if after.UserID != 0 {
		t.Fatal("the session still resolves after sign-out")
	}
}

func TestChangePasswordEndsEveryOtherSession(t *testing.T) {
	f := newFixture(t)
	first, _ := f.SignIn(f.ctx, "root", "correct horse battery", "")
	second, _ := f.SignIn(f.ctx, "root", "correct horse battery", "")
	a, _ := f.ResolveSession(f.ctx, first.Token)
	if err := f.ChangePassword(f.ctx, a, "correct horse battery", "another good passphrase"); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.ResolveSession(f.ctx, second.Token); got.UserID != 0 {
		t.Fatal("another session outlived the password change")
	}
	// The session doing the changing survives it: signing yourself out of the
	// screen you are standing on is not what "every other session" means.
	if got, _ := f.ResolveSession(f.ctx, first.Token); got.UserID == 0 {
		t.Fatal("the caller was signed out of its own session")
	}
	if _, err := f.SignIn(f.ctx, "root", "another good passphrase", ""); err != nil {
		t.Fatalf("the new password does not work: %v", err)
	}
}

// SPEC.md §14.2: a token names a user and carries no permissions of its own,
// and revocation takes effect on the next call because nothing is held in the
// process.
func TestAgentTokenResolvesToItsUserAndRevokesImmediately(t *testing.T) {
	f := newFixture(t)
	issued, err := f.IssueAgentToken(f.ctx, f.admin, "laptop")
	if err != nil {
		t.Fatal(err)
	}
	a, err := f.ResolveAgentToken(f.ctx, issued.Token)
	if err != nil {
		t.Fatal(err)
	}
	if a.UserID != f.admin.UserID {
		t.Fatalf("token resolved to user %d, want %d", a.UserID, f.admin.UserID)
	}
	if a.AgentTokenID != issued.Meta.ID {
		t.Fatal("the actor does not carry the token it arrived on")
	}
	if !a.IsAdmin {
		t.Fatal("the actor lost the user's own permissions")
	}
	if err := f.RevokeAgentToken(f.ctx, f.admin, issued.Meta.ID); err != nil {
		t.Fatal(err)
	}
	after, _ := f.ResolveAgentToken(f.ctx, issued.Token)
	if after.UserID != 0 {
		t.Fatal("a revoked token still authenticates")
	}
}

// An agent that can mint a credential outlives the one it was given
// (SPEC.md §14.3), so issuing and revoking are refused on an agent call.
func TestAgentCannotIssueOrRevokeTokens(t *testing.T) {
	f := newFixture(t)
	issued, _ := f.IssueAgentToken(f.ctx, f.admin, "laptop")
	agent, _ := f.ResolveAgentToken(f.ctx, issued.Token)

	_, err := f.IssueAgentToken(f.ctx, agent, "another")
	requireCode(t, err, CodeForbidden)
	requireCode(t, f.RevokeAgentToken(f.ctx, agent, issued.Meta.ID), CodeForbidden)
}

func TestPasswordResetLinkIsSingleUse(t *testing.T) {
	f := newFixture(t)
	marta := f.user("marta")
	link, err := f.CreatePasswordReset(f.ctx, f.admin, marta.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if link.Username != "marta" {
		t.Fatalf("reset link names %q", link.Username)
	}
	if _, err := f.RedeemInvitation(f.ctx, link.Token, "", "a whole new passphrase"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.RedeemInvitation(f.ctx, link.Token, "", "yet another passphrase"); err == nil {
		t.Fatal("a reset link was redeemed twice")
	}
	if _, err := f.SignIn(f.ctx, "marta", "a whole new passphrase", ""); err != nil {
		t.Fatalf("the reset password does not work: %v", err)
	}
}

func TestInvitationCreatesAnAccount(t *testing.T) {
	f := newFixture(t)
	link, err := f.CreateInvitation(f.ctx, f.admin)
	if err != nil {
		t.Fatal(err)
	}
	u, err := f.RedeemInvitation(f.ctx, link.Token, "jo", "a perfectly fine passphrase")
	if err != nil {
		t.Fatal(err)
	}
	if u.IsAdmin {
		t.Fatal("an invited account arrived as an instance administrator")
	}
	if _, err := f.SignIn(f.ctx, "jo", "a perfectly fine passphrase", ""); err != nil {
		t.Fatal(err)
	}
}

func TestOnlyAnAdministratorCreatesAccounts(t *testing.T) {
	f := newFixture(t)
	marta := f.user("marta")
	_, err := f.CreateUser(f.ctx, marta, "intruder", "a perfectly fine passphrase", "", false)
	requireCode(t, err, CodeForbidden)
	_, err = f.CreateInvitation(f.ctx, marta)
	requireCode(t, err, CodeForbidden)
}

func TestTOTPGatesSignIn(t *testing.T) {
	f := newFixture(t)
	secret, _, err := f.SetupTOTP(f.ctx, f.admin)
	if err != nil {
		t.Fatal(err)
	}
	code := totpNow(t, secret, f)
	if err := f.EnableTOTP(f.ctx, f.admin, secret, code); err != nil {
		t.Fatal(err)
	}
	res, err := f.SignIn(f.ctx, "root", "correct horse battery", "")
	if err != nil {
		t.Fatal(err)
	}
	if !res.TOTPRequired || res.Token != "" {
		t.Fatal("a session was issued without the second factor")
	}
	res, err = f.SignIn(f.ctx, "root", "correct horse battery", totpNow(t, secret, f))
	if err != nil {
		t.Fatal(err)
	}
	if res.Token == "" {
		t.Fatal("the correct code did not produce a session")
	}
}

func TestEveryAccountEventReachesTheLog(t *testing.T) {
	f := newFixture(t)
	f.user("marta")
	var found bool
	events, err := store.ListEvents(f.ctx, f.DB.R, store.EventFilter{}, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range events {
		if e.Kind == store.EvUserCreated && e.NewValue == "marta" {
			found = true
		}
	}
	if !found {
		t.Fatal("creating an account wrote no event")
	}
}

// A profile picture is stored inline and drawn by every screen a user appears
// on, so what may be stored is exactly an inline image (DEV-3).
func TestProfilePictureIsAnInlineImage(t *testing.T) {
	f := newFixture(t)
	// A 1x1 PNG.
	const png = "data:image/png;base64," +
		"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
	if err := f.UpdateProfile(f.ctx, f.admin, "root@example.invalid", png); err != nil {
		t.Fatal(err)
	}
	users, err := f.ListUsers(f.ctx, f.admin, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if users[0].Avatar != png {
		t.Fatalf("avatar %q", users[0].Avatar)
	}

	for _, bad := range []string{
		"https://example.invalid/me.png",              // would make every screen fetch elsewhere
		"data:text/html;base64,PHNjcmlwdD4=",          // a script asked to be drawn as a picture
		"data:image/png;base64,not base64 at all !!!", // unreadable
		"data:image/png;base64," + strings.Repeat("A", maxAvatar),
	} {
		if err := f.UpdateProfile(f.ctx, f.admin, "", bad); err == nil {
			t.Fatalf("%.40q was accepted", bad)
		} else {
			requireCode(t, err, CodeInvalid)
		}
	}

	// Clearing it is how a picture is removed.
	if err := f.UpdateProfile(f.ctx, f.admin, "", ""); err != nil {
		t.Fatal(err)
	}
}

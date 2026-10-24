package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/33313/campfire/internal/core"
	"github.com/33313/campfire/internal/forge"
	"github.com/33313/campfire/internal/store"
)

// Forges (DEV-114): GitHub, Forgejo and GitLab, read with an access token
// and heard from by a webhook.

// syncWindow is how far back a sync reads pull requests.
const syncWindow = 90 * 24 * time.Hour

// A forge is either polled, as often as its administrator chose, or told of
// changes by its webhook. GitHub takes no webhook on a personal account, nor
// GitLab, on its free tier, on a group: polling is what reaches those, and
// any forge that cannot reach this instance.

// PollForges looks every minute for polled forges that are due, until ctx
// ends.
func (s *Server) PollForges(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.pollOnce(ctx, time.Now())
		}
	}
}

// pollOnce reads each polled forge that is due over the whole window. That
// costs no more requests than reading only what changed, one per repository
// either way, and re-applying a pull request that did not change is a no-op.
// How it ended shows on the administration screen.
func (s *Server) pollOnce(ctx context.Context, now time.Time) {
	forges, err := s.svc.LoadForges(ctx)
	if err != nil {
		return
	}
	for _, f := range forges {
		due := f.SyncedAt == 0 || now.Sub(time.UnixMicro(f.SyncedAt)) >= time.Duration(f.PollMinutes)*time.Minute
		if f.Mode == "poll" && due {
			s.svc.MarkSynced(ctx, f.ID, s.sync(ctx, f, now.Add(-syncWindow)))
		}
	}
}

func toForge(f store.Forge) forge.Forge {
	return forge.Forge{Kind: f.Kind, URL: f.URL, Token: f.Token, Secret: f.WebhookSecret}
}

// checkToken asks the forge whose the token is.
func (s *Server) checkToken(ctx context.Context, f store.Forge) (string, error) {
	return s.Forges.Account(ctx, toForge(f))
}

func (s *Server) listForges(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	return s.svc.Forges(r.Context(), a)
}

func (s *Server) addForge(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	var in struct {
		Kind        string `json:"kind"`
		URL         string `json:"url"`
		Token       string `json:"token"`
		Mode        string `json:"mode"`
		PollMinutes int64  `json:"pollMinutes"`
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	f, err := s.svc.AddForge(r.Context(), a, store.Forge{Kind: in.Kind, Mode: in.Mode, URL: in.URL,
		Token: in.Token, PollMinutes: in.PollMinutes}, s.checkToken)
	if err != nil {
		return nil, err
	}
	return withSecret{f, f.WebhookSecret}, nil
}

// withSecret is a forge with its webhook secret: sent once, when it is made.
// A polled forge has none.
type withSecret struct {
	store.Forge
	WebhookSecret string `json:"webhookSecret,omitempty"`
}

func (s *Server) rotateForgeSecret(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	secret, err := s.svc.RotateForgeSecret(r.Context(), a, id)
	if err != nil {
		return nil, err
	}
	return map[string]string{"webhookSecret": secret}, nil
}

// setForgeToken replaces a token that expired, keeping the webhook the forge
// already delivers to.
func (s *Server) setForgeToken(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	var in struct {
		Token string `json:"token"`
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	if err := s.svc.SetForgeToken(r.Context(), a, id, in.Token, s.checkToken); err != nil {
		return nil, err
	}
	return s.svc.Forge(r.Context(), a, id)
}

func (s *Server) deleteForge(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	return nil, s.svc.DeleteForge(r.Context(), a, id)
}

// syncForge reads back the pull requests of the last syncWindow: what a forge
// that cannot reach the instance has to be asked for, and what a lost
// delivery left behind. It runs to the end even if the browser stops
// waiting, and records how it ended.
func (s *Server) syncForge(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	f, err := s.svc.Forge(r.Context(), a, id)
	if err != nil {
		return nil, err
	}
	ctx := context.WithoutCancel(r.Context())
	if err := s.svc.MarkSynced(ctx, id, s.sync(ctx, f, time.Now().Add(-syncWindow))); err != nil {
		return nil, err
	}
	return s.svc.Forge(ctx, a, id)
}

func (s *Server) sync(ctx context.Context, f store.Forge, since time.Time) error {
	prs, err := s.Forges.PullRequests(ctx, toForge(f), since)
	if err != nil {
		return err
	}
	for _, pr := range prs {
		if err := s.svc.ApplyPullRequest(ctx, f.ID, storePR(pr), forge.Identifiers(pr.Title)); err != nil {
			return err
		}
	}
	return nil
}

func storePR(p forge.PullRequest) store.PullRequest {
	return store.PullRequest{Remote: p.Remote, Repo: p.Repo, Number: p.Number, Title: p.Title, URL: p.URL,
		State: p.State, Author: p.Author, UpdatedAt: p.UpdatedAt.UnixMicro()}
}

// forgeWebhook takes a forge's deliveries. It is the one route that trusts a
// secret instead of a session: each forge's own, made when it was connected.
func (s *Server) forgeWebhook(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 5<<20))
	if err != nil {
		return nil, err
	}
	f, err := s.svc.LoadForge(r.Context(), id)
	if err != nil {
		return nil, err
	}
	pr, err := s.Forges.Delivery(r.Context(), toForge(f), r.Header, body)
	if errors.Is(err, forge.ErrSignature) {
		return nil, &core.Error{Code: core.CodeUnauthenticated, Message: err.Error()}
	}
	if err != nil {
		return nil, &core.Error{Code: core.CodeInvalid, Message: err.Error()}
	}
	if pr == nil {
		return nil, nil
	}
	return nil, s.svc.ApplyPullRequest(r.Context(), f.ID, storePR(*pr), forge.Identifiers(pr.Title))
}

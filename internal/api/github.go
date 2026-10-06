package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/33313/campfire/internal/core"
	"github.com/33313/campfire/internal/github"
	"github.com/33313/campfire/internal/store"
)

// The GitHub integration (DEV-13). The round trips through GitHub — creating
// the app, installing it — come back here as top-level navigations, so they
// answer with a redirect to the administration screen rather than JSON.

// syncWindow is how far back a sync reads pull requests.
const syncWindow = 90 * 24 * time.Hour

const gitHubScreen = "/admin?tab=github"

// redirect is a body that sends the browser on instead.
type redirect string

// back returns to the administration screen, saying what went wrong if
// something did.
func back(err error) (any, error) {
	if err == nil {
		return redirect(gitHubScreen), nil
	}
	msg := err.Error()
	var e *core.Error
	if errors.As(err, &e) {
		msg = e.Message
	}
	return redirect(gitHubScreen + "&error=" + url.QueryEscape(msg)), nil
}

// base is this instance's address as GitHub should use it: the host the
// browser reached, over HTTPS when it came through a proxy that said so.
func base(r *http.Request) string {
	if secure(r) {
		return "https://" + r.Host
	}
	return "http://" + r.Host
}

func gitHubApp(a store.GitHubApp) github.App {
	return github.App{ID: a.AppID, Slug: a.Slug, HTMLURL: a.HTMLURL, Owner: a.Owner,
		PEM: a.PrivateKey, WebhookSecret: a.WebhookSecret}
}

func (s *Server) getGitHub(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	gh, err := s.svc.GitHub(r.Context(), a)
	if err != nil || gh.App == nil {
		return gh, err
	}
	// The installations not linked yet, as GitHub lists them. One may be
	// installed by an organisation's owner who is not this instance's
	// administrator, and so never come back through setup.
	out := struct {
		*core.GitHub
		Available []github.Installation `json:"available"`
		Error     string                `json:"error,omitempty"`
		Webhook   string                `json:"webhook"`
	}{GitHub: gh, Available: []github.Installation{}, Webhook: base(r) + "/api/github/webhook"}
	app, err := s.svc.LoadGitHubApp(r.Context())
	if err != nil {
		return nil, err
	}
	all, err := s.GitHub.Installations(r.Context(), gitHubApp(app))
	if err != nil {
		out.Error = err.Error()
	}
	linked := map[int64]bool{}
	for _, i := range gh.Installations {
		linked[i.ID] = true
	}
	for _, i := range all {
		if !linked[i.ID] {
			out.Available = append(out.Available, i)
		}
	}
	return out, nil
}

// createGitHubApp answers with the form the browser posts to GitHub: where,
// and the manifest. GitHub shows it to the administrator to confirm.
func (s *Server) createGitHubApp(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	var in struct {
		Organization string `json:"organization"`
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	state, err := s.svc.GitHubState(r.Context(), a)
	if err != nil {
		return nil, err
	}
	// An app's name is unique across GitHub; the administrator can change it
	// on the form if this one is taken.
	name := "Campfire " + strings.Split(r.Host, ":")[0]
	if len(name) > 34 {
		name = name[:34]
	}
	manifest, _ := json.Marshal(github.Manifest(base(r), name))
	return map[string]string{
		"action":   s.GitHub.NewAppURL(strings.TrimSpace(in.Organization), state),
		"manifest": string(manifest),
	}, nil
}

// gitHubManifest is where GitHub sends the administrator back once the app
// exists, with a code to exchange for its credentials.
func (s *Server) gitHubManifest(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	q := r.URL.Query()
	if err := s.svc.SpendGitHubState(r.Context(), a, q.Get("state")); err != nil {
		return back(err)
	}
	app, err := s.GitHub.ConvertManifest(r.Context(), q.Get("code"))
	if err != nil {
		return back(err)
	}
	return back(s.svc.SaveGitHubApp(r.Context(), a, store.GitHubApp{AppID: app.ID, Slug: app.Slug,
		HTMLURL: app.HTMLURL, Owner: app.Owner, PrivateKey: app.PEM, WebhookSecret: app.WebhookSecret}))
}

func (s *Server) deleteGitHubApp(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	return nil, s.svc.DeleteGitHubApp(r.Context(), a)
}

// installGitHubApp sends the administrator to GitHub to install the app on
// an account or an organisation, and pick its repositories.
func (s *Server) installGitHubApp(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	app, err := s.svc.LoadGitHubApp(r.Context())
	if err != nil {
		return back(err)
	}
	state, err := s.svc.GitHubState(r.Context(), a)
	if err != nil {
		return back(err)
	}
	return redirect(s.GitHub.InstallURL(app.Slug, state)), nil
}

// gitHubSetup is where GitHub sends whoever installed the app. Arriving with
// a state this administrator was issued links the installation; arriving
// without one — a change of repositories made on GitHub, or a stranger
// installing the public app — links nothing.
func (s *Server) gitHubSetup(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	q := r.URL.Query()
	id, _ := strconv.ParseInt(q.Get("installation_id"), 10, 64)
	if q.Get("state") == "" || id == 0 {
		return back(nil)
	}
	if err := s.svc.SpendGitHubState(r.Context(), a, q.Get("state")); err != nil {
		return back(err)
	}
	return back(s.link(r.Context(), a, id))
}

// link asks GitHub whether the installation is the app's, and for whom,
// before linking it: an id in a URL proves nothing.
func (s *Server) link(ctx context.Context, a core.Actor, id int64) error {
	app, err := s.svc.LoadGitHubApp(ctx)
	if err != nil {
		return err
	}
	inst, err := s.GitHub.Installation(ctx, gitHubApp(app), id)
	if err != nil {
		return err
	}
	return s.svc.LinkInstallation(ctx, a, inst.ID, inst.Account, inst.AccountType)
}

func (s *Server) linkInstallation(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	if err := core.RequireInstanceAdmin(a); err != nil {
		return nil, err
	}
	if err := s.link(r.Context(), a, id); err != nil {
		var e *core.Error
		if errors.As(err, &e) {
			return nil, err
		}
		return nil, &core.Error{Code: core.CodeInvalid, Message: err.Error()}
	}
	return s.svc.Installation(r.Context(), a, id)
}

func (s *Server) unlinkInstallation(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	return nil, s.svc.UnlinkInstallation(r.Context(), a, id)
}

// syncInstallation reads back the pull requests of the last syncWindow:
// what an instance GitHub cannot reach has to ask for, and what a lost
// delivery left behind. It runs to the end even if the browser stops
// waiting, and records how it ended.
func (s *Server) syncInstallation(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	if _, err := s.svc.Installation(r.Context(), a, id); err != nil {
		return nil, err
	}
	app, err := s.svc.LoadGitHubApp(r.Context())
	if err != nil {
		return nil, err
	}
	ctx := context.WithoutCancel(r.Context())
	if err := s.svc.MarkSynced(ctx, id, s.sync(ctx, app, id)); err != nil {
		return nil, err
	}
	return s.svc.Installation(ctx, a, id)
}

func (s *Server) sync(ctx context.Context, app store.GitHubApp, id int64) error {
	prs, err := s.GitHub.PullRequests(ctx, gitHubApp(app), id, time.Now().Add(-syncWindow))
	if err != nil {
		return err
	}
	for _, pr := range prs {
		if err := s.svc.ApplyPullRequest(ctx, storePR(pr), github.Identifiers(pr.Title)); err != nil {
			return err
		}
	}
	return nil
}

func storePR(p github.PullRequest) store.PullRequest {
	return store.PullRequest{ID: p.ID, InstallationID: p.InstallationID, Repo: p.Repo, Number: p.Number,
		Title: p.Title, URL: p.URL, State: p.State, Author: p.Author, UpdatedAt: p.UpdatedAt.UnixMicro()}
}

// gitHubWebhook takes GitHub's deliveries. It is the one route that trusts a
// signature instead of a session: the body is signed with the secret GitHub
// handed over when the app was created.
func (s *Server) gitHubWebhook(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 5<<20))
	if err != nil {
		return nil, err
	}
	app, err := s.svc.LoadGitHubApp(r.Context())
	if err != nil {
		return nil, err
	}
	if !github.Verify(app.WebhookSecret, body, r.Header.Get("X-Hub-Signature-256")) {
		return nil, &core.Error{Code: core.CodeUnauthenticated, Message: "the delivery's signature does not match"}
	}
	e, err := github.ParseEvent(r.Header.Get("X-GitHub-Event"), body)
	if err != nil {
		return nil, &core.Error{Code: core.CodeInvalid, Message: err.Error()}
	}
	switch {
	case e.Removed:
		return nil, s.svc.InstallationRemoved(r.Context(), e.InstallationID)
	case e.PullRequest != nil:
		return nil, s.svc.ApplyPullRequest(r.Context(), storePR(*e.PullRequest), github.Identifiers(e.PullRequest.Title))
	}
	return nil, nil
}

package api

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/33313/campfire/internal/core"
	"github.com/33313/campfire/internal/forge"
	"github.com/33313/campfire/internal/store"
)

func (s *Server) addManualPull(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	var in struct{ URL string }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	return nil, s.svc.AddManualPull(r.Context(), a, id, in.URL, s.lookupManualPull)
}

func (s *Server) removeManualPull(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	pull, err := pathID(r, "pullId")
	if err != nil {
		return nil, err
	}
	return nil, s.svc.RemoveManualPull(r.Context(), a, id, pull)
}

func (s *Server) lookupManualPull(ctx context.Context, u *url.URL) store.PullRequest {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	p := store.PullRequest{URL: u.String(), Repo: u.Hostname(), State: "unknown"}
	var targets []forge.Forge
	configured, _ := s.svc.LoadForges(ctx)
	for _, f := range configured {
		targets = append(targets, toForge(f))
	}
	for _, kind := range []string{forge.GitHub, forge.GitLab, forge.Forgejo} {
		host, _ := forge.Hosted(kind)
		targets = append(targets, forge.Forge{Kind: kind, URL: host})
	}
	for _, f := range targets {
		repo, n, ok := forge.PullTarget(f, u)
		if !ok {
			continue
		}
		p.Repo, p.Number = repo, n
		if found, err := s.Forges.PullRequestByNumber(ctx, f, repo, n); err == nil && found.Number == n {
			switch found.State {
			case "open", "draft", "closed", "merged":
				p.State, p.Title, p.Author, p.UpdatedAt = found.State, found.Title, found.Author, found.UpdatedAt.UnixMicro()
				return p
			}
		}
	}
	return p
}

func (s *Server) PollManualPulls(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.svc.CheckManualPulls(ctx, s.lookupManualPull)
		}
	}
}

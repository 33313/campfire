package core

import (
	"context"
	"errors"
	"time"

	"github.com/33313/campfire/internal/auth"
	"github.com/33313/campfire/internal/store"
)

// The GitHub integration (DEV-13). An instance administrator creates the
// instance's GitHub App and links the accounts and organisations it is
// installed on. From then on a pull request whose title names an item, at
// its start or its end, is listed on that item, and its state with it.
//
// Pull requests are written by nobody: the events carry no actor, the way
// GitHub, not a user, made the change.

// GitHub is what the administration screen shows of the integration: each
// app, and the accounts linked through it.
type GitHub struct {
	Apps []*GitHubApp `json:"apps"`
}

type GitHubApp struct {
	ID      int64  `json:"id"`
	Slug    string `json:"slug"`
	HTMLURL string `json:"htmlUrl"`
	Owner   string `json:"owner"`
	Public  bool   `json:"public"`
	// Webhook is false for an app made where GitHub could not reach: it is
	// read by Sync alone.
	Webhook       bool                       `json:"webhook"`
	Installations []store.GitHubInstallation `json:"installations"`
}

func (s *Service) GitHub(ctx context.Context, a Actor) (*GitHub, error) {
	if err := RequireInstanceAdmin(a); err != nil {
		return nil, err
	}
	apps, err := store.ListGitHubApps(ctx, s.DB.R)
	if err != nil {
		return nil, err
	}
	insts, err := store.ListInstallations(ctx, s.DB.R)
	if err != nil {
		return nil, err
	}
	out := &GitHub{Apps: []*GitHubApp{}}
	for _, app := range apps {
		v := &GitHubApp{ID: app.ID, Slug: app.Slug, HTMLURL: app.HTMLURL, Owner: app.Owner,
			Public: app.Public, Webhook: app.WebhookSecret != "", Installations: []store.GitHubInstallation{}}
		for _, i := range insts {
			if i.AppID == app.ID {
				v.Installations = append(v.Installations, i)
			}
		}
		out.Apps = append(out.Apps, v)
	}
	return out, nil
}

// GitHubApps are the apps with their secrets, for talking to GitHub and
// checking its deliveries. They are never sent to a client.
func (s *Service) GitHubApps(ctx context.Context) ([]store.GitHubApp, error) {
	apps, err := store.ListGitHubApps(ctx, s.DB.R)
	return apps, mapErr(err)
}

func (s *Service) LoadGitHubApp(ctx context.Context, id int64) (store.GitHubApp, error) {
	app, err := store.LoadGitHubApp(ctx, s.DB.R, id)
	return app, mapErr(err)
}

const gitHubStateLifetime = time.Hour

// GitHubState issues the state a round trip through GitHub carries and
// brings back, so that what comes back is known to answer a request this
// administrator made, here. It is kept as a one-time link: in the database,
// spent once.
func (s *Service) GitHubState(ctx context.Context, a Actor) (string, error) {
	if err := RequireInstanceAdmin(a); err != nil {
		return "", err
	}
	var out string
	err := s.write(ctx, a, func(t *wtx) error {
		token, hash, err := auth.NewToken()
		if err != nil {
			return err
		}
		if err := store.ReapOneTimeLinks(ctx, t.Tx, t.now); err != nil {
			return err
		}
		_, err = store.CreateOneTimeLink(ctx, t.Tx, store.OneTimeLink{Kind: "github", CreatedBy: a.UserID,
			CreatedAt: t.now, ExpiresAt: t.now + int64(gitHubStateLifetime/time.Microsecond)}, hash)
		out = token
		return err
	})
	return out, err
}

// SpendGitHubState accepts a state once, from the administrator it was
// issued to.
func (s *Service) SpendGitHubState(ctx context.Context, a Actor, state string) error {
	if err := RequireInstanceAdmin(a); err != nil {
		return err
	}
	return s.write(ctx, a, func(t *wtx) error {
		l, err := store.OneTimeLinkByToken(ctx, t.Tx, auth.HashToken(state), t.now, "github")
		if errors.Is(err, store.ErrNotFound) || (err == nil && l.CreatedBy != a.UserID) {
			return errf(CodeForbidden, "that GitHub request did not start here, or it has expired; try again")
		}
		if err != nil {
			return err
		}
		return store.UseOneTimeLink(ctx, t.Tx, l.ID, t.now)
	})
}

// SaveGitHubApp keeps an app GitHub created from the manifest.
func (s *Service) SaveGitHubApp(ctx context.Context, a Actor, app store.GitHubApp) error {
	if err := RequireInstanceAdmin(a); err != nil {
		return err
	}
	return s.write(ctx, a, func(t *wtx) error {
		app.CreatedBy, app.CreatedAt = a.UserID, t.now
		if err := store.SaveGitHubApp(ctx, t.Tx, app); err != nil {
			return err
		}
		return t.log(ctx, store.Event{Kind: store.EvInstance, Entity: "instance",
			Field: "github_app", NewValue: app.Slug})
	})
}

// DeleteGitHubApp forgets an app and the accounts linked through it. The
// app itself stays on GitHub until it is deleted there; pull requests stay
// on their items.
func (s *Service) DeleteGitHubApp(ctx context.Context, a Actor, id int64) error {
	if err := RequireInstanceAdmin(a); err != nil {
		return err
	}
	return s.write(ctx, a, func(t *wtx) error {
		slug, err := store.DeleteGitHubApp(ctx, t.Tx, id)
		if err != nil {
			return err
		}
		return t.log(ctx, store.Event{Kind: store.EvInstance, Entity: "instance",
			Field: "github_app", OldValue: slug})
	})
}

// LinkInstallation lets the instance read an account's pull requests. The
// caller has asked GitHub that the installation is the app's.
func (s *Service) LinkInstallation(ctx context.Context, a Actor, appID, id int64, account, accountType string) error {
	if err := RequireInstanceAdmin(a); err != nil {
		return err
	}
	return s.write(ctx, a, func(t *wtx) error {
		added, err := store.LinkInstallation(ctx, t.Tx, store.GitHubInstallation{ID: id, AppID: appID, Account: account,
			AccountType: accountType, LinkedBy: a.UserID, LinkedAt: t.now})
		if err != nil || !added {
			return err
		}
		return t.log(ctx, store.Event{Kind: store.EvInstance, Entity: "instance",
			Field: "github_account", NewValue: account})
	})
}

func (s *Service) UnlinkInstallation(ctx context.Context, a Actor, id int64) error {
	if err := RequireInstanceAdmin(a); err != nil {
		return err
	}
	return s.unlinkInstallation(ctx, a, id)
}

// InstallationRemoved is GitHub saying the app was uninstalled from an
// account: there is nothing left to read there.
func (s *Service) InstallationRemoved(ctx context.Context, id int64) error {
	err := s.unlinkInstallation(ctx, Actor{}, id)
	if errors.Is(err, errNotFound) {
		return nil
	}
	return err
}

func (s *Service) unlinkInstallation(ctx context.Context, a Actor, id int64) error {
	return s.write(ctx, a, func(t *wtx) error {
		account, err := store.UnlinkInstallation(ctx, t.Tx, id)
		if err != nil {
			return err
		}
		return t.log(ctx, store.Event{Kind: store.EvInstance, Entity: "instance",
			Field: "github_account", OldValue: account})
	})
}

// Installation is one linked installation, for a caller about to sync it.
func (s *Service) Installation(ctx context.Context, a Actor, id int64) (store.GitHubInstallation, error) {
	if err := RequireInstanceAdmin(a); err != nil {
		return store.GitHubInstallation{}, err
	}
	i, err := store.Installation(ctx, s.DB.R, id)
	return i, mapErr(err)
}

// MarkSynced records how a sync of an installation ended.
func (s *Service) MarkSynced(ctx context.Context, id int64, syncErr error) error {
	msg := ""
	if syncErr != nil {
		msg = syncErr.Error()
	}
	return s.write(ctx, Actor{}, func(t *wtx) error {
		return store.MarkSynced(ctx, t.Tx, id, t.now, msg)
	})
}

// ApplyPullRequest brings one pull request's links up to date with its
// title, `identifiers` being the items the title names.
//
// A delivery from an installation that is not linked is dropped. One older
// than what is stored is dropped too: deliveries arrive out of order, and a
// sync reads what a webhook already wrote. A pull request that names no item
// is not kept at all.
func (s *Service) ApplyPullRequest(ctx context.Context, pr store.PullRequest, identifiers []string) error {
	return s.write(ctx, Actor{}, func(t *wtx) error {
		if _, err := store.Installation(ctx, t.Tx, pr.InstallationID); errors.Is(err, store.ErrNotFound) {
			return nil
		} else if err != nil {
			return err
		}
		want := map[int64]store.Item{}
		for _, id := range identifiers {
			if it, err := store.ItemByIdentifier(ctx, t.Tx, id); err == nil {
				want[it.ID] = it
			} else if !errors.Is(err, store.ErrNotFound) {
				return err
			}
		}
		have, err := store.PullRequestItems(ctx, t.Tx, pr.ID)
		if err != nil {
			return err
		}
		if len(want) == 0 && len(have) == 0 {
			return nil
		}
		oldState, written, err := store.UpsertPullRequest(ctx, t.Tx, pr)
		if err != nil || !written {
			return err
		}

		ref := pr.Ref()
		for _, id := range have {
			it, err := store.ItemByID(ctx, t.Tx, id)
			if err != nil {
				return err
			}
			if _, kept := want[id]; kept {
				delete(want, id)
				if oldState != "" && oldState != pr.State {
					if err := t.log(ctx, store.Event{Kind: store.EvPullRequestUpdated, Entity: "item",
						EntityID: id, ItemID: id, TeamID: it.TeamID, Field: "pull_request",
						OldValue: ref + " " + oldState, NewValue: ref + " " + pr.State}); err != nil {
						return err
					}
				}
				continue
			}
			if err := store.UnlinkPullRequest(ctx, t.Tx, id, pr.ID); err != nil {
				return err
			}
			if err := t.log(ctx, store.Event{Kind: store.EvPullRequestUnlinked, Entity: "item",
				EntityID: id, ItemID: id, TeamID: it.TeamID, Field: "pull_request", OldValue: ref}); err != nil {
				return err
			}
		}
		for id, it := range want {
			if err := store.LinkPullRequest(ctx, t.Tx, id, pr.ID); err != nil {
				return err
			}
			if err := t.log(ctx, store.Event{Kind: store.EvPullRequestLinked, Entity: "item",
				EntityID: id, ItemID: id, TeamID: it.TeamID, Field: "pull_request", NewValue: ref}); err != nil {
				return err
			}
		}
		// A pull request that no longer names anything is no longer kept.
		return store.DeletePullRequestIfUnlinked(ctx, t.Tx, pr.ID)
	})
}

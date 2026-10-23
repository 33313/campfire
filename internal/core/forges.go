package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"

	"github.com/33313/campfire/internal/store"
)

// Forges (DEV-114). An instance administrator connects GitHub, Forgejo or
// GitLab with an access token. From then on a pull request whose title names
// an item, at its start or its end, is listed on that item, and its state
// with it.
//
// Pull requests are written by nobody: the events carry no actor, the way
// the forge, not a user, made the change.

// hosted is each kind's own public forge, used when no address is given.
var hosted = map[string]string{"github": "https://github.com", "forgejo": "https://codeberg.org", "gitlab": "https://gitlab.com"}

// Forges are the connections, for the administration screen.
func (s *Service) Forges(ctx context.Context, a Actor) ([]store.Forge, error) {
	if err := RequireInstanceAdmin(a); err != nil {
		return nil, err
	}
	out, err := store.ListForges(ctx, s.DB.R)
	if out == nil {
		out = []store.Forge{}
	}
	return out, err
}

// Forge is one connection, token included, for a caller about to read it.
func (s *Service) Forge(ctx context.Context, a Actor, id int64) (store.Forge, error) {
	if err := RequireInstanceAdmin(a); err != nil {
		return store.Forge{}, err
	}
	return s.LoadForge(ctx, id)
}

// LoadForge is one connection with its secrets, for checking a delivery,
// which carries no session.
func (s *Service) LoadForge(ctx context.Context, id int64) (store.Forge, error) {
	f, err := store.LoadForge(ctx, s.DB.R, id)
	return f, mapErr(err)
}

// LoadForges is every connection with its secrets, for reading them on a
// timer.
func (s *Service) LoadForges(ctx context.Context) ([]store.Forge, error) {
	out, err := store.ListForges(ctx, s.DB.R)
	return out, mapErr(err)
}

// ForgeURL is the address a forge of this kind is reached at: its hosted one
// when none is given, otherwise a scheme and a host, and a path for one
// served under one.
func ForgeURL(kind, raw string) (string, error) {
	def, ok := hosted[kind]
	if !ok {
		return "", errf(CodeInvalid, "a forge is GitHub, Forgejo or GitLab")
	}
	if strings.TrimSpace(raw) == "" {
		return def, nil
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.RawQuery != "" {
		return "", errf(CodeInvalid, "a forge's address is like https://codeberg.org")
	}
	return u.Scheme + "://" + u.Host + strings.TrimRight(u.Path, "/"), nil
}

// AddForge keeps a connection whose token the caller has tried, and makes
// the secret its webhook signs with.
func (s *Service) AddForge(ctx context.Context, a Actor, f store.Forge) (store.Forge, error) {
	if err := RequireInstanceAdmin(a); err != nil {
		return f, err
	}
	if f.Mode != "poll" && f.Mode != "webhook" {
		return f, errf(CodeInvalid, "a forge is polled or sends webhooks")
	}
	if f.Token == "" {
		return f, errf(CodeInvalid, "an access token is required")
	}
	if f.PollMinutes == 0 {
		f.PollMinutes = 5
	}
	if f.PollMinutes < 1 || f.PollMinutes > 1440 {
		return f, errf(CodeInvalid, "a forge is checked every 1 to 1440 minutes")
	}
	var err error
	if f.URL, err = ForgeURL(f.Kind, f.URL); err != nil {
		return f, err
	}
	if f.Mode == "webhook" {
		f.WebhookSecret = newSecret()
	}
	err = s.write(ctx, a, func(t *wtx) error {
		f.CreatedBy, f.CreatedAt = a.UserID, t.now
		id, err := store.CreateForge(ctx, t.Tx, f)
		if err != nil {
			return err
		}
		f.ID = id
		return t.log(ctx, store.Event{Kind: store.EvInstance, Entity: "instance", Field: "forge", NewValue: f.URL})
	})
	return f, err
}

func newSecret() string {
	b := make([]byte, 24)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// RotateForgeSecret makes a new webhook secret, to be shown once. Deliveries
// signed with the old one are refused from now on.
func (s *Service) RotateForgeSecret(ctx context.Context, a Actor, id int64) (string, error) {
	if err := RequireInstanceAdmin(a); err != nil {
		return "", err
	}
	secret := newSecret()
	err := s.write(ctx, a, func(t *wtx) error {
		err := store.SetForgeSecret(ctx, t.Tx, id, secret)
		if errors.Is(err, store.ErrNotFound) {
			return errf(CodeNotFound, "no forge here takes webhooks with that id")
		}
		return err
	})
	return secret, err
}

// SetForgeToken replaces a connection's token, keeping its webhook.
func (s *Service) SetForgeToken(ctx context.Context, a Actor, id int64, account, token string) error {
	if err := RequireInstanceAdmin(a); err != nil {
		return err
	}
	if token == "" {
		return errf(CodeInvalid, "an access token is required")
	}
	return s.write(ctx, a, func(t *wtx) error {
		f, err := store.LoadForge(ctx, t.Tx, id)
		if err != nil {
			return err
		}
		if err := store.SetForgeToken(ctx, t.Tx, id, account, token); err != nil {
			return err
		}
		return t.log(ctx, store.Event{Kind: store.EvInstance, Entity: "instance", Field: "forge_token", NewValue: f.URL})
	})
}

// DeleteForge stops reading a forge. Its pull requests stay on their items.
func (s *Service) DeleteForge(ctx context.Context, a Actor, id int64) error {
	if err := RequireInstanceAdmin(a); err != nil {
		return err
	}
	return s.write(ctx, a, func(t *wtx) error {
		url, err := store.DeleteForge(ctx, t.Tx, id)
		if err != nil {
			return err
		}
		return t.log(ctx, store.Event{Kind: store.EvInstance, Entity: "instance", Field: "forge", OldValue: url})
	})
}

// MarkSynced records how a sync of a forge ended.
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
// A delivery for a forge that is gone is dropped. One older than what is
// stored is dropped too: deliveries arrive out of order, and a sync reads
// what a webhook already wrote. A pull request that names no item is not
// kept at all.
func (s *Service) ApplyPullRequest(ctx context.Context, forgeID int64, pr store.PullRequest, identifiers []string) error {
	return s.write(ctx, Actor{}, func(t *wtx) error {
		if _, err := store.LoadForge(ctx, t.Tx, forgeID); errors.Is(err, store.ErrNotFound) {
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
		prID, oldState, err := store.PullRequestByRemote(ctx, t.Tx, pr.Remote)
		if err != nil {
			return err
		}
		var have []int64
		if prID != 0 {
			if have, err = store.PullRequestItems(ctx, t.Tx, prID); err != nil {
				return err
			}
		}
		if len(want) == 0 && len(have) == 0 {
			return nil
		}
		prID, err = store.UpsertPullRequest(ctx, t.Tx, pr)
		if err != nil || prID == 0 {
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
			if err := store.UnlinkPullRequest(ctx, t.Tx, id, prID); err != nil {
				return err
			}
			if err := t.log(ctx, store.Event{Kind: store.EvPullRequestUnlinked, Entity: "item",
				EntityID: id, ItemID: id, TeamID: it.TeamID, Field: "pull_request", OldValue: ref}); err != nil {
				return err
			}
		}
		for id, it := range want {
			if err := store.LinkPullRequest(ctx, t.Tx, id, prID); err != nil {
				return err
			}
			if err := t.log(ctx, store.Event{Kind: store.EvPullRequestLinked, Entity: "item",
				EntityID: id, ItemID: id, TeamID: it.TeamID, Field: "pull_request", NewValue: ref}); err != nil {
				return err
			}
		}
		// A pull request that no longer names anything is no longer kept.
		return store.DeletePullRequestIfUnlinked(ctx, t.Tx, prID)
	})
}

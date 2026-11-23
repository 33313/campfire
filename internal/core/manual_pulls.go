package core

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/33313/campfire/internal/store"
)

type LookupPull func(context.Context, *url.URL) store.PullRequest

func (s *Service) AddManualPull(ctx context.Context, a Actor, itemID int64, raw string, lookup LookupPull) error {
	if err := requireUser(a); err != nil {
		return err
	}
	it, err := store.ItemByID(ctx, s.DB.R, itemID)
	if err != nil {
		return mapErr(err)
	}
	if err := s.requireItemEdit(ctx, s.DB.R, a, it); err != nil {
		return err
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if len(raw) > 2048 || err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil {
		return invalidField("url", "enter an HTTP or HTTPS pull request URL")
	}
	u.RawQuery, u.Fragment, u.RawFragment, u.ForceQuery = "", "", "", false
	u.Host = strings.ToLower(u.Host)
	u.Path, u.RawPath = strings.TrimRight(u.Path, "/"), strings.TrimRight(u.RawPath, "/")
	p := lookup(ctx, u)
	return s.write(ctx, a, func(t *wtx) error {
		fresh, err := store.ItemByID(ctx, t.Tx, itemID)
		if err != nil {
			return err
		}
		if err := s.requireItemEdit(ctx, t.Tx, a, fresh); err != nil {
			return err
		}
		if err := store.AddManualPull(ctx, t.Tx, itemID, p, t.now); err != nil {
			return err
		}
		return notifyPull(ctx, t, itemID, fresh.TeamID)
	})
}

func (s *Service) RemoveManualPull(ctx context.Context, a Actor, itemID, pullID int64) error {
	return s.write(ctx, a, func(t *wtx) error {
		it, err := store.ItemByID(ctx, t.Tx, itemID)
		if err != nil {
			return err
		}
		if err := s.requireItemEdit(ctx, t.Tx, a, it); err != nil {
			return err
		}
		if err := store.RemoveManualPull(ctx, t.Tx, itemID, pullID); err != nil {
			return err
		}
		return notifyPull(ctx, t, itemID, it.TeamID)
	})
}

func notifyPull(ctx context.Context, t *wtx, item, team int64) error {
	return t.log(ctx, store.Event{Kind: store.EvPullRequestChanged, Entity: "item", EntityID: item, ItemID: item, TeamID: team})
}

func (s *Service) CheckManualPulls(ctx context.Context, lookup LookupPull) error {
	pulls, err := store.DueManualPulls(ctx, s.DB.R, s.Clock()-int64((15*time.Minute)/time.Microsecond))
	if err != nil {
		return err
	}
	for _, ref := range pulls {
		u, err := url.Parse(ref.URL)
		if err != nil {
			return err
		}
		p := lookup(ctx, u)
		if err := s.write(ctx, Actor{}, func(t *wtx) error {
			changed, err := store.RefreshManualPull(ctx, t.Tx, ref.ID, p, t.now)
			if err != nil || !changed {
				return err
			}
			return notifyPull(ctx, t, ref.Item, ref.Team)
		}); err != nil {
			return err
		}
	}
	return nil
}

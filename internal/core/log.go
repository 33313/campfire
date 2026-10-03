package core

import (
	"context"
	"time"

	"github.com/33313/campfire/internal/store"
)

// The log is ordered by sequence number, never by timestamp, and paginated by
// keyset — so a footer offers "load older from 41192", never a page number
// (SPEC.md §10).

type LogPage struct {
	Events []store.Event `json:"events"`
	// Cursor is the sequence number to ask for next. Empty at the end of the
	// list.
	Cursor int64 `json:"cursor,omitempty"`
}

// Log reads the audit log under its ordinary scoping: an item's history, a
// team's log, or — for instance administrators only — the global log.
func (s *Service) Log(ctx context.Context, a Actor, f store.EventFilter, beforeSeq int64, limit int) (*LogPage, error) {
	if err := requireUser(a); err != nil {
		return nil, err
	}
	if f.ItemID == 0 && f.TeamID == 0 && !a.IsAdmin {
		return nil, errForbidden
	}
	limit = clampLimit(limit)
	events, err := store.ListEvents(ctx, s.DB.R, f, beforeSeq, limit)
	if err != nil {
		return nil, mapErr(err)
	}
	page := &LogPage{Events: events}
	if page.Events == nil {
		page.Events = []store.Event{}
	}
	if len(events) == limit {
		page.Cursor = events[len(events)-1].Seq
	}
	return page, nil
}

// LatestSeq is the cursor a client opens its event stream at.
func (s *Service) LatestSeq(ctx context.Context) (int64, error) {
	seq, err := store.LatestSeq(ctx, s.DB.R)
	return seq, mapErr(err)
}

// MaxStreamReplay is how far behind a reconnecting client may be and still be
// caught up event by event. Beyond it the client is told to discard its state
// and refetch (SPEC.md §12.2): a silent partial replay MUST NOT occur, and
// replaying tens of thousands of notifications is slower than one refetch.
const MaxStreamReplay = 2000

// NotificationFilter returns one stream's per-event visibility test —
// evaluated at fan-out against current permissions, never cached at subscribe
// time, so that a revoked permission takes effect on the next event and a
// granted one is not missed (SPEC.md §12.3).
//
// Reading items is instance-wide, so a team-scoped event goes to every
// authenticated user. An event with no team is an instance-level one, which
// only the global log shows and only instance administrators may read (§10).
//
// The permission read is refreshed at most once a second rather than once per
// event: a bulk edit writes one event per item and every one of them is fanned
// out to every open connection, which a point read per event cannot afford.
func (s *Service) NotificationFilter(a Actor) func(context.Context, int64) bool {
	var (
		checked      time.Time
		alive, admin bool
	)
	return func(ctx context.Context, teamID int64) bool {
		if now := time.Now(); now.Sub(checked) >= time.Second {
			checked = now
			u, err := store.UserByID(ctx, s.DB.R, a.UserID)
			alive = err == nil
			admin = err == nil && u.IsAdmin
		}
		if !alive {
			return false // the account is gone; so is its stream
		}
		return teamID != 0 || admin
	}
}

// StreamReplay returns the notifications a client missed, or reports that it
// is too far behind to be caught up.
func (s *Service) StreamReplay(ctx context.Context, a Actor, afterSeq int64) (events []store.Event, tooFarBehind bool, err error) {
	if err := requireUser(a); err != nil {
		return nil, false, err
	}
	latest, err := store.LatestSeq(ctx, s.DB.R)
	if err != nil {
		return nil, false, mapErr(err)
	}
	if latest-afterSeq > MaxStreamReplay {
		return nil, true, nil
	}
	events, err = store.EventsAfter(ctx, s.DB.R, afterSeq, MaxStreamReplay)
	return events, false, mapErr(err)
}

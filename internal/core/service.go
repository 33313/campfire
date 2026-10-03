package core

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/33313/campfire/internal/store"
)

// Actor is who a call is on behalf of, resolved fresh on every call.
//
// Nothing about permissions is cached here or anywhere else (SPEC.md §13.2
// rule 5): a revoked permission takes effect on the next call, and a granted
// one is not missed.
type Actor struct {
	UserID   int64
	Username string
	IsAdmin  bool
	// AgentTokenID is the token an MCP call arrived on, and zero for anything
	// done in the client. It is recorded on every event the call writes
	// (SPEC.md §14.4).
	AgentTokenID int64
	SessionID    int64
}

func (a Actor) authenticated() bool { return a.UserID != 0 }

// Service holds the database and the notification hook. It has no other state,
// by rule.
type Service struct {
	DB *store.DB

	// Notify is called once per event, after the transaction that wrote it has
	// committed, with the thin notification of SPEC.md §12.3. It is nil when
	// nothing is listening; the application is correct either way, because the
	// stream is not a source of truth (§12.4).
	Notify func(store.Event)

	// Clock is overridable for tests only.
	Clock func() int64
}

func New(db *store.DB) *Service {
	return &Service{DB: db, Clock: store.Now}
}

// wtx is a write transaction that also collects the events written inside it.
//
// Events are appended within the transaction, so the log cannot disagree with
// the data, and published only after it commits, so a client that refetches on
// the notification finds the change already there.
type wtx struct {
	*sql.Tx
	svc     *Service
	actor   Actor
	now     int64
	pending []store.Event
}

func (t *wtx) log(ctx context.Context, e store.Event) error {
	e.At = t.now
	e.ActorID = t.actor.UserID
	e.AgentTokenID = t.actor.AgentTokenID
	if e.ItemID != 0 && e.ItemIdentifier == "" {
		// The live-update notification carries the item identifier
		// (SPEC.md §12.3), and the log renders it. Resolving it here means no
		// caller has to remember to.
		if identifier, err := store.IdentifierOf(ctx, t.Tx, e.ItemID); err == nil {
			e.ItemIdentifier = identifier
		}
	}
	seq, err := store.AppendEvent(ctx, t.Tx, e)
	if err != nil {
		return err
	}
	e.Seq = seq
	t.pending = append(t.pending, e)
	return nil
}

// logField records one field change. A mutation touching three fields writes
// three events: a log that summarises is a log that cannot answer what
// happened to one ticket (SPEC.md §6.5).
func (t *wtx) logField(ctx context.Context, it store.Item, field, old, new string) error {
	return t.log(ctx, store.Event{
		Kind: store.EvItemUpdated, Entity: "item", EntityID: it.ID,
		ItemID: it.ID, TeamID: it.TeamID, Field: field, OldValue: old, NewValue: new,
	})
}

func (s *Service) write(ctx context.Context, a Actor, fn func(*wtx) error) error {
	t := &wtx{svc: s, actor: a, now: s.Clock()}
	err := s.DB.Write(ctx, func(tx *sql.Tx) error {
		t.Tx = tx
		t.pending = t.pending[:0]
		return fn(t)
	})
	if err != nil {
		return mapErr(err)
	}
	if s.Notify != nil {
		for _, e := range t.pending {
			s.Notify(e)
		}
	}
	return nil
}

// mapErr turns storage errors into the codes the interfaces render. Anything
// unrecognised stays as it is and becomes a 500: an unexpected error should
// look unexpected.
func mapErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, store.ErrNotFound):
		return errNotFound
	case errors.Is(err, store.ErrConflict):
		return &Error{Code: CodeConflict, Message: store.ErrConflict.Error()}
	case store.IsUniqueViolation(err):
		return errf(CodeConflict, "that value is already taken (%s)", store.ConstraintIndex(err))
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return err
}

const sessionLifetime = 30 * 24 * time.Hour

// list returns a non-nil slice. A nil slice encodes as JSON null, and a client
// that maps over its results should not have to guard every one of them.
func list[T any](in []T, err error) ([]T, error) {
	if in == nil {
		in = []T{}
	}
	return in, err
}

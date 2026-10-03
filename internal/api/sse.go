package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/33313/campfire/internal/core"
	"github.com/33313/campfire/internal/stream"
)

// Live updates over server-sent events, on the same origin and under the same
// authentication as the rest of the API (SPEC.md §12.1).
//
// WebSocket was rejected: the application needs server-to-client push only,
// browsers cannot set headers on a WebSocket handshake so it would need a
// second authentication path, and reconnection and resume would be
// hand-written. EventSource resumes from Last-Event-ID on its own, and that
// identifier is the audit log's sequence number.

// heartbeat is sent well inside the sixty-second idle timeout common to
// proxies, which is one of the operational requirements of SPEC.md §12.5.
const heartbeat = 20 * time.Second

func (s *Server) streamHandler(w http.ResponseWriter, r *http.Request) {
	a, err := s.actor(r)
	if err != nil || a.UserID == 0 {
		fail(w, &core.Error{Code: core.CodeUnauthenticated, Message: "sign in first"})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		fail(w, &core.Error{Code: "internal", Message: "this server cannot stream"})
		return
	}

	// Subscribe before reading anything. The client is required to open the
	// stream before fetching state (SPEC.md §12.2), and the server owes it the
	// same discipline: taking the cursor first would leave a window in which a
	// commit is neither replayed nor pushed.
	notifications, reset, cancel := s.bus.Subscribe()
	defer cancel()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Response buffering must be disabled on any intervening proxy
	// (SPEC.md §12.5); this asks nginx directly.
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx := r.Context()
	sent := int64(0)
	canSee := s.svc.NotificationFilter(a)

	// EventSource resends the last identifier it received; a first connection
	// may also name a cursor explicitly.
	cursor, _ := strconv.ParseInt(r.Header.Get("Last-Event-ID"), 10, 64)
	if cursor == 0 {
		cursor = queryInt(r, "cursor")
	}
	if cursor > 0 {
		events, tooFar, err := s.svc.StreamReplay(ctx, a, cursor)
		if err != nil {
			return
		}
		if tooFar {
			// An explicit instruction to discard and refetch. A silent partial
			// replay MUST NOT occur (SPEC.md §12.2).
			writeEvent(w, flusher, 0, stream.Notification{Kind: "reset", Reset: true})
		}
		for _, e := range events {
			if !canSee(ctx, e.TeamID) {
				continue
			}
			writeEvent(w, flusher, e.Seq, stream.Notification{
				Seq: e.Seq, Kind: e.Kind, Identifier: e.ItemIdentifier, TeamID: e.TeamID})
			sent = e.Seq
		}
	}

	// "hello" carries the cursor the client is now at, so a first connection
	// knows where to resume from without having read the log.
	if sent == 0 {
		latest, err := s.svc.LatestSeq(ctx)
		if err == nil {
			sent = latest
			writeEvent(w, flusher, latest, stream.Notification{Seq: latest, Kind: "hello"})
		}
	}

	ticker := time.NewTicker(heartbeat)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-reset:
			writeEvent(w, flusher, 0, stream.Notification{Kind: "reset", Reset: true})
			return
		case n, ok := <-notifications:
			if !ok {
				return
			}
			if n.Seq <= sent {
				continue // already delivered by the replay above
			}
			// Authorization is evaluated per event, here, against current
			// permissions — never cached at subscribe time (SPEC.md §12.3).
			if !canSee(ctx, n.TeamID) {
				continue
			}
			writeEvent(w, flusher, n.Seq, n)
			sent = n.Seq
		case <-ticker.C:
			fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}

func writeEvent(w http.ResponseWriter, f http.Flusher, id int64, n stream.Notification) {
	body, err := json.Marshal(n)
	if err != nil {
		return
	}
	if id > 0 {
		// The identifier is the audit log's sequence number, which is what
		// EventSource sends back as Last-Event-ID. Cursor bookkeeping is not
		// hand-written on either side (SPEC.md §12.2).
		fmt.Fprintf(w, "id: %d\n", id)
	}
	fmt.Fprintf(w, "data: %s\n\n", body)
	f.Flush()
}

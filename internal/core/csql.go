package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	_ "time/tzdata" // one binary: a reader's zone must not depend on the host having zoneinfo

	"github.com/campfire/campfire/internal/csql"
	"github.com/campfire/campfire/internal/store"
)

// A CSQL query arrives as text from the filter bar, an agent or a saved view,
// and is resolved against the instance before it is compiled: a name that
// names nothing is refused, pointing at itself, rather than matching nothing
// in silence.

type resolver struct {
	ctx context.Context
	q   store.Q
	me  int64
	now time.Time // in the reader's zone
	// stored is set for a saved view's own query, which was checked when it
	// was saved: a state it names that has since gone matches nothing.
	stored bool
}

func (r resolver) Me() int64      { return r.me }
func (r resolver) Now() time.Time { return r.now }

func (r resolver) StateExists(name string) bool {
	if r.stored {
		return true
	}
	var one int
	return r.q.QueryRowContext(r.ctx,
		`SELECT 1 FROM work_states WHERE name_folded = ? LIMIT 1`, store.Fold(name)).Scan(&one) == nil
}

func (r resolver) Resolve(field, name string) (int64, error) {
	switch field {
	case "assignee", "reporter":
		u, err := store.UserByUsername(r.ctx, r.q, name)
		if err != nil {
			return 0, fmt.Errorf("no user is called %q", name)
		}
		return u.ID, nil
	case "label":
		l, err := store.LabelByName(r.ctx, r.q, name)
		if err != nil {
			return 0, fmt.Errorf("no label is called %q", name)
		}
		return l.ID, nil
	case "team":
		if t, err := store.TeamByCode(r.ctx, r.q, name); err == nil {
			return t.ID, nil
		}
		var id int64
		if r.q.QueryRowContext(r.ctx, `SELECT id FROM teams WHERE name_folded = ?`,
			store.Fold(name)).Scan(&id) == nil {
			return id, nil
		}
		return 0, fmt.Errorf("no team has the code or name %q", name)
	case "bucket":
		// An identifier first, as a bucket is written on a card; otherwise its
		// title, which is only good while one bucket has it.
		if _, _, ok := store.ParseIdentifier(name); ok {
			it, err := store.ItemByIdentifier(r.ctx, r.q, name)
			if err != nil {
				return 0, fmt.Errorf("nothing is %s", name)
			}
			if it.Kind != store.KindBucket {
				return 0, fmt.Errorf("%s is a ticket, not a bucket", name)
			}
			return it.ID, nil
		}
		// Titles have no folded column, so they are folded here; buckets are
		// few enough to read them all.
		rows, err := r.q.QueryContext(r.ctx, `
			SELECT i.id, ot.short_code || '-' || i.number, i.title FROM items i
			JOIN teams ot ON ot.id = i.origin_team_id
			WHERE i.kind = 'bucket'`)
		if err != nil {
			return 0, err
		}
		defer rows.Close()
		var id int64
		var found []string
		for rows.Next() {
			var bid int64
			var ident, title string
			if err := rows.Scan(&bid, &ident, &title); err != nil {
				return 0, err
			}
			if store.Fold(title) == store.Fold(name) {
				id = bid
				found = append(found, ident)
			}
		}
		if err := rows.Err(); err != nil {
			return 0, err
		}
		switch len(found) {
		case 0:
			return 0, fmt.Errorf("no bucket is called %q", name)
		case 1:
			return id, nil
		}
		return 0, fmt.Errorf("%d buckets are called %q (%s); use the identifier",
			len(found), name, strings.Join(found, ", "))
	}
	return 0, fmt.Errorf("%s does not name things", field)
}

// zone is the reader's time zone, which a day in a query is read in (SPEC.md
// §9.2). None given is UTC.
func zone(tz string) (*time.Location, error) {
	if tz == "" {
		return time.UTC, nil
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, invalidField("tz", "%q is not a time zone; use a name such as Europe/Warsaw", tz)
	}
	return loc, nil
}

// parseQuery reads and resolves a query for the actor, or says where it is
// wrong. An empty query is nil.
func (s *Service) parseQuery(ctx context.Context, a Actor, q string, loc *time.Location) (csql.Node, error) {
	n, err := csql.Parse(q)
	if err == nil && n != nil {
		err = csql.Resolve(n, resolver{ctx: ctx, q: s.DB.R, me: a.UserID, now: time.UnixMicro(s.now()).In(loc)})
	}
	var e *csql.Error
	if errors.As(err, &e) {
		return nil, &Error{Code: CodeInvalid, Field: "query", Message: e.Msg, Span: []int{e.Pos, e.End}}
	}
	return n, err
}

// storeFilter is the filter as the store takes it: its query and the
// queries of its views, and-ed and compiled. Only the query can be wrong, so
// an error's span is always into it.
func (s *Service) storeFilter(ctx context.Context, a Actor, f Filter) (store.ItemFilter, error) {
	var sf store.ItemFilter
	loc, err := zone(f.TZ)
	if err != nil {
		return sf, err
	}
	n, err := s.parseQuery(ctx, a, f.Query, loc)
	if err != nil {
		return sf, err
	}
	var all []csql.Node
	if n != nil {
		all = append(all, n)
	}
	for _, id := range f.ViewIDs {
		v, err := store.SavedViewByID(ctx, s.DB.R, id)
		if err != nil {
			return sf, mapErr(err)
		}
		vn, err := csql.Parse(v.Query)
		if err == nil && vn != nil {
			err = csql.Resolve(vn, resolver{ctx: ctx, q: s.DB.R, me: a.UserID, stored: true,
				now: time.UnixMicro(s.now()).In(loc)})
		}
		if err != nil {
			return sf, err // stored canonical: not a reader's mistake
		}
		if vn != nil {
			all = append(all, vn)
		}
	}
	switch len(all) {
	case 0:
	case 1:
		sf.Where, sf.WhereArgs = csql.Compile(all[0])
	default:
		sf.Where, sf.WhereArgs = csql.Compile(csql.And{Of: all})
	}
	return sf, nil
}

// humanize writes a stored view's query with today's names, and lists what
// it names that has been deleted.
func humanize(v *store.SavedView) {
	names := map[csql.Ref]string{}
	for _, r := range v.Refs {
		if r.Name == "" {
			v.Missing = append(v.Missing, fmt.Sprintf("deleted %s #%d", r.Field, r.ID))
			continue
		}
		names[csql.Ref{Field: r.Field, ID: r.ID}] = r.Name
	}
	n, err := csql.Parse(v.Query)
	if err != nil {
		return // stored canonical, so this does not happen; the raw text is still true
	}
	v.Query = csql.Format(n, func(field string, id int64) (string, bool) {
		s, ok := names[csql.Ref{Field: field, ID: id}]
		return s, ok
	})
}

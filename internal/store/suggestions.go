package store

import (
	"context"
	"database/sql/driver"
	"strings"
	"unicode"

	"modernc.org/sqlite"
)

func matchScore(identifier, title, description, query string) int64 {
	if identifier == query {
		return 5
	}
	if strings.HasPrefix(title, query) {
		return 4
	}
	if strings.Contains(title, query) {
		previousWord := true
		for i, r := range title {
			if !previousWord && strings.HasPrefix(title[i:], query) {
				return 3
			}
			previousWord = unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r)
		}
		return 2
	}
	if strings.Contains(identifier, query) || strings.Contains(description, query) {
		return 1
	}
	return 0
}

func init() {
	sqlite.MustRegisterDeterministicScalarFunction("campfire_suggestion_score", 4,
		func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return matchScore(strings.ToLower(args[0].(string)), strings.ToLower(args[1].(string)),
				strings.ToLower(args[2].(string)), args[3].(string)), nil
		})
}

// SuggestItems ranks before limiting so a new weak match cannot hide an older
// exact one. This is a shortlist, not a paginated search result.
func SuggestItems(ctx context.Context, q Q, f ItemFilter, query string, limit int) ([]Item, error) {
	const score = `campfire_suggestion_score(ot.short_code || '-' || i.number, i.title, i.description, ?)`
	clause, args := f.where()
	join := " AND "
	if clause == "" {
		join = " WHERE "
	}
	rows, err := q.QueryContext(ctx, `SELECT `+itemColumns+itemJoins+clause+join+score+` > 0
		ORDER BY `+score+` DESC, i.id DESC LIMIT ?`, append(args, query, query, limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

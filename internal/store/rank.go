package store

import (
	"errors"
	"strings"
)

// Lexical ranks (SPEC.md §9.3).
//
// A rank is a short string ordered lexically. An item dropped between two
// neighbours takes a string sorting between theirs, so a move writes one row
// and never renumbers a column. The alphabet is restricted to 0-9A-Za-z,
// whose ASCII order is also its digit order, so byte comparison — SQLite's
// default collation for TEXT — is the ordering.
//
// Fractional numeric ranks were rejected: a float between two neighbours
// exhausts double precision after about fifty drops on the same midpoint, and
// repairing that needs a background job the application does not have
// (Appendix B).

const rankAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

const rankBase = len(rankAlphabet)

// ErrRankOrder is returned when the bounds handed to RankBetween are not in
// ascending order, which means the caller read a stale column.
var ErrRankOrder = errors.New("rank bounds out of order")

// ErrRankAlphabet is returned for a rank containing a character outside
// 0-9A-Za-z.
var ErrRankAlphabet = errors.New("rank contains a character outside 0-9A-Za-z")

func rankDigit(c byte) int { return strings.IndexByte(rankAlphabet, c) }

// ValidRank reports whether s is a well-formed rank.
func ValidRank(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if rankDigit(s[i]) < 0 {
			return false
		}
	}
	return true
}

// RankBetween returns a rank sorting strictly after lo and strictly before hi.
//
// An empty lo means "before everything in the column" and an empty hi means
// "after everything". RankBetween("", "") is the rank of the first item in an
// empty column, and lands mid-alphabet so the column can grow in both
// directions without lengthening.
func RankBetween(lo, hi string) (string, error) {
	if lo != "" && !ValidRank(lo) {
		return "", ErrRankAlphabet
	}
	if hi != "" && !ValidRank(hi) {
		return "", ErrRankAlphabet
	}
	if lo != "" && hi != "" && lo >= hi {
		return "", ErrRankOrder
	}

	var out []byte
	for i := 0; ; i++ {
		low := 0
		if i < len(lo) {
			low = rankDigit(lo[i])
		}
		high := rankBase
		if hi != "" && i < len(hi) {
			high = rankDigit(hi[i])
		}
		if high-low > 1 {
			// Room for a digit between them: take the midpoint and stop.
			// It is never digit zero, so a rank never ends in '0' and every
			// rank has room below it.
			return string(append(out, rankAlphabet[low+(high-low)/2])), nil
		}
		// No room at this position. Adopt lo's digit — or zero once lo has
		// run out — and look one position deeper.
		if i < len(lo) {
			out = append(out, lo[i])
		} else {
			out = append(out, rankAlphabet[0])
		}
	}
}

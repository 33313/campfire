package store

import (
	"testing"
)

func TestRankBetweenOrders(t *testing.T) {
	cases := []struct{ lo, hi string }{
		{"", ""},
		{"", "V"},
		{"V", ""},
		{"1", "2"},
		{"1", "11"},
		{"zzzz", ""},
		{"", "0001"},
	}
	for _, c := range cases {
		got, err := RankBetween(c.lo, c.hi)
		if err != nil {
			t.Fatalf("RankBetween(%q, %q): %v", c.lo, c.hi, err)
		}
		if !ValidRank(got) {
			t.Fatalf("RankBetween(%q, %q) = %q, not a valid rank", c.lo, c.hi, got)
		}
		if c.lo != "" && got <= c.lo {
			t.Fatalf("RankBetween(%q, %q) = %q, not after lo", c.lo, c.hi, got)
		}
		if c.hi != "" && got >= c.hi {
			t.Fatalf("RankBetween(%q, %q) = %q, not before hi", c.lo, c.hi, got)
		}
	}
}

// The failure mode fractional ranks have is repeated insertion at the same
// midpoint. Lexical ranks must survive it: after a thousand drops in the same
// gap the column is still strictly ordered.
func TestRankBetweenSurvivesRepeatedMidpointInsertion(t *testing.T) {
	lo, hi := "1", "2"
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		mid, err := RankBetween(lo, hi)
		if err != nil {
			t.Fatalf("drop %d: %v", i, err)
		}
		if mid <= lo || mid >= hi {
			t.Fatalf("drop %d: %q not strictly between %q and %q", i, mid, lo, hi)
		}
		if seen[mid] {
			t.Fatalf("drop %d: %q repeated", i, mid)
		}
		seen[mid] = true
		hi = mid // always drop just above lo, the worst case
	}
}

func TestRankBetweenRejectsBadBounds(t *testing.T) {
	if _, err := RankBetween("2", "1"); err != ErrRankOrder {
		t.Fatalf("descending bounds: got %v", err)
	}
	if _, err := RankBetween("1", "1"); err != ErrRankOrder {
		t.Fatalf("equal bounds: got %v", err)
	}
	if _, err := RankBetween("a-b", ""); err != ErrRankAlphabet {
		t.Fatalf("out-of-alphabet bound: got %v", err)
	}
}

package csql

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// The corpus is what both parsers must say: this one, and the client's
// (internal/web/src/lib/csql.test.js runs the same file).
func TestCorpus(t *testing.T) {
	raw, err := os.ReadFile("testdata/corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Q, Out, Err string
		Span        []int
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		n, err := Parse(c.Q)
		if c.Err == "" {
			if err != nil {
				t.Errorf("%q: %v", c.Q, err)
			} else if got := Format(n, nil); got != c.Out {
				t.Errorf("%q printed %q, want %q", c.Q, got, c.Out)
			}
			continue
		}
		var e *Error
		if !errors.As(err, &e) {
			t.Errorf("%q: got %v, want %q", c.Q, err, c.Err)
			continue
		}
		if e.Msg != c.Err || e.Pos != c.Span[0] || e.End != c.Span[1] {
			t.Errorf("%q: got %q at [%d %d], want %q at %v", c.Q, e.Msg, e.Pos, e.End, c.Err, c.Span)
		}
	}
}

// A resolver over a fixed little world: marta is 1, sally 2, the label roses
// 7, the bucket DEV-6 is 60, the team DEV is 5, and it is the afternoon of
// 2026-10-01 in Warsaw.
type world struct{}

var warsaw, _ = time.LoadLocation("Europe/Warsaw")

func (world) Me() int64 { return 99 }
func (world) Now() time.Time {
	return time.Date(2026, 10, 1, 15, 0, 0, 0, warsaw)
}
func (world) StateExists(name string) bool {
	return strings.EqualFold(name, "In Review") || strings.EqualFold(name, "Done")
}
func (world) Resolve(field, name string) (int64, error) {
	ids := map[string]int64{"assignee:marta": 1, "assignee:sally": 2, "reporter:marta": 1,
		"label:roses": 7, "bucket:DEV-6": 60, "team:DEV": 5}
	if id, ok := ids[field+":"+name]; ok {
		return id, nil
	}
	return 0, errors.New("no " + field + " is called " + name)
}

func names(field string, id int64) (string, bool) {
	n, ok := map[int64]string{1: "marta", 2: "sally", 7: "roses", 60: "DEV-6", 5: "DEV"}[id]
	return n, ok
}

func TestCanonicalNamesByIDAndReadsBack(t *testing.T) {
	for _, tc := range []struct{ in, canonical string }{
		{"assignee in (marta, none, me) and not label = roses", "assignee in (#1, none, me) and not label = #7"},
		{"bucket = DEV-6 or team = DEV", "bucket = #60 or team = #5"},
		{"state = Done and priority <= P2 and created >= -7d", "state = Done and priority <= 2 and created >= -7d"},
		{"label not in (#404)", "label not in (#404)"},
	} {
		n, err := Parse(tc.in)
		if err == nil {
			err = Resolve(n, world{})
		}
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if got := Canonical(n); got != tc.canonical {
			t.Errorf("%q canonical = %q, want %q", tc.in, got, tc.canonical)
		}
		// Read back and named again, it is what was typed.
		again, _ := Parse(Canonical(n))
		if got, want := Format(again, names), Format(n, nil); got != want && !strings.Contains(tc.in, "#404") {
			t.Errorf("%q reads back as %q, want %q", tc.in, got, want)
		}
	}
}

func TestDaysAreTheReadersDays(t *testing.T) {
	n, _ := Parse("created >= -7d and due <= +2w and completed = today and started < 2026-09-01")
	if err := Resolve(n, world{}); err != nil {
		t.Fatal(err)
	}
	var got []string
	walk(n, func(c *Cond) error {
		v := c.Values[0]
		got = append(got, v.Day+" "+time.UnixMicro(v.From).In(warsaw).Format(time.RFC3339))
		return nil
	})
	want := []string{
		"2026-09-24 2026-09-24T00:00:00+02:00", "2026-10-15 2026-10-15T00:00:00+02:00",
		"2026-10-01 2026-10-01T00:00:00+02:00", "2026-09-01 2026-09-01T00:00:00+02:00",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("days %v, want %v", got, want)
	}
}

func TestResolvingPointsAtWhatIsNotThere(t *testing.T) {
	for _, tc := range []struct{ in, at, msg string }{
		{"assignee = marty", "marty", "no assignee is called marty"},
		{"assignee in (marta, \"sal ly\")", "\"sal ly\"", "no assignee is called sal ly"},
		{"state = Doing", "Doing", "no work state"},
	} {
		n, err := Parse(tc.in)
		if err == nil {
			err = Resolve(n, world{})
		}
		var e *Error
		if !errors.As(err, &e) || !strings.Contains(e.Msg, tc.msg) || tc.in[e.Pos:e.End] != tc.at {
			t.Errorf("%q: got %v, want %q at %q", tc.in, err, tc.msg, tc.at)
		}
	}
}

func TestRefsAreWhatIsNamedByID(t *testing.T) {
	n, _ := Parse("assignee in (marta, me, none) and label = roses and state = Done and priority = 1 and label = roses")
	if err := Resolve(n, world{}); err != nil {
		t.Fatal(err)
	}
	got := Refs(n)
	if len(got) != 2 || got[0] != (Ref{"assignee", 1}) || got[1] != (Ref{"label", 7}) {
		t.Fatalf("refs = %v", got)
	}
}

package updates

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		release, current string
		want             bool
	}{
		{"v0.12.0", "v0.11.0", true},
		{"v0.11.0", "v0.11.0", false},
		{"v0.10.9", "v0.11.0", false},
		{"v1.0.0", "v0.99.99", true},
		{"v0.11.10", "v0.11.9", true},
		{"v1.0.0", "v1.0.0-rc1", true},
		{"v0.11.0", "v0.11.1-0.20261005-abc123+dirty", false},
		{"v0.11.1", "v0.11.1-0.20261005-abc123", true},
		{"v0.12.0", "dev", false},
		{"nonsense", "v0.11.0", false},
	} {
		if got := Newer(c.release, c.current); got != c.want {
			t.Errorf("Newer(%q, %q) = %v", c.release, c.current, got)
		}
	}
}

func TestCheckReadsTheLatestRelease(t *testing.T) {
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"tag_name":"v0.12.0","html_url":"https://example.com/v0.12.0"}`))
	}))
	defer gh.Close()
	c := New("v0.11.0")
	c.URL = gh.URL
	c.Check(context.Background())
	s := c.Status()
	if !s.Newer || s.Latest.Version != "v0.12.0" || s.Latest.URL != "https://example.com/v0.12.0" || s.Error != "" {
		t.Fatalf("status %+v", s)
	}
}

// A failed check says why, and keeps what the last good one found.
func TestAFailedCheckKeepsTheLastAnswer(t *testing.T) {
	ok := true
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"tag_name":"v0.12.0"}`))
	}))
	defer gh.Close()
	c := New("v0.11.0")
	c.URL = gh.URL
	c.Check(context.Background())
	ok = false
	c.Check(context.Background())
	s := c.Status()
	if s.Latest == nil || s.Latest.Version != "v0.12.0" || s.Error == "" {
		t.Fatalf("status %+v", s)
	}
}

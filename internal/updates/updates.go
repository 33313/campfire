// Package updates asks GitHub, once a day, whether a newer release exists.
//
// It only tells: the administration screen says so, and updating stays the
// operator's act (DEV-43; self-updating is DEV-96).
package updates

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// LatestURL is the releases API for this repository. A public repository's
// releases need no token.
const LatestURL = "https://api.github.com/repos/33313/campfire/releases/latest"

const interval = 24 * time.Hour

type Release struct {
	Version string    `json:"version"`
	URL     string    `json:"url"`
	Checked time.Time `json:"checked"`
}

// Checker holds what the last check found.
type Checker struct {
	Current string
	URL     string // LatestURL, or a test server

	mu      sync.Mutex
	running bool
	latest  *Release
	err     string
}

func New(current string) *Checker { return &Checker{Current: current, URL: LatestURL} }

// Run checks now and then once a day until ctx ends.
func (c *Checker) Run(ctx context.Context) {
	c.mu.Lock()
	c.running = true
	c.mu.Unlock()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		c.Check(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (c *Checker) Check(ctx context.Context) {
	r, err := c.fetch(ctx)
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		c.err = err.Error()
		log.Printf("update check: %v", err)
		return
	}
	c.latest, c.err = r, ""
}

func (c *Checker) fetch(ctx context.Context) (*Release, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "campfire/"+c.Current)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub answered %s", res.Status)
	}
	var body struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, err
	}
	return &Release{Version: body.TagName, URL: body.HTMLURL, Checked: time.Now()}, nil
}

// Status is what the administration screen shows.
type Status struct {
	Current string   `json:"current"`
	Check   bool     `json:"check"` // updates.check
	Latest  *Release `json:"latest,omitempty"`
	Newer   bool     `json:"newer"`
	Error   string   `json:"error,omitempty"`
}

func (c *Checker) Status() Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := Status{Current: c.Current, Check: c.running, Latest: c.latest, Error: c.err}
	if c.latest != nil {
		s.Newer = Newer(c.latest.Version, c.Current)
	}
	return s
}

// Newer reports whether release is later than current. A build that is not a
// version at all ("dev") is never behind: it says nothing rather than guess.
func Newer(release, current string) bool {
	r, rPre, ok1 := parse(release)
	c, cPre, ok2 := parse(current)
	if !ok1 || !ok2 {
		return false
	}
	for i := range r {
		if r[i] != c[i] {
			return r[i] > c[i]
		}
	}
	// v1.2.0 is later than v1.2.0-rc1, and than a build ahead of v1.1.x,
	// which Go stamps v1.1.1-0.<time>-<commit>.
	return !rPre && cPre
}

func parse(v string) (n [3]int, pre bool, ok bool) {
	v, _, _ = strings.Cut(strings.TrimPrefix(v, "v"), "+")
	v, _, pre = strings.Cut(v, "-")
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return n, false, false
	}
	for i, p := range parts {
		x, err := strconv.Atoi(p)
		if err != nil {
			return n, false, false
		}
		n[i] = x
	}
	return n, pre, true
}

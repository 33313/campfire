// Package forge reads pull requests from where the code lives: GitHub,
// Forgejo (and Gitea, whose API it shares, and Codeberg, which runs it), and
// GitLab (DEV-114).
//
// An instance connects to a forge with an access token, which is what lets it
// see private repositories, and is told of changes by a webhook the
// administrator adds there with a secret this instance made. Sync reads pull
// requests back from the API for a forge that cannot reach the instance, or a
// delivery that went missing.
//
// Nothing here knows about Campfire's data. It turns forges into plain values.
package forge

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	GitHub  = "github"
	Forgejo = "forgejo" // and Gitea
	GitLab  = "gitlab"
)

// kind is what differs between the forges' APIs.
type kind struct {
	name   string // as people know it
	hosted string // its public instance's address
	root   string // its API's path under the address
	auth   string // its Authorization scheme
	repos  string // the repositories the token can see
	pulls  string // a repository's pull requests, newest update first; %s is its key
}

var kinds = map[string]kind{
	GitHub: {"GitHub", "https://github.com", "/api/v3", "Bearer ", "/user/repos?per_page=100",
		"/repos/%s/pulls?state=all&sort=updated&direction=desc&per_page=100"},
	Forgejo: {"Forgejo", "https://codeberg.org", "/api/v1", "token ", "/user/repos?limit=50",
		"/repos/%s/pulls?state=all&sort=recentupdate&limit=50"},
	GitLab: {"GitLab", "https://gitlab.com", "/api/v4", "Bearer ", "/projects?membership=true&simple=true&per_page=100",
		"/projects/%s/merge_requests?state=all&order_by=updated_at&sort=desc&per_page=100"},
}

// Hosted is the address of a kind's public instance: GitHub.com, Codeberg
// for Forgejo, and GitLab.com. False for a kind there is not.
func Hosted(k string) (string, bool) {
	kd, ok := kinds[k]
	return kd.hosted, ok
}

// Forge is one connection: where, and the credentials for it.
type Forge struct {
	Kind   string
	URL    string // the web address, https://codeberg.org
	Token  string
	Secret string // the webhook's
}

// api is the root of the forge's REST API. GitHub's own is on its own host;
// GitHub Enterprise's is under the web address.
func (f Forge) api() string {
	if f.Kind == GitHub && f.URL == kinds[GitHub].hosted {
		return "https://api.github.com"
	}
	return f.URL + kinds[f.Kind].root
}

// PullRequest is one pull request (a merge request, on GitLab), its state
// folded to one of open, draft, merged and closed.
type PullRequest struct {
	// Remote names it across every forge: the forge's address and its id
	// there, which survives renaming the repository.
	Remote    string
	Repo      string // owner/name
	Number    int64
	Title     string
	URL       string
	State     string
	Author    string
	UpdatedAt time.Time
}

type Client struct {
	HTTP *http.Client
}

func New() *Client {
	return &Client{HTTP: &http.Client{Timeout: 30 * time.Second}}
}

// Account is whom the token belongs to, which is also how a token is known
// to work.
func (c *Client) Account(ctx context.Context, f Forge) (string, error) {
	var out struct {
		Login    string `json:"login"`
		Username string `json:"username"` // GitLab
	}
	if _, err := c.do(ctx, f, f.api()+"/user", &out); err != nil {
		return "", err
	}
	if out.Login == "" {
		return out.Username, nil
	}
	return out.Login, nil
}

// PullRequests reads the pull requests updated since `since` in every
// repository the token can see. A repository it may list but not read pull
// requests in (a GitLab project with merge requests off, or where the token's
// user is a guest) is skipped; if every one is, the token is wrong for the job.
//
// ponytail: one request per repository and page; a token that sees thousands
// of repositories wants a search API instead, where the forge has one.
func (c *Client) PullRequests(ctx context.Context, f Forge, since time.Time) ([]PullRequest, error) {
	repos, err := c.repos(ctx, f)
	if err != nil {
		return nil, err
	}
	var out []PullRequest
	var denied error
	skipped := 0
	for _, r := range repos {
		next := f.api() + fmt.Sprintf(kinds[f.Kind].pulls, r.key)
		for next != "" {
			var page []PullRequest
			if f.Kind == GitLab {
				var mrs []mrJSON
				next, err = c.do(ctx, f, next, &mrs)
				for _, m := range mrs {
					page = append(page, m.value(f, r.name))
				}
			} else {
				var pulls []pullJSON
				next, err = c.do(ctx, f, next, &pulls)
				for _, p := range pulls {
					page = append(page, p.value(f, r.name))
				}
			}
			var se *statusError
			if errors.As(err, &se) && (se.status == 403 || se.status == 404) {
				denied = fmt.Errorf("%s: %w", r.name, err)
				skipped++
				break
			}
			if err != nil {
				return nil, fmt.Errorf("%s: %w", r.name, err)
			}
			for _, pr := range page {
				if pr.UpdatedAt.Before(since) {
					next = "" // sorted by update, so the rest are older still
					break
				}
				out = append(out, pr)
			}
		}
	}
	if skipped > 0 && skipped == len(repos) {
		return nil, denied
	}
	return out, nil
}

// repo is a repository's owner/name, and its key in the forge's API: GitLab
// names a project by its id, the others by owner/name.
type repo struct{ key, name string }

// repos lists what the token can see: on GitLab the projects it is a member
// of, not every public project on the instance.
func (c *Client) repos(ctx context.Context, f Forge) ([]repo, error) {
	next := f.api() + kinds[f.Kind].repos
	var out []repo
	for next != "" {
		var page []struct {
			ID       int64  `json:"id"`
			FullName string `json:"full_name"`
			Path     string `json:"path_with_namespace"` // GitLab
		}
		var err error
		if next, err = c.do(ctx, f, next, &page); err != nil {
			return nil, err
		}
		for _, r := range page {
			name := r.FullName + r.Path // one is empty
			key := name
			if f.Kind == GitLab {
				key = strconv.FormatInt(r.ID, 10)
			}
			out = append(out, repo{key, name})
		}
	}
	return out, nil
}

// pullJSON is a pull request as GitHub and Forgejo both write it.
type pullJSON struct {
	ID        int64      `json:"id"`
	Number    int64      `json:"number"`
	Title     string     `json:"title"`
	HTMLURL   string     `json:"html_url"`
	State     string     `json:"state"`
	Draft     bool       `json:"draft"`
	Merged    bool       `json:"merged"`
	MergedAt  *time.Time `json:"merged_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
}

func (p pullJSON) value(f Forge, repo string) PullRequest {
	state := p.State // open or closed
	switch {
	case p.Merged || p.MergedAt != nil:
		state = "merged"
	case state == "open" && p.Draft:
		state = "draft"
	}
	return PullRequest{Remote: remote(f, p.ID), Repo: repo, Number: p.Number, Title: p.Title,
		URL: p.HTMLURL, State: state, Author: p.User.Login, UpdatedAt: p.UpdatedAt}
}

// mrJSON is a GitLab merge request.
type mrJSON struct {
	ID             int64     `json:"id"`
	IID            int64     `json:"iid"`
	Title          string    `json:"title"`
	WebURL         string    `json:"web_url"`
	State          string    `json:"state"` // opened, closed, locked or merged
	Draft          bool      `json:"draft"`
	WorkInProgress bool      `json:"work_in_progress"`
	UpdatedAt      time.Time `json:"updated_at"`
	Author         struct {
		Username string `json:"username"`
	} `json:"author"`
}

func (m mrJSON) value(f Forge, repo string) PullRequest {
	state := "closed"
	switch {
	case m.State == "merged":
		state = "merged"
	case m.State == "opened" && (m.Draft || m.WorkInProgress):
		state = "draft"
	case m.State == "opened":
		state = "open"
	}
	return PullRequest{Remote: remote(f, m.ID), Repo: repo, Number: m.IID, Title: m.Title,
		URL: m.WebURL, State: state, Author: m.Author.Username, UpdatedAt: m.UpdatedAt}
}

func remote(f Forge, id int64) string { return f.URL + "#" + strconv.FormatInt(id, 10) }

// ErrSignature is a delivery that is not from the forge.
var ErrSignature = errors.New("the delivery's signature does not match")

// Delivery reads a webhook delivery: the pull request it is about, or nil
// for one this instance does not act on. GitHub and Forgejo sign the body
// with the secret; GitLab sends the secret itself.
//
// A GitLab delivery does not name a merge request's author, so it is read
// back from the API instead.
func (c *Client) Delivery(ctx context.Context, f Forge, h http.Header, body []byte) (*PullRequest, error) {
	if !verify(f, h, body) {
		return nil, ErrSignature
	}
	// Either forge can be told to post a form, with the JSON in it.
	if strings.HasPrefix(h.Get("Content-Type"), "application/x-www-form-urlencoded") {
		form, err := url.ParseQuery(string(body))
		if err != nil {
			return nil, err
		}
		body = []byte(form.Get("payload"))
	}

	if f.Kind == GitLab {
		var in struct {
			Kind    string `json:"object_kind"`
			Project struct {
				ID   int64  `json:"id"`
				Path string `json:"path_with_namespace"`
			} `json:"project"`
			MR struct {
				IID int64 `json:"iid"`
			} `json:"object_attributes"`
		}
		if err := json.Unmarshal(body, &in); err != nil {
			return nil, err
		}
		if in.Kind != "merge_request" {
			return nil, nil
		}
		var m mrJSON
		if _, err := c.do(ctx, f, fmt.Sprintf("%s/projects/%d/merge_requests/%d", f.api(), in.Project.ID, in.MR.IID), &m); err != nil {
			return nil, err
		}
		pr := m.value(f, in.Project.Path)
		return &pr, nil
	}

	if !strings.HasPrefix(first(h, "X-GitHub-Event", "X-Forgejo-Event", "X-Gitea-Event"), "pull_request") {
		return nil, nil
	}
	var in struct {
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
		PullRequest *pullJSON `json:"pull_request"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		return nil, err
	}
	if in.PullRequest == nil || in.PullRequest.ID == 0 {
		return nil, nil
	}
	pr := in.PullRequest.value(f, in.Repository.FullName)
	return &pr, nil
}

// verify checks a delivery against the forge's secret. Nothing verifies
// against an empty one: anyone can sign with an empty key.
func verify(f Forge, h http.Header, body []byte) bool {
	if f.Secret == "" {
		return false
	}
	if f.Kind == GitLab {
		return subtle.ConstantTimeCompare([]byte(h.Get("X-Gitlab-Token")), []byte(f.Secret)) == 1
	}
	mac := hmac.New(sha256.New, []byte(f.Secret))
	mac.Write(body)
	want := mac.Sum(nil)
	sig := h.Get("X-Hub-Signature-256")
	if sig == "" {
		sig = "sha256=" + first(h, "X-Forgejo-Signature", "X-Gitea-Signature")
	}
	got, err := hex.DecodeString(strings.TrimPrefix(sig, "sha256="))
	return err == nil && strings.HasPrefix(sig, "sha256=") && hmac.Equal(got, want)
}

// first is the first of the headers that is set. Forgejo sends its own and,
// for tools written for them, Gitea's and GitHub's too.
func first(h http.Header, names ...string) string {
	for _, n := range names {
		if v := h.Get(n); v != "" {
			return v
		}
	}
	return ""
}

// identifier is SOL-42 as a word: a short code (letters and digits, not
// ending in a digit) and a number.
var identifier = regexp.MustCompile(`^[A-Za-z0-9]{0,7}[A-Za-z]-[1-9][0-9]*$`)

// Identifiers reads the item identifiers a pull request's title names: a run
// of them at its start or its end, each bare or in brackets.
//
//	DEV-12: fix the thing          [DEV-12] fix the thing
//	fix the thing DEV-12           fix the thing (DEV-12)
//	[DEV-12][DEV-13] two at once   DEV-12, DEV-13: two at once
//
// One in the middle of the title is prose about it, not a claim to be it.
func Identifiers(title string) []string {
	words := strings.FieldsFunc(title, func(r rune) bool { return r == ' ' || r == '\t' || r == ',' })
	var out []string
	seen := map[string]bool{}
	take := func(w string) bool {
		w = strings.TrimRight(w, ":.")
		var found []string
		for _, part := range strings.Split(strings.ReplaceAll(w, "][", "] ["), " ") {
			part = strings.TrimSuffix(strings.TrimPrefix(part, "["), "]")
			part = strings.TrimSuffix(strings.TrimPrefix(part, "("), ")")
			if !identifier.MatchString(part) {
				return false
			}
			found = append(found, strings.ToUpper(part))
		}
		for _, id := range found {
			if !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
		return true
	}
	i := 0
	for i < len(words) && take(words[i]) {
		i++
	}
	for j := len(words) - 1; j >= i && take(words[j]); j-- {
	}
	return out
}

// do sends one GET and decodes the answer into out. It returns the next
// page's address, from the Link header, when there is one.
func (c *Client) do(ctx context.Context, f Forge, url string, out any) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", kinds[f.Kind].auth+f.Token)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 32<<20))
	if err != nil {
		return "", err
	}
	if res.StatusCode >= 300 {
		var e struct {
			Message any    `json:"message"` // GitLab's is sometimes an object
			Error   string `json:"error"`
		}
		json.Unmarshal(body, &e)
		msg := res.Status
		if s, ok := e.Message.(string); ok && s != "" {
			msg = s
		} else if e.Error != "" {
			msg = e.Error
		}
		status := res.StatusCode
		if res.Header.Get("X-RateLimit-Remaining") == "0" || res.Header.Get("Retry-After") != "" {
			status = http.StatusTooManyRequests // GitHub says so with a 403
		}
		return "", &statusError{status, kinds[f.Kind].name + ": " + msg}
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(out); err != nil {
		return "", err
	}
	return nextPage(res.Header.Get("Link")), nil
}

// statusError is a forge's refusal, with its HTTP status.
type statusError struct {
	status int
	msg    string
}

func (e *statusError) Error() string { return e.msg }

var linkNext = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

func nextPage(link string) string {
	if m := linkNext.FindStringSubmatch(link); m != nil {
		return m[1]
	}
	return ""
}

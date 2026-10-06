// Package github talks to GitHub as a GitHub App (DEV-13).
//
// An instance creates its own app from a manifest, so GitHub hands it the
// key and the webhook secret and nobody copies them by hand. Installing the
// app on an account or an organisation, on all of its repositories or some,
// is what lets the instance see them, private ones included. Pull requests
// arrive by webhook; Sync reads them back from the API for an instance GitHub
// cannot reach, or a delivery that went missing.
//
// Nothing here knows about Campfire's data. It turns GitHub into plain values.
package github

import (
	"bytes"
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
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

type Client struct {
	API  string // https://api.github.com
	Web  string // https://github.com
	HTTP *http.Client
}

func New() *Client {
	return &Client{API: "https://api.github.com", Web: "https://github.com",
		HTTP: &http.Client{Timeout: 30 * time.Second}}
}

// App is what the manifest conversion hands back, secrets included.
type App struct {
	ID            int64
	Slug          string
	HTMLURL       string
	Owner         string
	PEM           string
	WebhookSecret string
}

type Installation struct {
	ID          int64  `json:"id"`
	Account     string `json:"account"`
	AccountType string `json:"accountType"` // User or Organization
}

// PullRequest is one pull request, its state folded to one of open, draft,
// merged and closed.
type PullRequest struct {
	ID             int64
	InstallationID int64
	Repo           string // owner/name
	Number         int64
	Title          string
	URL            string
	State          string
	Author         string
	UpdatedAt      time.Time
}

// Manifest is the app an instance asks GitHub to create: read access to pull
// requests, and their events delivered to base. It is public so that it can
// be installed on more than the account that owns it; an installation the
// instance has not linked is ignored.
func Manifest(base, name string) map[string]any {
	return map[string]any{
		"name":         name,
		"url":          base,
		"public":       true,
		"redirect_url": base + "/api/github/manifest",
		"setup_url":    base + "/api/github/setup",
		"hook_attributes": map[string]any{
			"url": base + "/api/github/webhook",
		},
		"default_permissions": map[string]string{"metadata": "read", "pull_requests": "read"},
		"default_events":      []string{"pull_request"},
	}
}

// NewAppURL is where the manifest is posted, under an organisation when one
// is named and under the signed-in user otherwise.
func (c *Client) NewAppURL(org, state string) string {
	path := "/settings/apps/new"
	if org != "" {
		path = "/organizations/" + url.PathEscape(org) + "/settings/apps/new"
	}
	return c.Web + path + "?state=" + url.QueryEscape(state)
}

// InstallURL is where an account or organisation installs the app.
func (c *Client) InstallURL(slug, state string) string {
	return c.Web + "/apps/" + url.PathEscape(slug) + "/installations/new?state=" + url.QueryEscape(state)
}

// ConvertManifest exchanges the code GitHub redirects back with for the app
// it created. The code is good once, for an hour.
func (c *Client) ConvertManifest(ctx context.Context, code string) (App, error) {
	var out struct {
		ID            int64  `json:"id"`
		Slug          string `json:"slug"`
		HTMLURL       string `json:"html_url"`
		PEM           string `json:"pem"`
		WebhookSecret string `json:"webhook_secret"`
		Owner         struct {
			Login string `json:"login"`
		} `json:"owner"`
	}
	_, err := c.do(ctx, http.MethodPost, c.API+"/app-manifests/"+url.PathEscape(code)+"/conversions", "", &out)
	if err != nil {
		return App{}, err
	}
	if out.PEM == "" || out.WebhookSecret == "" {
		return App{}, errors.New("GitHub created the app without a key or a webhook secret")
	}
	return App{ID: out.ID, Slug: out.Slug, HTMLURL: out.HTMLURL, Owner: out.Owner.Login,
		PEM: out.PEM, WebhookSecret: out.WebhookSecret}, nil
}

type installationJSON struct {
	ID      int64 `json:"id"`
	Account struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"account"`
}

func (i installationJSON) value() Installation {
	return Installation{ID: i.ID, Account: i.Account.Login, AccountType: i.Account.Type}
}

// Installation reads one installation of the app, which is how a returning
// installation id is known to be the app's and not made up.
func (c *Client) Installation(ctx context.Context, app App, id int64) (Installation, error) {
	jwt, err := appJWT(app)
	if err != nil {
		return Installation{}, err
	}
	var out installationJSON
	_, err = c.do(ctx, http.MethodGet, c.API+"/app/installations/"+strconv.FormatInt(id, 10), "Bearer "+jwt, &out)
	return out.value(), err
}

// Installations lists every account and organisation the app is installed on.
func (c *Client) Installations(ctx context.Context, app App) ([]Installation, error) {
	jwt, err := appJWT(app)
	if err != nil {
		return nil, err
	}
	var out []Installation
	for next := c.API + "/app/installations?per_page=100"; next != ""; {
		var page []installationJSON
		if next, err = c.do(ctx, http.MethodGet, next, "Bearer "+jwt, &page); err != nil {
			return nil, err
		}
		for _, i := range page {
			out = append(out, i.value())
		}
	}
	return out, nil
}

// PullRequests reads the pull requests updated since `since` in every
// repository the installation can see.
//
// ponytail: one request per repository and page; an installation on
// thousands of repositories wants the search API instead.
func (c *Client) PullRequests(ctx context.Context, app App, installation int64, since time.Time) ([]PullRequest, error) {
	jwt, err := appJWT(app)
	if err != nil {
		return nil, err
	}
	var tok struct {
		Token string `json:"token"`
	}
	if _, err := c.do(ctx, http.MethodPost,
		c.API+"/app/installations/"+strconv.FormatInt(installation, 10)+"/access_tokens", "Bearer "+jwt, &tok); err != nil {
		return nil, err
	}
	auth := "token " + tok.Token

	var repos []string
	for next := c.API + "/installation/repositories?per_page=100"; next != ""; {
		var page struct {
			Repositories []struct {
				FullName string `json:"full_name"`
			} `json:"repositories"`
		}
		if next, err = c.do(ctx, http.MethodGet, next, auth, &page); err != nil {
			return nil, err
		}
		for _, r := range page.Repositories {
			repos = append(repos, r.FullName)
		}
	}

	var out []PullRequest
	for _, repo := range repos {
		next := c.API + "/repos/" + repo + "/pulls?state=all&sort=updated&direction=desc&per_page=100"
		for next != "" {
			var page []pullJSON
			if next, err = c.do(ctx, http.MethodGet, next, auth, &page); err != nil {
				return nil, fmt.Errorf("%s: %w", repo, err)
			}
			for _, p := range page {
				pr := p.value(installation, repo)
				if pr.UpdatedAt.Before(since) {
					next = "" // sorted by update, so the rest are older still
					break
				}
				out = append(out, pr)
			}
		}
	}
	return out, nil
}

type pullJSON struct {
	ID        int64      `json:"id"`
	Number    int64      `json:"number"`
	Title     string     `json:"title"`
	HTMLURL   string     `json:"html_url"`
	State     string     `json:"state"`
	Draft     bool       `json:"draft"`
	MergedAt  *time.Time `json:"merged_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
}

func (p pullJSON) value(installation int64, repo string) PullRequest {
	state := p.State // open or closed
	switch {
	case p.MergedAt != nil:
		state = "merged"
	case state == "open" && p.Draft:
		state = "draft"
	}
	return PullRequest{ID: p.ID, InstallationID: installation, Repo: repo, Number: p.Number,
		Title: p.Title, URL: p.HTMLURL, State: state, Author: p.User.Login, UpdatedAt: p.UpdatedAt}
}

// Event is a webhook delivery, reduced to what the instance acts on. At most
// one of PullRequest and Installation is set; neither means it is ignored.
type Event struct {
	InstallationID int64
	PullRequest    *PullRequest
	// Removed is set when the app was uninstalled from an account.
	Removed bool
}

// ParseEvent reads a delivery whose signature has been checked. `kind` is the
// X-GitHub-Event header.
func ParseEvent(kind string, body []byte) (Event, error) {
	var in struct {
		Action       string `json:"action"`
		Installation struct {
			ID int64 `json:"id"`
		} `json:"installation"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
		PullRequest *pullJSON `json:"pull_request"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		return Event{}, err
	}
	e := Event{InstallationID: in.Installation.ID}
	switch {
	case kind == "pull_request" && in.PullRequest != nil:
		pr := in.PullRequest.value(e.InstallationID, in.Repository.FullName)
		e.PullRequest = &pr
	case kind == "installation" && in.Action == "deleted":
		e.Removed = true
	}
	return e, nil
}

// Verify checks a delivery's X-Hub-Signature-256 against the webhook secret.
func Verify(secret string, body []byte, signature string) bool {
	got, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil || !strings.HasPrefix(signature, "sha256=") {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hmac.Equal(got, mac.Sum(nil))
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

// appJWT is the app's own credential: ten minutes, signed with its key, good
// for asking about installations and for minting an installation's token.
func appJWT(app App) (string, error) {
	block, _ := pem.Decode([]byte(app.PEM))
	if block == nil {
		return "", errors.New("the app's private key is not PEM")
	}
	var key *rsa.PrivateKey
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		key = k
	} else if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		key, _ = k.(*rsa.PrivateKey)
	}
	if key == nil {
		return "", errors.New("the app's private key is not an RSA key")
	}
	now := time.Now().Unix()
	enc := base64.RawURLEncoding
	header := enc.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	// Issued a minute early, for a clock that runs behind GitHub's.
	claims, _ := json.Marshal(map[string]any{"iat": now - 60, "exp": now + 540, "iss": strconv.FormatInt(app.ID, 10)})
	signing := header + "." + enc.EncodeToString(claims)
	sum := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return signing + "." + enc.EncodeToString(sig), nil
}

// do sends one request and decodes the answer into out. It returns the next
// page's address, from the Link header, when there is one.
func (c *Client) do(ctx context.Context, method, url, auth string, out any) (string, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
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
			Message string `json:"message"`
		}
		json.Unmarshal(body, &e)
		if e.Message == "" {
			e.Message = res.Status
		}
		return "", fmt.Errorf("GitHub: %s", e.Message)
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(out); err != nil {
		return "", err
	}
	return nextPage(res.Header.Get("Link")), nil
}

var linkNext = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

func nextPage(link string) string {
	if m := linkNext.FindStringSubmatch(link); m != nil {
		return m[1]
	}
	return ""
}

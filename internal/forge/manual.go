package forge

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// PullTarget derives an API target only from a known hosted forge or a
// connection an administrator configured. The submitted link is never fetched.
func PullTarget(f Forge, link *url.URL) (string, int64, bool) {
	base, err := url.Parse(f.URL)
	if err != nil || base.Scheme != link.Scheme || !strings.EqualFold(base.Host, link.Host) {
		return "", 0, false
	}
	path, ok := strings.CutPrefix(strings.TrimRight(link.Path, "/"), strings.TrimRight(base.Path, "/")+"/")
	if !ok {
		return "", 0, false
	}
	parts := strings.Split(path, "/")
	if len(parts) < 4 {
		return "", 0, false
	}
	last := len(parts) - 1
	n, err := strconv.ParseInt(parts[last], 10, 64)
	if err != nil || n <= 0 || strconv.FormatInt(n, 10) != parts[last] {
		return "", 0, false
	}
	var repo []string
	switch {
	case f.Kind == GitLab && len(parts) >= 5 && parts[last-2] == "-" && parts[last-1] == "merge_requests":
		repo = parts[:last-2]
	case len(parts) == 4 && (f.Kind == GitHub && parts[2] == "pull" || f.Kind == Forgejo && parts[2] == "pulls"):
		repo = parts[:2]
	default:
		return "", 0, false
	}
	for _, part := range repo {
		if part == "" || part == "." || part == ".." || strings.ContainsAny(part, `\%`) {
			return "", 0, false
		}
	}
	return strings.Join(repo, "/"), n, true
}

func (c *Client) PullRequestByNumber(ctx context.Context, f Forge, repo string, number int64) (PullRequest, error) {
	// Do not leave the trusted API host on a redirect from a repository.
	httpClient := *c.HTTP
	httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || req.URL.Scheme != via[0].URL.Scheme || req.URL.Host != via[0].URL.Host {
			return http.ErrUseLastResponse
		}
		return nil
	}
	safe := Client{HTTP: &httpClient}
	suffix := "/" + strconv.FormatInt(number, 10)
	if f.Kind == GitLab {
		var pr mrJSON
		_, err := safe.do(ctx, f, f.api()+"/projects/"+url.PathEscape(repo)+"/merge_requests"+suffix, &pr)
		return pr.value(f, repo), err
	}
	parts := strings.Split(repo, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	var pr pullJSON
	_, err := safe.do(ctx, f, f.api()+"/repos/"+strings.Join(parts, "/")+"/pulls"+suffix, &pr)
	return pr.value(f, repo), err
}

package forge

import (
	"net/url"
	"testing"
)

func TestPullTargetUsesTrustedForgeAndRepository(t *testing.T) {
	for _, tc := range []struct{ kind, base, link, want string }{
		{GitHub, "https://github.com", "https://github.com/acme/app/pull/17", "acme/app"},
		{Forgejo, "https://forge.example/code", "https://forge.example/code/acme/app/pulls/17", "acme/app"},
		{GitLab, "https://gitlab.com", "https://gitlab.com/group/team/app/-/merge_requests/17", "group/team/app"},
		{GitHub, "https://github.com", "https://github.com.example/acme/app/pull/17", ""},
		{GitHub, "https://github.com", "https://github.com/%2e%2e/app/pull/17", ""},
	} {
		u, err := url.Parse(tc.link)
		if err != nil {
			t.Fatal(err)
		}
		got, n, ok := PullTarget(Forge{Kind: tc.kind, URL: tc.base}, u)
		if got != tc.want || ok != (tc.want != "") || (ok && n != 17) {
			t.Errorf("%s: %q, %d, %v; want %q", tc.link, got, n, ok, tc.want)
		}
	}
}

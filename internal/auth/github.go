package auth

import (
	"net/url"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// readGitHub files the tokens the GitHub CLI and GitHub's own runners would send
// to a GitHub REST API, each for its own host's API and no other, as gh chooses
// them:
//
//   - github.com's API (api.github.com): `GH_TOKEN`, else `GITHUB_TOKEN`, else
//     the `oauth_token` of github.com in gh's hosts.yml. On a runner of a GitHub
//     Enterprise Server `GITHUB_API_URL` names that instance, and the runner's
//     `GITHUB_TOKEN` is that instance's: it goes there, not to github.com.
//   - an Enterprise Server instance `GITHUB_API_URL` or `GH_HOST` names:
//     `GH_ENTERPRISE_TOKEN`, else `GITHUB_ENTERPRISE_TOKEN`, else (for
//     GITHUB_API_URL's) `GITHUB_TOKEN`, else hosts.yml's token for the host;
//   - any other host of hosts.yml: its own token.
//
// An Enterprise Server's token is limited to the paths of its API (/api/v3/),
// github.com's and a data-residency host's (api.<name>.ghe.com) are their API
// hosts'. The token gh keeps in the system's keyring (its default) is not read,
// and `gh auth token` is not run.
//
// Implements: REQ-AUTH-036
func (c *Store) readGitHub(m userconf.Machine) {
	hosts := map[string]string{}
	for _, h := range m.GitHubCLIHosts() {
		hosts[h.Host] = h.Token
	}
	instance := ""
	if u, err := url.Parse(strings.TrimSpace(m.Environment("GITHUB_API_URL"))); err == nil && u.Host != "" &&
		!strings.EqualFold(u.Hostname(), "api.github.com") {
		instance = strings.TrimRight(u.String(), "/")
	}
	github := first(m.Environment("GH_TOKEN"), hosts["github.com"])
	if instance == "" {
		github = first(m.Environment("GH_TOKEN"), m.Environment("GITHUB_TOKEN"), hosts["github.com"])
	}
	c.fileGitHub(userconf.GitHubAPI("github.com"), github)
	enterprise := first(m.Environment("GH_ENTERPRISE_TOKEN"), m.Environment("GITHUB_ENTERPRISE_TOKEN"))
	done := map[string]bool{"github.com": true}
	if instance != "" {
		host := strings.ToLower(hostOfAPI(instance))
		done[host] = true
		c.fileGitHub(instance, first(enterprise, m.Environment("GITHUB_TOKEN"), hosts[host]))
	}
	if host := strings.ToLower(strings.TrimSpace(m.Environment("GH_HOST"))); host != "" && !done[host] {
		done[host] = true
		c.fileGitHub(userconf.GitHubAPI(host), first(enterprise, hosts[host]))
	}
	for host, token := range hosts {
		if !done[host] {
			c.fileGitHub(userconf.GitHubAPI(host), token)
		}
	}
}

// fileGitHub files a token for a GitHub API: for the whole host when the API is
// at its root, else for the API's path alone.
func (c *Store) fileGitHub(api, token string) {
	if token == "" || api == "" {
		return
	}
	u, err := url.Parse(api)
	if err != nil || u.Host == "" {
		return
	}
	c.notePlain(u)
	c.file(u.Host, pathPrefix(u.Path), secret{bearer: true, value: token}, true)
}

// hostOfAPI is the GitHub host an API URL serves: github.com for api.github.com,
// <name>.ghe.com for api.<name>.ghe.com, else the URL's own host.
func hostOfAPI(api string) string {
	u, err := url.Parse(api)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	if host == "api.github.com" {
		return "github.com"
	}
	if strings.HasSuffix(host, ".ghe.com") {
		return strings.TrimPrefix(host, "api.")
	}
	return host
}

// first is the first of values that is not empty.
func first(values ...string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

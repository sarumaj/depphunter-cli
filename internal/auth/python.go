package auth

import (
	"maps"
	"net/url"
	"slices"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/pyconf"
	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// readPython reads the index credentials of this machine's uv, Poetry and PDM
// configuration, each for an index this machine's own configuration names:
//
//   - uv: UV_INDEX_<NAME>_USERNAME and _PASSWORD for the index of that name in the
//     user's or the system's uv.toml, or in UV_INDEX / UV_DEFAULT_INDEX (name=url);
//   - Poetry: the http-basic credential of a repository its config.toml or a
//     POETRY_REPOSITORIES_<NAME>_URL variable defines (config.toml, auth.toml, then
//     POETRY_HTTP_BASIC_<NAME>_USERNAME/_PASSWORD);
//   - PDM: the username and password of [pypi] and [pypi.<name>] in its global
//     config.toml, PDM_PYPI_USERNAME and PDM_PYPI_PASSWORD over [pypi]'s.
//
// Each goes to its index's path (see pythonPrefix), over what netrc holds for the
// host. The credential of an index only the repository names is the repository's
// choice of destination and is lent by internal/index when this machine vouches
// for it (LendIndex).
//
// Implements: REQ-AUTH-026, REQ-AUTH-020
func (c *Store) readPython(m userconf.Machine) {
	put := func(index string, cred pyconf.Credential) {
		if cred.Password == "" && cred.Username == "" {
			return
		}
		u, err := url.Parse(strings.TrimSpace(index))
		if err != nil || u.Host == "" {
			return
		}
		c.notePlain(u)
		c.file(u.Host, pythonPrefix(u.Path), secret{value: cred.Username + ":" + cred.Password}, true)
	}
	pdm := pyconf.PDMMachine(m)
	for _, i := range pdm.Indexes {
		index := i.URL
		if index == "" && i.Name == pyconf.PDMPyPI {
			continue // PyPI itself: nothing private to log in to
		}
		put(index, pyconf.Credential{Username: i.Username, Password: i.Password})
	}
	repos, creds := pyconf.PoetryMachine(m)
	for _, name := range slices.Sorted(maps.Keys(repos)) {
		put(repos[name], creds[name])
	}
	for _, i := range pyconf.UVMachine(m) {
		user, pass := pyconf.UVCredential(m.Env, i.Name)
		put(i.URL, pyconf.Credential{Username: user, Password: pass})
	}
}

// LendIndex files the credential of a Python index the repository names while this
// machine supplies the secret - a UV_INDEX_<NAME>_PASSWORD, a Poetry http-basic
// entry, a PDM [pypi.<name>] password or a ${VAR} in the index URL - once the
// caller has established that this machine vouches for the index
// (internal/index). It serves the index's path only; a credential this machine
// already holds there is kept.
//
// Implements: REQ-AUTH-023, REQ-AUTH-026
func (c *Store) LendIndex(index, user, password string) {
	if c == nil || user == "" && password == "" {
		return
	}
	u, err := url.Parse(strings.TrimSpace(index))
	if err != nil || u.Host == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.file(u.Host, pythonPrefix(u.Path), secret{value: user + ":" + password}, false)
}

// pythonPrefix is the path a Python index's credential serves: the index's path
// without its /simple suffix (devpi's /+simple), where the JSON API
// (<prefix>/pypi/<name>/json), the simple pages and, on most indexes, the files
// and their PEP 658 metadata all live - GitLab's /api/v4/projects/<id>/packages/pypi/,
// Nexus's /repository/<name>/, devpi's /<user>/<index>/ - and "" (the whole host)
// for an index at the host's root. A file served from another path or host gets
// whatever is filed for that one.
func pythonPrefix(p string) string {
	p = strings.TrimRight(p, "/")
	p = strings.TrimSuffix(p, "/simple")
	p = strings.TrimSuffix(p, "/+simple")
	return pathPrefix(p)
}

package auth

import (
	"net/url"
	"os"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/npmconf"
	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// readYarn takes the credentials of this machine's Yarn Berry configuration
// (userconf.YarnConfigs: the files of the directories above the analyzed one
// outside its checkout, then the home directory's, merged key by key as Yarn merges
// them), with the YARN_NPM_REGISTRY_SERVER, YARN_NPM_AUTH_TOKEN,
// YARN_NPM_AUTH_IDENT and YARN_NPM_ALWAYS_AUTH variables over its top level: an
// npmAuthToken is sent as a Bearer token and an npmAuthIdent as Basic credentials,
// to the registry each belongs to, and only with the requests Yarn sends it with
// (see npmconf.Settings.YarnCredentials: without npmAlwaysAuth, those for scoped
// packages). ${VAR} references are resolved from this machine's environment; a
// value naming an unset variable without a fallback is dropped, as Yarn refuses it.
//
// What npm's configuration, read first, holds for a registry wins: a Yarn
// credential for the same registry path is not filed at all, whichever packages it
// would have served.
//
// A project's .yarnrc.yml is never read here: internal/index lends the machine's
// secret it refers to only to a registry this machine vouches for (REQ-AUTH-023).
//
// Implements: REQ-AUTH-024
func (c *Store) readYarn(m userconf.Machine) {
	var files [][]byte
	for _, name := range m.YarnConfigs() {
		if data, err := os.ReadFile(name); err == nil {
			files = append(files, data)
		}
	}
	s, _ := npmconf.MergeYarnrc(files)
	s.YarnEnvironment(m.Environment)
	resolve := func(v string) string {
		out, ok := npmconf.Interpolate(v, m.Environment)
		if !ok {
			return ""
		}
		return out
	}
	var entries []npmconf.Entry
	for _, e := range s.YarnCredentials(resolve) {
		e.URL, e.Token, e.Ident = resolve(e.URL), resolve(e.Token), resolve(e.Ident)
		entries = append(entries, e)
		if strings.TrimRight(e.URL, "/") == npmconf.YarnDefault {
			// The default registry is npm's, under Yarn's name for it.
			e.URL = npmconf.BunDefault
			entries = append(entries, e)
		}
	}
	held := make([]bool, len(entries))
	for i, e := range entries {
		if u, err := url.Parse(strings.TrimSpace(e.URL)); err == nil {
			held[i] = c.holds(u.Host, pathPrefix(u.Path))
		}
	}
	for i, e := range entries {
		if !held[i] {
			c.registry(e)
		}
	}
}

// readBun takes the credentials of Bun's global bunfig (userconf.BunConfig): the
// token, or the username and password, of [install] registry and of each
// [install.scopes] entry, written in a table or into the URL. $VAR and ${VAR} are
// resolved from this machine's environment.
//
// A project's bunfig.toml is never read here (see readYarn).
//
// Implements: REQ-AUTH-024
func (c *Store) readBun(m userconf.Machine) {
	name := m.BunConfig()
	if name == "" {
		return
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return
	}
	s, ok := npmconf.ParseBunfig(data)
	if !ok {
		return
	}
	for _, e := range s.Credentials(npmconf.BunDefault) {
		for _, v := range []*string{&e.URL, &e.Token, &e.Username, &e.Password} {
			*v = os.Expand(*v, m.Environment)
		}
		c.registry(e)
	}
}

// registry files an npm-family registry's credential under the registry's host and
// path (see pathPrefix), limited to the packages the entry says (see
// packagePrefix), unless one is already there: npm's configuration, read first,
// wins.
func (c *Store) registry(e npmconf.Entry) {
	u, err := url.Parse(strings.TrimSpace(e.URL))
	if err != nil || u.Host == "" {
		return
	}
	c.notePlain(u)
	if e.Token != "" {
		c.file(u.Host, packagePrefix(u.Path, e.Packages), secret{bearer: true, value: e.Token}, false)
	} else if pair, ok := e.Basic(); ok {
		c.file(u.Host, packagePrefix(u.Path, e.Packages), secret{value: pair}, false)
	}
}

// packagePrefix is the path prefix of the requests a registry's credential is sent
// with: the registry's path (pathPrefix) for every package; that path followed by
// "@" for the scoped packages only, or by "@scope/" for one scope's. A package's
// metadata is asked for at the registry's path followed by its name, so the
// longest prefix (see Apply) gives a scope's own credential precedence over the
// registry's, as Yarn does.
func packagePrefix(registryPath, packages string) string {
	prefix := pathPrefix(registryPath)
	if packages == "" {
		return prefix
	}
	if prefix == "" {
		prefix = "/"
	}
	if packages != "@" {
		packages += "/"
	}
	return prefix + packages
}

// holds reports whether a credential is filed for a host at exactly this path
// prefix ("" meaning the whole host).
func (c *Store) holds(host, prefix string) bool {
	if prefix != "" {
		_, ok := c.scoped[host][prefix]
		return ok
	}
	_, bearer := c.bearer[host]
	_, basic := c.basic[host]
	return bearer || basic
}

// LendRegistry files the credential of an npm-family registry that the repository
// names while this machine's environment supplies the secret - a .yarnrc.yml
// `npmAuthToken: "${NPM_TOKEN}"`, a bunfig.toml `token = "$NPM_TOKEN"` - once the
// caller has established that the registry is one this machine vouches for
// (internal/index). The credential serves the registry's path only (host-wide for
// a registry at the root of its host), with the requests packages allows (see
// packagePrefix); one this machine already holds for the registry is kept. bearer
// says the value is a token; otherwise it is "user:password".
//
// Implements: REQ-AUTH-023
func (c *Store) LendRegistry(registry, packages string, bearer bool, value string) {
	if c == nil || value == "" {
		return
	}
	u, err := url.Parse(strings.TrimSpace(registry))
	if err != nil || u.Host == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if packages != "" && c.holds(u.Host, pathPrefix(u.Path)) {
		return
	}
	c.file(u.Host, packagePrefix(u.Path, packages), secret{bearer: bearer, value: value}, false)
}

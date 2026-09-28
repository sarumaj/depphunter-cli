package auth

import (
	"net/url"
	"os"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/npmconf"
	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// readYarn takes the credentials of Yarn Berry's configuration in the home
// directory (userconf.YarnUserConfig), with the YARN_NPM_REGISTRY_SERVER,
// YARN_NPM_AUTH_TOKEN and YARN_NPM_AUTH_IDENT variables over its top level: an
// npmAuthToken is sent as a Bearer token and an npmAuthIdent as Basic credentials,
// to the registry each belongs to (see npmconf.Settings.Credentials). ${VAR}
// references are resolved from this machine's environment; a value naming an unset
// variable without a fallback is dropped, as Yarn refuses it.
//
// A project's .yarnrc.yml is never read here: internal/index lends the machine's
// secret it refers to only to a registry this machine vouches for (REQ-AUTH-023).
//
// Implements: REQ-AUTH-024
func (c *Store) readYarn(m userconf.Machine) {
	var s npmconf.Settings
	if name := m.YarnUserConfig(); name != "" {
		if data, err := os.ReadFile(name); err == nil {
			s, _ = npmconf.ParseYarnrc(data)
		}
	}
	for v, field := range map[string]*string{
		"YARN_NPM_REGISTRY_SERVER": &s.Registry.URL,
		"YARN_NPM_AUTH_TOKEN":      &s.Registry.Token,
		"YARN_NPM_AUTH_IDENT":      &s.Registry.Ident,
	} {
		if value := m.Environment(v); value != "" {
			*field = value
		}
	}
	resolve := func(v string) string {
		out, ok := npmconf.Interpolate(v, m.Environment)
		if !ok {
			return ""
		}
		return out
	}
	for _, e := range s.Credentials(npmconf.YarnDefault) {
		e.URL, e.Token, e.Ident = resolve(e.URL), resolve(e.Token), resolve(e.Ident)
		c.registry(e)
		if strings.TrimRight(e.URL, "/") == npmconf.YarnDefault {
			// The default registry is npm's, under Yarn's name for it.
			e.URL = npmconf.BunDefault
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
// path (see pathPrefix), unless one is already there: npm's configuration, read
// first, wins.
func (c *Store) registry(e npmconf.Entry) {
	u, err := url.Parse(strings.TrimSpace(e.URL))
	if err != nil || u.Host == "" {
		return
	}
	c.notePlain(u)
	if e.Token != "" {
		c.file(u.Host, pathPrefix(u.Path), secret{bearer: true, value: e.Token}, false)
	} else if pair, ok := e.Basic(); ok {
		c.file(u.Host, pathPrefix(u.Path), secret{value: pair}, false)
	}
}

// LendRegistry files the credential of an npm-family registry that the repository
// names while this machine's environment supplies the secret - a .yarnrc.yml
// `npmAuthToken: "${NPM_TOKEN}"`, a bunfig.toml `token = "$NPM_TOKEN"` - once the
// caller has established that the registry is one this machine vouches for
// (internal/index). The credential serves the registry's path only (host-wide for
// a registry at the root of its host); one this machine already holds there is
// kept. bearer says the value is a token; otherwise it is "user:password".
//
// Implements: REQ-AUTH-023
func (c *Store) LendRegistry(registry string, bearer bool, value string) {
	if c == nil || value == "" {
		return
	}
	u, err := url.Parse(strings.TrimSpace(registry))
	if err != nil || u.Host == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.file(u.Host, pathPrefix(u.Path), secret{bearer: bearer, value: value}, false)
}

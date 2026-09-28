package index

import (
	"maps"
	"net/url"
	"os"
	"slices"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/npmconf"
	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// machineYarnBun reads the registries of this machine's Yarn and Bun
// configuration, after npm's (whose registry, when it names one, stays the
// replacement asked): Yarn Berry's file in the home directory, with
// YARN_NPM_REGISTRY_SERVER over its npmRegistryServer and ${VAR} references
// resolved from the environment; Yarn 1's ~/.yarnrc; Bun's global bunfig, with
// $VAR resolved. Each names a default registry and the registry of each scope.
//
// Implements: REQ-SUP-015, REQ-SUP-064
func machineYarnBun(m userconf.Machine, k sink) {
	read := func(name string) []byte {
		if name == "" {
			return nil
		}
		data, _ := os.ReadFile(name)
		return data
	}
	berry, _ := npmconf.ParseYarnrc(read(m.YarnUserConfig()))
	if v := m.Env("YARN_NPM_REGISTRY_SERVER"); v != "" {
		berry.Registry.URL = v
	}
	addNpmSettings(berry, k.add, func(v string) string {
		out, ok := npmconf.Interpolate(v, m.Env)
		if !ok {
			return ""
		}
		return out
	})
	if data := read(m.YarnClassicConfig()); data != nil {
		addNpmSettings(npmconf.ParseYarnClassic(data), k.add, nil)
	}
	if bun, ok := npmconf.ParseBunfig(read(m.BunConfig())); ok {
		addNpmSettings(bun, k.add, func(v string) string { return os.Expand(v, m.Env) })
	}
}

// addNpmSettings records the default registry and the scope registries of one
// Yarn or Bun file. resolve fills a value's variable references; nil leaves a
// value that has any unused, as the repository's are: its variables are this
// machine's, and would be drawn on the map.
func addNpmSettings(s npmconf.Settings, add func(eco, url, scope string), resolve func(string) string) {
	value := func(v string) string {
		if resolve != nil {
			return resolve(v)
		}
		if strings.Contains(v, "$") {
			return ""
		}
		return v
	}
	add(NPM, value(s.Registry.URL), "")
	for _, scope := range slices.Sorted(maps.Keys(s.Scopes)) {
		add(NPM, value(s.Scopes[scope].URL), scope)
	}
}

// yarnRCFilename is the name of Yarn Berry's configuration files on this machine.
func (c *Config) yarnRCFilename() string {
	if c.m.Env == nil {
		return ".yarnrc.yml"
	}
	return c.m.YarnRCFilename()
}

// projectYarnrc records the registries of a repository's .yarnrc.yml, and lends
// the credentials it binds to them whose secret is this machine's: an
// npmAuthToken or npmAuthIdent that is exactly ${NAME} (or ${NAME:-fallback}
// with NAME set) - see lendNpm. A token or ident written out, or one only its
// fallback fills, is the repository's and is discarded.
//
// Implements: REQ-SUP-015, REQ-AUTH-023
func (c *Config) projectYarnrc(data []byte, add func(eco, url, scope string)) {
	s, ok := npmconf.ParseYarnrc(data)
	if !ok {
		return
	}
	addNpmSettings(s, add, nil)
	for _, e := range s.Credentials(npmconf.YarnDefault) {
		if token, ok := c.secretOf(e.Token); ok {
			c.lendNpm(e.URL, true, token)
		} else if ident, ok := c.secretOf(e.Ident); ok {
			if pair, ok := (npmconf.Entry{Ident: ident}).Basic(); ok {
				c.lendNpm(e.URL, false, pair)
			}
		}
	}
}

// projectBunfig records the registries of a repository's bunfig.toml, and lends
// the credentials whose token or password is exactly $NAME or ${NAME} (a user
// name may be written out) - see lendNpm. A token or password written out,
// in a table or in the URL, is discarded.
//
// Implements: REQ-SUP-015, REQ-AUTH-023
func (c *Config) projectBunfig(data []byte, add func(eco, url, scope string)) {
	s, ok := npmconf.ParseBunfig(data)
	if !ok {
		return
	}
	addNpmSettings(s, add, nil)
	for _, e := range s.Credentials(npmconf.BunDefault) {
		if token, ok := c.secretOf(e.Token); ok {
			c.lendNpm(e.URL, true, token)
		} else if pass, ok := c.secretOf(e.Password); ok && e.Username != "" {
			user := e.Username
			if name, _ := npmconf.Reference(user); name != "" {
				user = c.m.Env(name)
			}
			if user != "" {
				c.lendNpm(e.URL, false, user+":"+pass)
			}
		}
	}
}

// secretOf is the value of the machine's variable that v refers to and consists
// of, when it is set; ok is false for anything written out, including a
// fallback.
func (c *Config) secretOf(v string) (string, bool) {
	name, _ := npmconf.Reference(v)
	if name == "" || c.m.Env == nil {
		return "", false
	}
	value := c.m.Env(name)
	return value, value != ""
}

// lendNpm hands the credential store a credential for a registry the repository
// names, when this machine vouches for it: the user did with --trust-index, or
// this machine's own npm, Yarn or Bun configuration names a registry on its host.
// Anything else would let the repository choose where this machine's secret goes.
//
// Implements: REQ-AUTH-023
func (c *Config) lendNpm(registry string, bearer bool, value string) {
	if value == "" || c.credentials == nil || strings.Contains(registry, "$") {
		return
	}
	u, err := url.Parse(strings.TrimSpace(registry))
	if err != nil || u.Host == "" {
		return
	}
	if c.vouched(NPM, registry, u) {
		c.credentials.LendRegistry(registry, bearer, value)
	}
}

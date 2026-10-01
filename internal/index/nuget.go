package index

import (
	"net/url"
	"os"
	"path"
	"slices"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang/nuget"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// projectNuGet reads the repository's nuget.config files, closest first: NuGet
// reads the file of a project's own directory before those of the directories above
// it, so a deeper file overrides a shallower one. Files in sibling directories each
// apply to their own projects in NuGet; here, one configuration serves the whole
// repository, and they are merged in path order.
func projectNuGet(files []*scan.File) []nuget.ConfigFile {
	var found []*scan.File
	for _, f := range files {
		if strings.EqualFold(path.Base(f.Path), "nuget.config") {
			found = append(found, f)
		}
	}
	sort.SliceStable(found, func(i, j int) bool {
		return strings.Count(found[i].Path, "/") > strings.Count(found[j].Path, "/")
	})
	var out []nuget.ConfigFile
	for _, f := range found {
		if data, err := os.ReadFile(f.AbsolutePath); err == nil {
			if config, ok := nuget.ParseConfig(data); ok {
				out = append(out, config)
			}
		}
	}
	return out
}

// nugetAbove reads the nuget.config files of the directories between the analyzed
// directory and the top of its checkout (userconf.Machine.DirectoriesAbove),
// closest first: NuGet reads them after the project's own, and they are the
// repository's.
//
// Implements: REQ-SUP-065, REQ-SUP-079
func (c *Config) nugetAbove() []nuget.ConfigFile {
	repository, _ := c.m.DirectoriesAbove()
	checkout := checkoutRoot(repository)
	var out []nuget.ConfigFile
	for _, directory := range repository {
		if name := c.m.NuGetConfigIn(directory); name != "" {
			if data, ok := checkout.ReadBounded(name); ok {
				if config, ok := nuget.ParseConfig(data); ok {
					out = append(out, config)
				}
			}
		}
	}
	return out
}

// applyNuGet merges the repository's nuget.config files (project, closest first)
// over this machine's and records what the merge says, replacing what an earlier
// call recorded: the enabled package sources, asked beside nuget.org (this
// machine's trusted, the repository's not); whether nuget.org is switched off; and
// the packageSourceMapping that candidates follows (nugetRouted). A package no
// pattern covers is asked as if there were no mapping - NuGet itself would not
// restore it at all.
//
// A source is this machine's when the URL it ends up with came from this machine's
// files. A repository that redefines a machine source's key with another URL makes
// it the repository's source, which is not fetched from unless vouched for.
//
// Implements: REQ-SUP-015, REQ-SUP-063, REQ-SUP-065
func (c *Config) applyNuGet(project []nuget.ConfigFile) {
	c.sources[NuGet] = slices.DeleteFunc(c.sources[NuGet], func(s Source) bool { return s.nugetKey != "" })
	delete(c.off, NuGet)
	s := nuget.Merge(append(slices.Clone(project), c.nugetMachine...))
	c.nuget = s
	for _, f := range s.Feeds {
		u, official, ok := nugetFeed(f.URL)
		if !ok || official || !s.Enabled(f.Key) {
			continue
		}
		source := Source{URL: u, Kind: Additive, nugetKey: f.Key, Origin: OriginProject}
		if f.Layer >= len(project) {
			source.Trusted, source.Origin = true, OriginMachine
		}
		c.Add(NuGet, source)
	}
	if nugetOff(s) {
		origin := OriginProject
		if nugetOff(nuget.Merge(c.nugetMachine)) {
			origin = OriginMachine
		}
		c.SwitchOff(NuGet, origin)
	}
	c.lendNuGet(s, len(project))
}

// nugetOff reports whether a merged NuGet configuration leaves nuget.org out: no
// enabled source is nuget.org, and either a <clear/> dropped the nuget.org NuGet
// otherwise assumes, a source naming nuget.org is disabled, or the key "nuget.org"
// is.
func nugetOff(s nuget.Settings) bool {
	listed := false
	for _, f := range s.Feeds {
		if _, official, ok := nugetFeed(f.URL); ok && official {
			if s.Enabled(f.Key) {
				return false
			}
			listed = true
		}
	}
	return s.Cleared || listed || !s.Enabled("nuget.org")
}

// nugetRouted are the candidates of a package packageSourceMapping sends to the
// sources of keys: each enabled one, nuget.org as the public default. A key no
// enabled source has adds nothing, and a package mapped only to such keys is asked
// of nothing - NuGet would not restore it from anywhere, and the public index is
// the last place its name should go.
//
// Implements: REQ-SUP-065
func (c *Config) nugetRouted(keys []string) []candidate {
	var out []candidate
	seen := map[string]bool{}
	for _, key := range keys {
		if !c.nuget.Enabled(key) {
			continue
		}
		var u string
		if f, ok := c.nuget.Lookup(key); ok {
			feed, official, ok := nugetFeed(f.URL)
			switch {
			case !ok:
				continue
			case official:
				u = public[NuGet]
			default:
				u = c.recorded(feed)
			}
		} else if strings.EqualFold(key, "nuget.org") && !c.nuget.Cleared {
			u = public[NuGet] // the source NuGet assumes, under the name it gives it
		} else {
			continue
		}
		if seen[u] {
			continue
		}
		seen[u] = true
		k := candidate{url: u, primary: true, known: u == public[NuGet] || c.trusted[u]}
		for _, s := range c.sources[NuGet] {
			if s.URL == u && c.fetchable(NuGet, s) {
				k.known = true
			}
		}
		out = append(out, k)
	}
	return out
}

// recorded is an index URL as Add records it: trimmed, without a trailing slash or
// a credential.
func (c *Config) recorded(u string) string {
	return c.credentials.FromURL(strings.TrimRight(strings.TrimSpace(u), "/"), false)
}

// lendNuGet hands the credential store the credential of each enabled source the
// repository's nuget.config defines (sources, with the first project layers being
// the repository's), when the secret comes from this machine - a
// NuGetPackageSourceCredentials_ variable, a %NAME% password, or a machine file's
// entry for the key - and the feed is one this machine vouches for (see lend).
// A password the repository writes out is discarded.
//
// Implements: REQ-AUTH-023
func (c *Config) lendNuGet(s nuget.Settings, project int) {
	variables := c.m.NuGetCredentialVariables()
	for _, f := range s.Feeds {
		if f.Layer >= project || !s.Enabled(f.Key) {
			continue
		}
		key := strings.ToLower(f.Key)
		if user, pass, ok := nuget.EnvironmentCredential(variables[key]); ok {
			c.lend(f.URL, user, pass)
			continue
		}
		credential, ok := s.Credentials[key]
		if !ok || !credential.Basic() {
			continue
		}
		user, _ := c.fromEnvironment(credential.Username)
		pass, reference := c.fromEnvironment(credential.ClearTextPassword)
		if reference || credential.Layer >= project {
			c.lend(f.URL, user, pass)
		}
	}
}

// lendPaket hands the credential store the credentials of paket.dependencies'
// sources whose password is a %NAME% reference to this machine's environment, for
// the feeds this machine vouches for (see lend). A user name may be written out; a
// password written out is discarded, and an authtype other than basic is not sent.
//
// Implements: REQ-AUTH-023
func (c *Config) lendPaket(data []byte) {
	_, sources := nuget.ParseDependencies(data)
	for _, s := range sources {
		if s.AuthType != "" && !strings.EqualFold(s.AuthType, "basic") {
			continue
		}
		if pass, reference := c.fromEnvironment(s.Password); reference {
			user, _ := c.fromEnvironment(s.Username)
			c.lend(strings.Trim(s.URL, `"`), user, pass)
		}
	}
}

// fromEnvironment resolves a value that is exactly %NAME% from this machine's environment
// (ref true); any other value is returned as written.
func (c *Config) fromEnvironment(v string) (value string, reference bool) {
	v = strings.TrimSpace(v)
	name, ok := strings.CutPrefix(v, "%")
	if name, ok2 := strings.CutSuffix(name, "%"); ok && ok2 && name != "" && !strings.Contains(name, "%") {
		if c.m.Environment == nil {
			return "", true
		}
		return c.m.Environment(name), true
	}
	return v, false
}

// lend files a credential the repository's configuration binds to a feed while this
// machine supplies the secret. It is sent only to a feed this machine vouches for:
// one on the host of a NuGet source this machine's own configuration names, or one
// the user vouched for with --trust-index (by URL or host). Anything else would let
// the repository choose where this machine's secret is sent.
//
// Implements: REQ-AUTH-023
func (c *Config) lend(feed, user, pass string) {
	if pass != "" && c.mayLend(NuGet, feed) {
		c.credentials.Lend(feed, user, pass)
	}
}

// mayLend reports whether a credential may be lent for a feed the repository names:
// there is a store to lend to, and the feed parses, names a host, and is one this
// machine vouches for (vouched). Every lending path asks it first.
//
// Implements: REQ-AUTH-023
func (c *Config) mayLend(ecosystem, feed string) bool {
	if c.credentials == nil {
		return false
	}
	u, err := url.Parse(strings.TrimSpace(feed))
	return err == nil && u.Host != "" && c.vouched(ecosystem, feed, u)
}

// knownRegistry reports whether a registry a package names by its host is known:
// the user vouched for it by URL or by host, or this machine holds a credential for
// the host (credential, asked only when neither vouches).
func (c *Config) knownRegistry(registry, host string, credential func(host string) bool) bool {
	return c.trusted[registry] || c.trusted[host] || credential(host)
}

// vouched reports whether this machine vouches for a feed the repository names:
// the user did with --trust-index (by URL or host), or this machine's own
// configuration names a source of the ecosystem on the feed's host (for PyPI, a
// Poetry repository or an explicit uv index there counts too).
func (c *Config) vouched(ecosystem, feed string, u *url.URL) bool {
	if c.trusted[c.recorded(feed)] || c.trusted[u.Host] || ecosystem == PyPI && c.py.hosts[strings.ToLower(u.Host)] {
		return true
	}
	for _, s := range c.sources[ecosystem] {
		if m, err := url.Parse(s.URL); err == nil && s.Trusted && strings.EqualFold(m.Host, u.Host) {
			return true
		}
	}
	return false
}

// nugetUnmapped reports whether packageSourceMapping is in force and covers no
// pattern of id. NuGet then refuses to restore the package (NU1100); candidates
// asks the sources as if there were no mapping, which the report notes.
//
// Implements: REQ-SUP-065, REQ-TRC-017
func (c *Config) nugetUnmapped(id string) bool {
	if !c.nuget.Mapped() {
		return false
	}
	_, ok := c.nuget.Route(id)
	return !ok
}

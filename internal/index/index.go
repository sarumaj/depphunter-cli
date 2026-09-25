// Package index works out where a dependency comes from: the package index or mirror
// that would serve it, read from this machine's configuration and from the
// repository's own (an .npmrc naming an internal registry, a --extra-index-url).
//
// What the repository carries answers that question and nothing else. Fetching, when
// --online allows it, goes only to the indexes this machine configures: a repository
// pointing at an index nobody here configured is the shape a dependency-confusion
// attack takes, and the map says so rather than following it.
package index

import (
	"net/url"
	"slices"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/auth"
	"github.com/sarumaj/depphunter-cli/internal/trace"
)

// Ecosystem ids, as the language plugins emit them.
const (
	NPM   = "npm"
	PyPI  = "pypi"
	Go    = "go"
	Cargo = "crates"
	Maven = "maven"
	NuGet = "nuget"
	OCI   = "oci"
)

// public is where each ecosystem's packages come from unless something says otherwise.
var public = map[string]string{
	NPM:   "https://registry.npmjs.org",
	PyPI:  "https://pypi.org/simple",
	Go:    "https://proxy.golang.org",
	Cargo: "https://crates.io",
	Maven: "https://repo.maven.apache.org/maven2",
	NuGet: "https://api.nuget.org/v3/index.json",
	OCI:   "https://registry-1.docker.io",
}

// Where an index was learned from. It decides nothing on its own - Trusted does
// that - but it is the first thing anyone reading the resolution report wants to
// know about an index they did not expect to see.
const (
	OriginMachine = "this machine"    // the environment, or a config file in $HOME
	OriginProject = "the repository"  // a file the repository carries
	OriginVouched = "--trust-index"   // the repository's, and the user vouched for it
	OriginPublic  = "public default"  // nothing named one, so the ecosystem's own
	OriginImage   = "image reference" // an image reference carries its own registry
)

// Source is one index, and what it serves.
type Source struct {
	URL string
	// Scope limits the source to the packages it names: an npm scope ("@acme"), a
	// Maven groupId prefix, or a distribution name. Empty serves everything.
	Scope string
	// Trusted marks a source this machine's own configuration named. A source the
	// repository declares is not trusted: it says where a package came from, and
	// nothing is ever fetched from it.
	Trusted bool
	// Origin is where the source was learned from, for the resolution report. It is
	// one of the Origin constants above.
	Origin string
}

// Config is the index configuration of one analysis: what this machine knows, and
// what the repository asks for.
type Config struct {
	sources map[string][]Source // ecosystem -> sources, the repository's last
	// trusted holds the indexes the user vouched for themselves (--trust-index), for
	// the organization whose repositories carry their own .npmrc naming the company
	// registry: without it every one of its packages is marked, and a warning that
	// is always on is a warning nobody reads.
	trusted map[string]bool
	// credentials receives what an index URL carries in it - the form a private pip
	// or Cargo mirror is usually configured with - and is what the URL is stripped
	// of before it is recorded. nil strips it and keeps nothing.
	credentials *auth.Store
}

func New() *Config { return &Config{sources: map[string][]Source{}, trusted: map[string]bool{}} }

// Credentials is where a credential written into an index URL is filed. It is set
// before anything is discovered, since a URL is stripped as it arrives.
func (c *Config) Credentials(s *auth.Store) { c.credentials = s }

// Trust vouches for index URLs whatever names them. It is the user's own say-so, from
// their config or the command line; a repository cannot reach it (see config.go).
//
// Implements: REQ-SUP-042
func (c *Config) Trust(urls []string) {
	for _, u := range urls {
		if u = strings.TrimRight(strings.TrimSpace(u), "/"); u != "" {
			c.trusted[u] = true
		}
	}
}

// Public reports whether an index is the ecosystem's own public one - registry.npmjs.org,
// proxy.golang.org, Docker Hub. It is what decides whether naming a package to it
// would tell the world that the package exists.
//
// Implements: REQ-SUP-038
func (c *Config) Public(eco, index string) bool {
	return index != "" && index == public[eco]
}

// Add records a source for an ecosystem. Sources added first are preferred, which is
// why the machine's own configuration is read before the repository's.
//
// A URL carrying a credential is stripped of it here, always: the index a package
// resolves from is drawn on the map, named in the side panel and written into every
// export, and an export is a file this project's documentation suggests sharing. The
// credential is kept only where this machine's own configuration supplied it.
//
// Implements: REQ-SUP-016, REQ-AUTH-013
func (c *Config) Add(eco string, s Source) {
	if s.URL == "" {
		return
	}
	s.URL = strings.TrimRight(strings.TrimSpace(s.URL), "/")
	if s.URL == "" {
		return
	}
	s.URL = c.credentials.FromURL(s.URL, s.Trusted)
	for _, have := range c.sources[eco] {
		if have.URL == s.URL && have.Scope == s.Scope {
			return
		}
	}
	c.sources[eco] = append(c.sources[eco], s)
}

// forgetProject drops what the repository declared, before it is read again.
func (c *Config) forgetProject() {
	for eco, sources := range c.sources {
		c.sources[eco] = slices.DeleteFunc(sources, func(s Source) bool { return s.Origin == OriginProject })
	}
}

// Sources lists what was found for an ecosystem, machine first.
func (c *Config) Sources(eco string) []Source { return c.sources[eco] }

// Report lists every index this configuration holds, plus the public default of
// each ecosystem that has one and is not already named, for internal/trace. It is
// the answer to "which indexes did this run even know about", which is the question
// underneath every surprising index on the map.
//
// Implements: REQ-TRC-002
func (c *Config) Report() []trace.Source {
	ecosystems := make([]string, 0, len(c.sources))
	for eco := range c.sources {
		ecosystems = append(ecosystems, eco)
	}
	sort.Strings(ecosystems)
	var out []trace.Source
	for _, eco := range ecosystems {
		for _, s := range c.sources[eco] {
			out = append(out, trace.Source{
				Ecosystem: eco, URL: s.URL, Scope: s.Scope,
				Origin: c.origin(s), Trusted: c.fetchable(eco, s),
			})
		}
		if url := public[eco]; url != "" && !c.has(eco, url) {
			out = append(out, trace.Source{Ecosystem: eco, URL: url, Origin: OriginPublic, Trusted: true})
		}
	}
	return out
}

// has reports whether an ecosystem already names an index, so the public default is
// not reported twice.
func (c *Config) has(eco, url string) bool {
	for _, s := range c.sources[eco] {
		if s.URL == url {
			return true
		}
	}
	return false
}

// origin says where a source was learned from: this machine, the repository, or the
// repository with the user vouching for it afterwards.
//
// Implements: REQ-TRC-002, REQ-SUP-042
func (c *Config) origin(s Source) string {
	if !s.Trusted && c.trusted[s.URL] {
		return OriginVouched
	}
	if s.Origin != "" {
		return s.Origin
	}
	if s.Trusted {
		return OriginMachine
	}
	return OriginProject
}

// fetchable reports whether anything here allows fetching from a source - the same
// judgment For makes, which is why it is written once.
//
// Implements: REQ-SUP-019, REQ-SUP-042
func (c *Config) fetchable(eco string, s Source) bool {
	return s.Trusted || s.URL == public[eco] || c.trusted[s.URL]
}

// For reports which index serves a package, and whether anything here vouches for it:
// a public default or a source this machine's configuration names is known, a source
// only the repository asks for is not.
//
// Implements: REQ-SUP-014, REQ-SUP-016, REQ-SUP-018
func (c *Config) For(eco, pkg string) (index string, known bool) {
	if eco == OCI {
		// A container reference carries its registry: "ghcr.io/org/app" is not
		// "app" from Docker Hub. Nothing needs to be configured to see that.
		return ociRegistry(pkg), ociRegistry(pkg) == public[OCI]
	}
	scoped, plain := Source{}, Source{}
	for _, s := range c.sources[eco] {
		switch {
		case s.Scope != "" && matches(eco, s.Scope, pkg):
			if scoped.URL == "" {
				scoped = s
			}
		case s.Scope == "":
			if plain.URL == "" {
				plain = s
			}
		}
	}
	best := scoped
	if best.URL == "" {
		best = plain
	}
	if best.URL == "" {
		return public[eco], public[eco] != ""
	}
	return best.URL, c.fetchable(eco, best)
}

// matches reports whether a scope covers a package name. npm scopes are exact, Maven
// groups match by prefix, and everything else is a name.
func matches(eco, scope, pkg string) bool {
	switch eco {
	case NPM:
		return strings.HasPrefix(pkg, scope+"/")
	case Maven:
		return pkg == scope || strings.HasPrefix(pkg, scope+".")
	}
	return strings.EqualFold(scope, pkg)
}

// ociRegistry reads the registry out of an image reference. A first segment with a dot
// or a port is a host; anything else is a Docker Hub image, official or not.
//
// Implements: REQ-SUP-017
func ociRegistry(image string) string {
	first, _, ok := strings.Cut(image, "/")
	if !ok || (!strings.Contains(first, ".") && !strings.Contains(first, ":") && first != "localhost") {
		return public[OCI]
	}
	return "https://" + first
}

// Host shortens an index URL to what identifies it on screen.
func Host(index string) string {
	u, err := url.Parse(index)
	if err != nil || u.Host == "" {
		return index
	}
	return u.Host
}

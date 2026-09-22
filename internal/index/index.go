// Package index works out where a dependency comes from: the package index or mirror
// that would serve it. A repository often carries that configuration itself - an
// .npmrc naming an internal registry, a requirements file with --extra-index-url - and
// reading it is how the map can say "this one is ours" and "this one is the public
// one".
//
// The configuration a repository carries is read for that answer only. Fetching, when
// --online allows it, goes to the indexes this machine's own configuration names: a
// repository that points at an index nobody here configured is the shape a dependency
// confusion attack takes, and the map says so rather than following it.
package index

import (
	"net/url"
	"strings"
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
}

func New() *Config { return &Config{sources: map[string][]Source{}, trusted: map[string]bool{}} }

// Trust vouches for index URLs whatever names them. It is the user's own say-so, from
// their config or the command line; a repository cannot reach it (see config.go).
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
func (c *Config) Public(eco, index string) bool {
	return index != "" && index == public[eco]
}

// Add records a source for an ecosystem. Sources added first are preferred, which is
// why the machine's own configuration is read before the repository's.
func (c *Config) Add(eco string, s Source) {
	if s.URL == "" {
		return
	}
	s.URL = strings.TrimRight(strings.TrimSpace(s.URL), "/")
	if s.URL == "" {
		return
	}
	for _, have := range c.sources[eco] {
		if have.URL == s.URL && have.Scope == s.Scope {
			return
		}
	}
	c.sources[eco] = append(c.sources[eco], s)
}

// Sources lists what was found for an ecosystem, machine first.
func (c *Config) Sources(eco string) []Source { return c.sources[eco] }

// For reports which index serves a package, and whether anything here vouches for it:
// a public default or a source this machine's configuration names is known, a source
// only the repository asks for is not.
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
	return best.URL, best.Trusted || best.URL == public[eco] || c.trusted[best.URL]
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

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
	"github.com/sarumaj/depphunter-cli/internal/lang"
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
	// Composer is PHP's; its public index is Packagist.
	Composer = "composer"
	// RubyGems is Ruby's, as Bundler installs from it.
	RubyGems = "rubygems"
	// Pub is Dart's and Flutter's; its public index is pub.dev.
	Pub = "pub"
	// Hex is Elixir's and Erlang's, as Mix and rebar3 install from it; its index
	// is hex.pm's API.
	Hex = "hex"
	// CRAN is R's, as install.packages and renv install from it; its package
	// metadata is read from crandb (see cranPackage).
	CRAN = "cran"
	// Hackage is Haskell's, as cabal and stack install from it.
	Hackage = "hackage"
	// TerraformModule is the Terraform plugin's island of modules; the public
	// Terraform Registry serves them, and a module named with a host is served by
	// that host's registry (see For).
	TerraformModule = "terraform-module"
	// CocoaPods is the objc plugin's island of pods; its public index is the
	// CocoaPods CDN, which serves the trunk spec repository as files.
	CocoaPods = "cocoapods"
	// LuaRocks is the lua plugin's island of rocks; its public index is the
	// luarocks.org rocks server.
	LuaRocks = "luarocks"
	// CPAN is the perl plugin's island of distributions; they are read from the
	// MetaCPAN API, which describes every CPAN release.
	CPAN = "cpan"
	// Opam is the ocaml plugin's island of opam packages; opam-repository's
	// package descriptions are read from its git repository as files.
	Opam = "opam"
	// Julia is the julia plugin's island of Pkg packages; a registry's package
	// files (Versions.toml, Deps.toml, Compat.toml) are read from its git
	// repository.
	Julia = "julia"
	// Bazel is the bazel plugin's island of Bzlmod modules; a registry (the Bazel
	// Central Registry by default) serves each version's MODULE.bazel as a file.
	Bazel = "bazel"
	// Elm is the elm plugin's island of packages, named author/name; the package
	// site serves each version's elm.json and a package's releases as files.
	Elm = "elm"
	// PureScript is the purescript plugin's island of registry packages; the
	// registry's metadata and its index of manifests are git repositories read
	// as files.
	PureScript = "purescript"
	// Dub is the d plugin's island of dub packages; the registry's API serves each
	// package's versions with their recipes.
	Dub = "dub"
	// Alire is the ada plugin's island of Alire crates; the community index is
	// a git repository of release manifests, read as files.
	Alire = "alire"
	// Quicklisp is the commonlisp plugin's island of Quicklisp projects; a
	// dist's system index (systems.txt) lists every system with its
	// dependencies as one text file.
	Quicklisp = "quicklisp"
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
	// Composer: the metadata host Composer itself reads; packagist.org is its website.
	Composer: "https://repo.packagist.org",
	RubyGems: "https://rubygems.org",
	Pub:      "https://pub.dev",
	Hex:      "https://hex.pm/api",
	CRAN:     "https://cloud.r-project.org",
	Hackage:  "https://hackage.haskell.org",
	// Terraform's public registry; OpenTofu's registry.opentofu.org serves the same
	// namespaces, and the plugin names modules of either without a host.
	TerraformModule: "https://registry.terraform.io",
	CocoaPods:       "https://cdn.cocoapods.org",
	LuaRocks:        "https://luarocks.org",
	CPAN:            "https://fastapi.metacpan.org",
	// opam.ocaml.org serves opam-repository as an archive and its /packages pages
	// as HTML; the repository's own files are served one by one here.
	Opam: "https://raw.githubusercontent.com/ocaml/opam-repository/master",
	// The General registry; pkg.julialang.org serves registries only as tarballs.
	Julia: "https://raw.githubusercontent.com/JuliaRegistries/General/master",
	Bazel: "https://bcr.bazel.build",
	Elm:   "https://package.elm-lang.org",
	// The owner of the registry's repositories: registry (metadata/) and
	// registry-index (the manifests) are read below it.
	PureScript: "https://raw.githubusercontent.com/purescript",
	Dub:        "https://code.dlang.org",
	// The community index's branch for the index format Alire 2 reads; its
	// manifests are served one by one.
	Alire: "https://raw.githubusercontent.com/alire-project/alire-index/stable-1.4.0",
	// The Quicklisp dist's current distinfo; a dated version's is beside it
	// (quicklisp/<version>/distinfo.txt).
	Quicklisp: "https://beta.quicklisp.org/dist/quicklisp.txt",
}

// Clojars is the Maven repository Clojure's libraries are published to. Leiningen,
// tools.deps and babashka search it after Maven Central without being told to, so a
// repository with a Clojure manifest has it as a public index of its own (see
// Config.clojure).
const Clojars = "https://repo.clojars.org"

// clojarsURL is where Clojars is asked; tests point it at their own server.
var clojarsURL = Clojars

// MavenPublic reports whether a Maven repository URL is Maven Central (any of its
// hosts) or Clojars, which Clojure manifests name as often as a private one: they
// are public indexes, not ones the repository brings along.
//
// Implements: REQ-SUP-056
func MavenPublic(repo string) bool {
	u, err := url.Parse(strings.TrimSpace(repo))
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "repo.maven.apache.org", "repo1.maven.org", "central.maven.org", "repo.clojars.org":
		return true
	case "clojars.org":
		return strings.HasPrefix(u.Path, "/repo")
	}
	return false
}

// BazelCentral reports whether a registry a .bazelrc names is the Bazel Central
// Registry itself: the public index, not one the repository brings along.
//
// Implements: REQ-SUP-057
func BazelCentral(registry string) bool {
	u, err := url.Parse(strings.TrimSpace(registry))
	return err == nil && strings.EqualFold(u.Hostname(), "bcr.bazel.build")
}

// HackageItself reports whether a repository URL is Hackage (any scheme, with or
// without a trailing slash), which cabal configurations name as often as they name a
// mirror: it is the public index, not one the repository brings along.
func HackageItself(index string) bool {
	u, err := url.Parse(strings.TrimSpace(index))
	return err == nil && strings.EqualFold(u.Hostname(), "hackage.haskell.org")
}

// CocoaPodsTrunk reports whether a spec repository a Podfile or Podfile.lock names is
// CocoaPods' own public one: the CDN, the trunk spec repository on GitHub it serves,
// or "trunk" as Podfile.lock calls it. It is the public index, not one the
// repository brings along.
//
// Implements: REQ-SUP-051
func CocoaPodsTrunk(repo string) bool {
	r := strings.TrimSuffix(strings.TrimRight(strings.ToLower(strings.TrimSpace(repo)), "/"), ".git")
	switch r {
	case "trunk", "https://cdn.cocoapods.org", "https://github.com/cocoapods/specs", "git@github.com:cocoapods/specs":
		return true
	}
	return false
}

// LuaRocksItself reports whether a rocks server a LuaRocks configuration names is
// luarocks.org itself (its root or its /dev and /manifests/<user> manifests): the
// public index, not one the repository or machine brings along.
//
// Implements: REQ-SUP-052
func LuaRocksItself(server string) bool {
	u, err := url.Parse(strings.TrimSpace(server))
	return err == nil && strings.EqualFold(u.Hostname(), "luarocks.org")
}

// CRANMirror reports whether an R repository URL is CRAN itself - cloud.r-project.org,
// cran.r-project.org and its *.r-project.org mirrors, RStudio's - or Posit Package
// Manager's copy of it, which renv projects name as often as CRAN. Such a repository
// is the public index, not one the repository brings along, and its packages are what
// crandb describes. Other mirrors are not guessed from their names: an internal
// repository called cran.corp.example would have its packages named to crandb.
//
// Implements: REQ-SUP-048
func CRANMirror(index string) bool {
	u, err := url.Parse(strings.TrimSpace(index))
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	switch {
	case host == "cran.rstudio.com", host == "r-project.org", strings.HasSuffix(host, ".r-project.org"):
		return true
	case host == "packagemanager.posit.co", host == "packagemanager.rstudio.com", host == "p3m.dev":
		// Posit Package Manager serves CRAN as /cran/<snapshot> and CRAN with
		// Bioconductor as /all/<snapshot>.
		seg := strings.Split(strings.Trim(u.Path, "/"), "/")[0]
		return seg == "cran" || seg == "all"
	}
	return false
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

// Kind is how a source relates to the ecosystem's public default: whether the
// package manager asks it instead of the default or beside it.
type Kind uint8

const (
	// Replace serves every package instead of the public default: npm's
	// `registry`, pip's `index-url`, a Cargo `replace-with`, a Maven mirror of
	// Central. The first one found is the one used. It is the zero value.
	Replace Kind = iota
	// ReplaceAll is a Replace that also stands in for every additive source: a
	// Maven mirror of `*`, through which the repositories a POM declares are
	// fetched as well.
	ReplaceAll
	// Additive serves packages beside the public default (or the Replace source):
	// pip's `extra-index-url`, the repositories of a POM, a Gradle build or a
	// Clojure manifest, a Composer repository, a NuGet feed. The package manager
	// asks it too, so a package it lacks is still found on the default.
	Additive
	// Listed is an entry of an ordered list that replaces the public default
	// entry by entry: GOPROXY. Every entry is asked in turn, and nothing that is
	// not on the list.
	Listed
)

// Source is one index, and what it serves.
type Source struct {
	URL string
	// Scope limits the source to the packages it names: an npm scope ("@acme"), a
	// Maven groupId prefix, or a distribution name. Empty serves everything.
	Scope string
	// Kind says whether an unscoped source replaces the public default or is asked
	// beside it (see For). A scoped source serves its packages alone whatever it
	// says.
	Kind Kind
	// Registry names a Cargo alternative registry (`[registries.<name>]`): the
	// source serves only the crates that declare it, by that name in Cargo.toml or
	// by its index URL in Cargo.lock (lang.Target.Registry), and nothing else.
	Registry string
	// OnError moves on to the next Listed source when this one fails in any way,
	// not only when it does not have the package: GOPROXY's "|" separator.
	OnError bool
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
	// clojure says the repository has a Clojure manifest (deps.edn, project.clj,
	// shadow-cljs.edn, bb.edn, build.boot), whose tools search Clojars after Maven
	// Central: Maven artifacts are then asked of both.
	clojure bool
	// credentials receives what an index URL carries in it - the form a private pip
	// or Cargo mirror is usually configured with - and is what the URL is stripped
	// of before it is recorded. nil strips it and keeps nothing.
	credentials *auth.Store
	// off holds the ecosystems whose public default is switched off, and by whom
	// (an Origin): Composer's `"packagist.org": false`, NuGet's `<clear/>`, a
	// GOPROXY that is set. Only the sources named are asked then.
	off map[string]string
	// private reports a package the organization owns (internal/scope). Such a
	// package is attributed to the source that would serve it rather than to a
	// public index it is never named to.
	private func(eco, pkg string) bool
}

func New() *Config {
	return &Config{sources: map[string][]Source{}, trusted: map[string]bool{}, off: map[string]string{}}
}

// Private tells the configuration which packages are the organization's own (see
// For). nil makes every package public.
func (c *Config) Private(match func(eco, pkg string) bool) { c.private = match }

// SwitchOff records that the public default of an ecosystem is not used: only the
// sources named are. origin is where that was said, so that under --watch what the
// repository said is forgotten with its sources.
//
// Implements: REQ-SUP-063
func (c *Config) SwitchOff(eco, origin string) {
	if c.off == nil {
		c.off = map[string]string{}
	}
	if _, done := c.off[eco]; !done {
		c.off[eco] = origin
	}
}

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
// proxy.golang.org, Docker Hub, and Clojars beside Maven Central. It is what decides
// whether naming a package to it would tell the world that the package exists.
//
// Implements: REQ-SUP-038
func (c *Config) Public(eco, index string) bool {
	return index != "" && (index == public[eco] || eco == Maven && index == clojarsURL)
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
		if have.URL == s.URL && have.Scope == s.Scope && have.Registry == s.Registry {
			return
		}
	}
	c.sources[eco] = append(c.sources[eco], s)
}

// forgetProject drops what the repository declared, before it is read again.
func (c *Config) forgetProject() {
	c.clojure = false
	for eco, origin := range c.off {
		if origin == OriginProject {
			delete(c.off, eco)
		}
	}
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
	if _, listed := c.sources[Maven]; !listed && c.clojure {
		ecosystems = append(ecosystems, Maven) // Central and Clojars, which Clojure's tools search
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
		if url := public[eco]; url != "" && !c.has(eco, url) && c.off[eco] == "" {
			out = append(out, trace.Source{Ecosystem: eco, URL: url, Origin: OriginPublic, Trusted: true})
		}
		if eco == Maven && c.clojure && !c.has(eco, Clojars) {
			out = append(out, trace.Source{Ecosystem: eco, URL: Clojars, Origin: OriginPublic, Trusted: true})
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

// For reports which index a package is attributed to, and whether anything here
// vouches for it: a public default or a source this machine's configuration names is
// known, a source only the repository asks for is not. It is ForTarget for a package
// that names no registry of its own.
//
// Implements: REQ-SUP-014, REQ-SUP-016, REQ-SUP-018, REQ-SUP-026
func (c *Config) For(eco, pkg string) (index string, known bool) {
	return c.ForTarget(lang.Target{Ecosystem: eco, Package: pkg})
}

// ForTarget reports which index the map attributes a package to before anything is
// asked: the first candidate (see candidates) that serves the package as a matter of
// configuration - its scoped source or Cargo registry, the source that replaces the
// public default, or the public default itself. An additive source is not: it serves
// only what it holds, which cannot be known offline, and a repository's extra index
// would otherwise mark every public package. The client, which does ask, reports
// where a package was actually found (Client.Located).
//
// A package the organization owns is attributed to the first candidate that is not a
// public index, since it is never named to one: an additive private index is then
// where it comes from.
//
// Implements: REQ-SUP-014, REQ-SUP-016, REQ-SUP-018, REQ-SUP-063
func (c *Config) ForTarget(t lang.Target) (index string, known bool) {
	candidates := c.candidates(t.Ecosystem, t.Package, t.Registry)
	if len(candidates) == 0 {
		return "", false
	}
	if c.private != nil && c.private(t.Ecosystem, t.Package) {
		for _, k := range candidates {
			if !c.Public(t.Ecosystem, k.url) {
				return k.url, k.known
			}
		}
	}
	for _, k := range candidates {
		if k.primary {
			return k.url, k.known
		}
	}
	return candidates[0].url, candidates[0].known // only additive sources: the default is off
}

// candidate is one index a package may be asked of.
type candidate struct {
	url   string
	known bool // something here allows fetching from it (see fetchable)
	// primary marks the index that serves the package by configuration rather
	// than by holding it: a scoped source, a replacement, the public default.
	primary bool
	// onError moves on to the next candidate after any failure, not only after
	// "not found" (GOPROXY's "|").
	onError bool
}

// candidates lists the indexes a package is asked of, in the order they are asked;
// the client moves to the next one when an index does not have the package. The
// rules follow the package managers':
//
//   - An OCI image or a Terraform module named with a host is served by that host.
//   - A crate that names a Cargo registry (lang.Target.Registry) is served by that
//     registry alone; a registry serves no other crate.
//   - A scoped source that covers the package (an npm scope, a gem's source block, a
//     pubspec's hosted server) serves it alone. It is authoritative: a package
//     missing from it is not looked for on the public index, which is what a
//     dependency-confusion attack would plant it on.
//   - Otherwise the additive sources come first, in the order found (the machine's,
//     then the repository's), and the primary index last: the first Replace source,
//     every Listed one in order, or the public default (with Clojars after Maven
//     Central for a Clojure project) unless it is switched off. A ReplaceAll source
//     leaves the additive ones out. Asking the organization's own index first is
//     what Maven and Composer do, and it keeps the name of a package that index has
//     from being sent to the public one at all.
//
// A source the repository names is listed like any other, with known false: the
// client does not ask it, but reports it when nothing it could ask had the package.
//
// Implements: REQ-SUP-014, REQ-SUP-016, REQ-SUP-063
func (c *Config) candidates(eco, pkg, registry string) []candidate {
	if eco == OCI {
		// A container reference carries its registry: "ghcr.io/org/app" is not
		// "app" from Docker Hub. Nothing needs to be configured to see that; to be
		// asked, the registry has to be Docker Hub, one this machine's container
		// configuration names, or one the user vouched for.
		registry := ociRegistry(pkg)
		host := Host(registry)
		return []candidate{{url: registry, primary: true, known: registry == public[OCI] || c.trusted[registry] ||
			c.trusted[host] || c.credentials.Registry(host)}}
	}
	if eco == TerraformModule {
		// A module address carries its registry's host when it is not the public
		// one (app.terraform.io/acme/vpc/aws): that host serves it, and it is known
		// when this machine's Terraform configuration names it.
		if host := terraformHost(pkg); host != "" {
			registry := "https://" + host
			return []candidate{{url: registry, primary: true,
				known: c.trusted[registry] || c.trusted[host] || c.credentials.TerraformHost(host)}}
		}
	}
	one := func(s Source) []candidate {
		return []candidate{{url: s.URL, primary: true, known: c.fetchable(eco, s)}}
	}
	if eco == Cargo && registry != "" {
		// A crate from a named registry, by name (Cargo.toml) or by index URL
		// (Cargo.lock). An index URL nothing here configures is still where the
		// crate comes from, and is known only if the user vouched for it.
		want := cargoRegistryURL(registry)
		for _, s := range c.sources[eco] {
			if s.Registry != "" && (s.Registry == registry || cargoRegistryURL(s.URL) == want) {
				return one(s)
			}
		}
		if strings.Contains(registry, "://") {
			return one(Source{URL: want})
		}
		return nil // a registry no configuration defines: Cargo itself would fail
	}
	for _, s := range c.sources[eco] {
		if s.Scope != "" && s.Registry == "" && matches(eco, s.Scope, pkg) {
			return one(s)
		}
	}
	var additive, primary []candidate
	var chosen *Source
	for i, s := range c.sources[eco] {
		if s.Scope != "" || s.Registry != "" {
			continue
		}
		k := candidate{url: s.URL, known: c.fetchable(eco, s), onError: s.OnError}
		switch s.Kind {
		case Additive:
			additive = append(additive, k)
		case Listed:
			if chosen == nil || chosen.Kind == Listed {
				k.primary = true
				primary = append(primary, k)
				chosen = &c.sources[eco][i]
			}
		default:
			if chosen == nil {
				k.primary = true
				primary = append(primary, k)
				chosen = &c.sources[eco][i]
			}
		}
	}
	if chosen == nil && c.off[eco] == "" && public[eco] != "" {
		primary = append(primary, candidate{url: public[eco], known: true, primary: true})
	}
	if eco == Maven && c.clojure && (chosen == nil || chosen.Kind != ReplaceAll) && c.off[eco] == "" {
		// Clojure's tools search Clojars after Maven Central (or its mirror)
		// without being told to.
		primary = append(primary, candidate{url: clojarsURL, known: true})
	}
	if chosen != nil && chosen.Kind == ReplaceAll {
		additive = nil
	}
	out := make([]candidate, 0, len(additive)+len(primary))
	seen := map[string]bool{}
	for _, k := range append(additive, primary...) {
		if !seen[k.url] {
			seen[k.url] = true
			out = append(out, k)
		}
	}
	return out
}

// cargoRegistryURL is a Cargo registry's index URL as Cargo.lock and a config file
// compare: without the protocol prefix ("registry+", "sparse+") and trailing slash.
func cargoRegistryURL(u string) string {
	u = strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(u), "registry+"), "sparse+")
	return strings.TrimRight(u, "/")
}

// matches reports whether a scope covers a package name. npm scopes are exact, Maven
// groups match by prefix, and everything else is a name.
func matches(eco, scope, pkg string) bool {
	switch eco {
	case NPM:
		return strings.HasPrefix(pkg, scope+"/")
	case Maven:
		// A Maven package is group:artifact; the scope is a group prefix.
		group, _, _ := strings.Cut(pkg, ":")
		return group == scope || strings.HasPrefix(group, scope+".")
	}
	return strings.EqualFold(scope, pkg)
}

// terraformHost is the registry host a module address names ("" for the public
// registry): the first of four segments before any //subdirectory.
func terraformHost(pkg string) string {
	addr, _, _ := strings.Cut(pkg, "//")
	if parts := strings.Split(addr, "/"); len(parts) == 4 {
		return parts[0]
	}
	return ""
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

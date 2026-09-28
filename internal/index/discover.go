package index

import (
	"encoding/json"
	"encoding/xml"
	"maps"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/edn"
	"github.com/sarumaj/depphunter-cli/internal/lang/luarocks"
	"github.com/sarumaj/depphunter-cli/internal/lang/nuget"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// Discover reads index configuration from this machine first and the repository
// second, so a source the machine names is preferred and trusted, and one only the
// repository names is recorded but never vouched for.
func Discover(files []*scan.File, env func(string) string, home string) *Config {
	return NewDiscoverer(env, home).Discover(files)
}

// Discoverer fills one configuration once the files are known. The configuration
// exists before that, so a client can be built against it up front and still see what
// the scan later finds.
type Discoverer struct {
	cfg  *Config
	env  func(string) string
	home string
	// read says the machine's configuration has been read: it is this user's, it
	// does not change with the repository, and --watch discovers on every analysis.
	read bool
}

func NewDiscoverer(env func(string) string, home string) *Discoverer {
	return &Discoverer{cfg: New(), env: env, home: home}
}

// Config is the configuration this discoverer fills.
func (d *Discoverer) Config() *Config { return d.cfg }

// Discover reads the machine's configuration on the first call and the repository's
// on every call, replacing what the repository declared before, so under --watch the
// sources follow the repository's files as they change.
//
// Implements: REQ-SUP-015, REQ-SUP-016
func (d *Discoverer) Discover(files []*scan.File) *Config {
	if !d.read {
		d.cfg.machine(d.env, d.home)
		d.read = true
	}
	d.cfg.forgetProject()
	d.cfg.project(files)
	return d.cfg
}

// machine reads the configuration of whoever is running depphunter: environment first,
// then the files their package managers read.
//
// Implements: REQ-SUP-015
func (c *Config) machine(env func(string) string, home string) {
	k := sink{
		put: func(eco string, s Source) {
			s.Trusted, s.Origin = true, OriginMachine
			c.Add(eco, s)
		},
		off: func(eco string) { c.SwitchOff(eco, OriginMachine) },
	}
	add := k.add

	add(NPM, env("NPM_CONFIG_REGISTRY"), "")
	add(PyPI, env("PIP_INDEX_URL"), "")
	for _, u := range strings.Fields(env("PIP_EXTRA_INDEX_URL")) {
		k.extra(PyPI, u)
	}
	parseGoproxy(env("GOPROXY"), k)
	// Composer's global configuration lives in COMPOSER_HOME, whose default depends on
	// the platform; both usual places are read below.
	if dir := env("COMPOSER_HOME"); dir != "" {
		if data, err := os.ReadFile(filepath.Join(dir, "config.json")); err == nil {
			parseComposer(data, k)
		}
	}
	// Bundler's mirror of rubygems.org, from the environment.
	add(RubyGems, env("BUNDLE_MIRROR__RUBYGEMS__ORG"), "")
	// The server dart pub and flutter pub get install from instead of pub.dev.
	add(Pub, env("PUB_HOSTED_URL"), "")
	// The Hex API Mix and rebar3 talk to instead of hex.pm's. HEX_MIRROR is not read:
	// a mirror serves the repository's signed protobuf files, not this API.
	add(Hex, env("HEX_API_URL"), "")
	// The repositories renv restores from instead of those renv.lock records, and the
	// R profile R reads first, which is where options(repos = ...) usually lives.
	for _, u := range strings.FieldsFunc(env("RENV_CONFIG_REPOS_OVERRIDE"), func(r rune) bool { return r == ';' || r == ',' }) {
		if k, v, ok := strings.Cut(u, "="); ok && !strings.Contains(k, "/") {
			u = v // CRAN=https://...
		}
		if u = strings.TrimSpace(u); !CRANMirror(u) {
			add(CRAN, u, "")
		}
	}
	if p := env("R_PROFILE_USER"); p != "" {
		if data, err := os.ReadFile(p); err == nil {
			parseRprofile(data, add)
		}
	}
	// cabal's configuration: CABAL_CONFIG names the file, CABAL_DIR the directory
	// holding it; else ~/.config/cabal/config (XDG) or ~/.cabal/config below.
	if f := env("CABAL_CONFIG"); f != "" {
		if data, err := os.ReadFile(f); err == nil {
			parseCabalRepositories(data, add)
		}
	} else if dir := env("CABAL_DIR"); dir != "" {
		if data, err := os.ReadFile(filepath.Join(dir, "config")); err == nil {
			parseCabalRepositories(data, add)
		}
	}
	// The LuaRocks configuration file LUAROCKS_CONFIG names replaces the user's.
	if f := env("LUAROCKS_CONFIG"); f != "" {
		if data, err := os.ReadFile(f); err == nil {
			parseLuaRocksConfig(data, add)
		}
	}
	// Julia registries installed in the depots (JULIA_DEPOT_PATH, else ~/.julia)
	// besides General. JULIA_PKG_SERVER is not read: a package server serves
	// registries as tarballs, not as the files asked for here.
	depots := []string{}
	for _, d := range filepath.SplitList(env("JULIA_DEPOT_PATH")) {
		if d == "" {
			d = filepath.Join(home, ".julia")
		}
		depots = append(depots, d)
	}
	if len(depots) == 0 && home != "" {
		depots = []string{filepath.Join(home, ".julia")}
	}
	for _, d := range depots {
		files, _ := filepath.Glob(filepath.Join(d, "registries", "*", "Registry.toml"))
		sort.Strings(files)
		for _, f := range files {
			if data, err := os.ReadFile(f); err == nil {
				parseJuliaRegistry(data, add)
			}
		}
	}
	if home == "" {
		return
	}
	for _, v := range []string{"5.1", "5.2", "5.3", "5.4"} {
		if data, err := os.ReadFile(filepath.Join(home, ".luarocks", "config-"+v+".lua")); err == nil {
			parseLuaRocksConfig(data, add)
		}
	}
	for _, f := range []struct {
		path  string
		parse func([]byte, sink)
	}{
		{filepath.Join(home, ".npmrc"), plain(parseNpmrc)},
		{filepath.Join(home, ".config", "pip", "pip.conf"), parsePipConf},
		{filepath.Join(home, ".pip", "pip.conf"), parsePipConf},
		{filepath.Join(home, ".cargo", "config.toml"), parseCargoConfig},
		{filepath.Join(home, ".m2", "settings.xml"), parseMavenSettings},
		// Both places the dotnet CLI keeps the user's NuGet.Config, the same two the
		// credentials are read from, so a feed with a password is also a feed.
		{filepath.Join(home, ".nuget", "NuGet", "NuGet.Config"), parseNuGetConfig},
		{filepath.Join(home, ".config", "NuGet", "NuGet.Config"), parseNuGetConfig},
		{filepath.Join(home, ".config", "composer", "config.json"), parseComposer},
		{filepath.Join(home, ".composer", "config.json"), parseComposer},
		{filepath.Join(home, ".gemrc"), plain(parseGemrc)},
		{filepath.Join(home, ".bundle", "config"), plain(parseBundleConfig)},
		{filepath.Join(home, ".Rprofile"), plain(parseRprofile)},
		{filepath.Join(home, ".config", "cabal", "config"), plain(parseCabalRepositories)},
		{filepath.Join(home, ".bazelrc"), plain(parseBazelrc)},
		{filepath.Join(home, ".cabal", "config"), plain(parseCabalRepositories)},
	} {
		if data, err := os.ReadFile(f.path); err == nil {
			f.parse(data, k)
		}
	}
}

// sink is where a parser records what it reads. Most name a URL and perhaps a scope
// (add); those that know how a source relates to the public default say so (put,
// extra), and a configuration that switches the default off says that (off).
type sink struct {
	put func(eco string, s Source)
	off func(eco string)
}

// add records a source that replaces the public default, or serves its scope.
func (k sink) add(eco, url, scope string) { k.put(eco, Source{URL: url, Scope: scope}) }

// extra records a source asked beside the public default.
func (k sink) extra(eco, url string) { k.put(eco, Source{URL: url, Kind: Additive}) }

// plain adapts a parser that only ever calls add.
func plain(parse func([]byte, func(eco, url, scope string))) func([]byte, sink) {
	return func(data []byte, k sink) { parse(data, k.add) }
}

// parseGoproxy reads GOPROXY as the go command does: a list of proxies asked in
// order, each moving on to the next when it answers 404 or 410 (a "," after it) or
// after any failure (a "|"). "direct" (a version control fetch) and "off" end what
// can be asked; entries after them are not reached by anything asked here. A GOPROXY
// that is set replaces proxy.golang.org, so GOPROXY=direct or =off leaves no proxy
// to ask at all.
//
// Implements: REQ-SUP-015, REQ-SUP-063
func parseGoproxy(value string, k sink) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	k.off(Go)
	for value != "" {
		i := strings.IndexAny(value, ",|")
		entry, sep := value, byte(0)
		if i >= 0 {
			entry, sep, value = value[:i], value[i], value[i+1:]
		} else {
			value = ""
		}
		switch entry = strings.TrimSpace(entry); entry {
		case "":
			continue
		case "direct", "off":
			return
		}
		k.put(Go, Source{URL: entry, Kind: Listed, OnError: sep == '|'})
	}
}

// project reads what the repository asks for. Nothing here is trusted: it says where a
// package comes from, and a source nobody on this machine knows is worth seeing.
//
// Implements: REQ-SUP-015, REQ-SUP-018
func (c *Config) project(files []*scan.File) {
	k := sink{
		put: func(eco string, s Source) {
			s.Trusted, s.Origin = false, OriginProject
			c.Add(eco, s)
		},
		off: func(eco string) { c.SwitchOff(eco, OriginProject) },
	}
	add := k.add
	// A repository may declare several indexes for one ecosystem. Which one is shown
	// is then arbitrary, but it must not change between runs, or the same repository
	// would draw differently each time.
	ordered := append([]*scan.File(nil), files...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Path < ordered[j].Path })
	for _, f := range ordered {
		base := strings.ToLower(path.Base(f.Path))
		data, err := os.ReadFile(f.Abs)
		if err != nil {
			continue
		}
		switch {
		case base == ".npmrc":
			parseNpmrc(data, add)
		case base == ".yarnrc.yml":
			parseYarnrc(data, add)
		case base == "pip.conf", base == "pip.ini":
			parsePipConf(data, k)
		case strings.HasPrefix(base, "requirements") && strings.HasSuffix(base, ".txt"):
			parseRequirements(data, k)
		case base == "pyproject.toml":
			parsePyproject(data, k)
		case base == "nuget.config":
			parseNuGetConfig(data, k)
		case base == "paket.dependencies":
			parsePaketSources(data, k)
		case base == "paket.lock":
			parsePaketLock(data, k)
		case base == "pom.xml":
			parsePom(data, k)
		case base == "build.gradle", base == "build.gradle.kts", base == "settings.gradle", base == "settings.gradle.kts":
			parseGradleRepos(data, k)
		case base == "deps.edn", base == "bb.edn", base == "shadow-cljs.edn", base == "project.clj", base == "build.boot":
			c.clojure = true
			parseClojureRepos(base, data, k)
		case base == "composer.json":
			parseComposer(data, k)
		case f.Path == "Gemfile" || strings.HasSuffix(f.Path, "/Gemfile") || base == "gems.rb":
			parseGemfile(data, add)
		case base == "gemfile.lock" || base == "gems.locked":
			parseGemfileLock(data, add)
		case base == "pubspec.yaml" || base == "pubspec_overrides.yaml":
			parsePubspec(data, add)
		case base == "pubspec.lock":
			parsePubspecLock(data, add)
		case base == "renv.lock":
			parseRenvLock(data, add)
		case base == ".rprofile" || base == "rprofile.site":
			parseRprofile(data, add)
		case base == "cabal.project" || base == "cabal.project.local":
			parseCabalRepositories(data, add)
		case base == "config.toml" && strings.HasSuffix(path.Dir(f.Path), ".cargo"):
			parseCargoConfig(data, k)
		case f.Path == "Podfile" || strings.HasSuffix(f.Path, "/Podfile"):
			parsePodfile(data, add)
		case base == "podfile.lock":
			parsePodfileLock(data, add)
		case base == ".bazelrc" || strings.HasSuffix(base, ".bazelrc"):
			parseBazelrc(data, add)
		case strings.HasPrefix(base, "config-") && strings.HasSuffix(base, ".lua") && path.Base(path.Dir(f.Path)) == ".luarocks":
			parseLuaRocksConfig(data, add) // what luarocks init writes for the project
		}
	}
}

// parseNpmrc reads "registry=" and the per-scope "@scope:registry=" of an npm config.
func parseNpmrc(data []byte, add func(eco, url, scope string)) {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, value = strings.TrimSpace(key), strings.Trim(strings.TrimSpace(value), `"`)
		switch {
		case key == "registry":
			add(NPM, value, "")
		case strings.HasSuffix(key, ":registry") && strings.HasPrefix(key, "@"):
			add(NPM, value, strings.TrimSuffix(key, ":registry"))
		}
	}
}

// parseYarnrc reads Yarn Berry's npmRegistryServer and its per-scope equivalent.
func parseYarnrc(data []byte, add func(eco, url, scope string)) {
	scope := ""
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		value := func() string {
			_, v, _ := strings.Cut(trimmed, ":")
			return strings.Trim(strings.TrimSpace(v), `"'`)
		}
		switch {
		case strings.HasPrefix(trimmed, "npmRegistryServer:"):
			if strings.HasPrefix(line, " ") {
				add(NPM, value(), scope) // inside npmScopes
			} else {
				add(NPM, value(), "")
			}
		case !strings.HasPrefix(line, " ") && strings.HasSuffix(trimmed, ":"):
			scope = ""
		case strings.HasPrefix(line, "  ") && strings.HasSuffix(trimmed, ":") && !strings.Contains(trimmed, " "):
			// "  acme:" inside npmScopes names a scope without its @.
			scope = "@" + strings.TrimSuffix(trimmed, ":")
		}
	}
}

// parsePipConf reads a pip configuration file: index-url replaces PyPI, and each URL
// of extra-index-url (on the line, or on the indented lines that continue it) is
// asked beside it.
//
// Implements: REQ-SUP-015, REQ-SUP-063
func parsePipConf(data []byte, k sink) {
	extra := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if extra && trimmed != "" && (line[0] == ' ' || line[0] == '\t') && !strings.Contains(trimmed, "=") {
			for _, u := range strings.Fields(trimmed) {
				k.extra(PyPI, u)
			}
			continue
		}
		extra = false
		key, value, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "index-url", "index_url":
			k.add(PyPI, strings.TrimSpace(value), "")
		case "extra-index-url", "extra_index_url":
			extra = true
			for _, u := range strings.Fields(value) {
				k.extra(PyPI, u)
			}
		}
	}
}

// parseRequirements reads the index options a requirements file may carry:
// --index-url replaces PyPI, --extra-index-url is asked beside it.
//
// Implements: REQ-SUP-015, REQ-SUP-063
func parseRequirements(data []byte, k sink) {
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 0 {
			continue
		}
		option, value := fields[0], ""
		if len(fields) > 1 {
			value = fields[1]
		}
		if o, v, ok := strings.Cut(option, "="); ok { // --index-url=https://…
			option, value = o, v
		}
		switch option {
		case "--index-url", "-i":
			k.add(PyPI, value, "")
		case "--extra-index-url":
			k.extra(PyPI, value)
		}
	}
}

// parsePyproject reads the indexes Poetry and uv declare, as each tool uses them.
//
// A Poetry source is primary unless its priority says otherwise, and a primary
// source replaces PyPI; a supplemental one (or a legacy secondary one) is asked
// beside it; an explicit one serves only the dependencies that name it with
// `source = "<name>"`. A uv index is asked before PyPI unless it is the default
// (default = true), which replaces PyPI; an explicit one serves only the packages
// [tool.uv.sources] pins to it, as any index a package is pinned to does.
//
// Implements: REQ-SUP-015, REQ-SUP-063
func parsePyproject(data []byte, k sink) {
	type poetrySource struct {
		Name, URL, Priority string
		Default, Secondary  bool
	}
	var doc struct {
		Tool struct {
			Poetry struct {
				Source       []poetrySource
				Dependencies map[string]any
				Group        map[string]struct{ Dependencies map[string]any }
			}
			UV struct {
				Index []struct {
					Name, URL         string
					Default, Explicit bool
				}
				Sources map[string]any
			} `toml:"uv"`
		}
	}
	if _, err := toml.Decode(string(data), &doc); err != nil {
		return
	}
	poetry := doc.Tool.Poetry
	named := map[string]string{}
	for _, s := range poetry.Source {
		named[s.Name] = s.URL
		switch {
		case s.Priority == "explicit":
		case s.Priority == "supplemental" || s.Secondary || s.Priority == "secondary":
			k.extra(PyPI, s.URL)
		default:
			k.add(PyPI, s.URL, "")
		}
	}
	deps := []map[string]any{poetry.Dependencies}
	for _, g := range slices.Sorted(maps.Keys(poetry.Group)) {
		deps = append(deps, poetry.Group[g].Dependencies)
	}
	for _, d := range deps {
		for _, name := range slices.Sorted(maps.Keys(d)) {
			if spec, ok := d[name].(map[string]any); ok {
				if src, ok := spec["source"].(string); ok && named[src] != "" {
					k.add(PyPI, named[src], name)
				}
			}
		}
	}
	uv := doc.Tool.UV
	clear(named)
	for _, s := range uv.Index {
		named[s.Name] = s.URL
		switch {
		case s.Explicit:
		case s.Default:
			k.add(PyPI, s.URL, "")
		default:
			k.extra(PyPI, s.URL)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(uv.Sources)) {
		// { index = "name" }, or a list of such entries with markers
		specs, ok := uv.Sources[name].([]map[string]any)
		if !ok {
			if list, isList := uv.Sources[name].([]any); isList {
				for _, e := range list {
					if m, ok := e.(map[string]any); ok {
						specs = append(specs, m)
					}
				}
			} else if m, isMap := uv.Sources[name].(map[string]any); isMap {
				specs = append(specs, m)
			}
		}
		for _, spec := range specs {
			if idx, ok := spec["index"].(string); ok && named[idx] != "" {
				k.add(PyPI, named[idx], name)
			}
		}
	}
}

// parseCargoConfig reads what replaces crates.io and the alternative registries.
//
// crates.io is replaced by whatever `[source.crates-io] replace-with` ends at,
// following the chain; that source serves every crate crates.io would. An
// alternative registry (`[registries.<name>]`) is not a replacement: it serves only
// the crates that declare it (lang.Target.Registry), and is recorded under its name
// for them. A [source] no chain reaches serves nothing.
//
// Implements: REQ-SUP-015, REQ-SUP-063
func parseCargoConfig(data []byte, k sink) {
	var doc struct {
		Source map[string]struct {
			Registry    string
			ReplaceWith string `toml:"replace-with"`
		}
		Registries map[string]struct{ Index string }
	}
	if _, err := toml.Decode(string(data), &doc); err != nil {
		return
	}
	name := "crates-io"
	for range len(doc.Source) { // bounded: a replace-with cycle is a broken config
		next := doc.Source[name].ReplaceWith
		if next == "" {
			break
		}
		name = next
	}
	if s, ok := doc.Source[name]; ok && name != "crates-io" {
		k.add(Cargo, s.Registry, "")
	} else if r, ok := doc.Registries[name]; ok {
		k.add(Cargo, r.Index, "")
	}
	for _, key := range slices.Sorted(maps.Keys(doc.Registries)) {
		k.put(Cargo, Source{URL: doc.Registries[key].Index, Registry: key, Kind: Additive})
	}
}

// nugetEntry is NuGet's <add key="…" value="…" />, which is how a config names both a
// package source and a credential.
type nugetEntry struct {
	Key   string `xml:"key,attr"`
	Value string `xml:"value,attr"`
}

// nugetSources is the <packageSources> block: the feeds a config declares, which
// auth.go reads for the same reason this does.
type nugetSources struct {
	Add []nugetEntry `xml:"add"`
}

// parseNuGetConfig reads the package sources of a NuGet configuration. NuGet asks
// every source it has for a package, nuget.org among them, so each feed is asked
// beside the public default - unless the config's <packageSources> starts with
// <clear/>, which drops what other configs (and so nuget.org) contributed. A feed
// on nuget.org itself is the public default; a local folder is no index.
//
// Implements: REQ-SUP-015, REQ-SUP-063
func parseNuGetConfig(data []byte, k sink) {
	var doc struct {
		PackageSources struct {
			nugetSources
			Clear *struct{} `xml:"clear"`
		} `xml:"packageSources"`
	}
	if xml.Unmarshal(data, &doc) != nil {
		return
	}
	official := false
	for _, s := range doc.PackageSources.Add {
		official = addNuGetFeed(s.Value, k) || official
	}
	if doc.PackageSources.Clear != nil && !official {
		k.off(NuGet)
	}
}

// parsePaketSources reads the NuGet feeds paket.dependencies names in its `source`
// lines (every group's). A directory source is no index, and nuget.org (Paket
// projects often still name its retired v2 API) is the public index already.
//
// Implements: REQ-FSHARP-010
func parsePaketSources(data []byte, k sink) {
	_, sources := nuget.ParseDependencies(data)
	for _, s := range sources {
		addNuGetFeed(s.URL, k)
	}
}

// parsePaketLock reads the feeds paket.lock resolved its NuGet packages from (the
// `remote:` lines of its NUGET sections).
//
// Implements: REQ-FSHARP-010
func parsePaketLock(data []byte, k sink) {
	seen := map[string]bool{}
	for _, l := range nuget.ParseLock(data) {
		if l.Kind == "nuget" && !seen[l.Remote] {
			seen[l.Remote] = true
			addNuGetFeed(l.Remote, k)
		}
	}
}

// addNuGetFeed records a feed asked beside nuget.org, and reports whether the feed
// is nuget.org itself (by any of its addresses): the public default, which is not
// recorded again.
func addNuGetFeed(feed string, k sink) (official bool) {
	u, err := url.Parse(strings.TrimSpace(strings.Trim(feed, `"`)))
	if err != nil || u.Scheme != "https" && u.Scheme != "http" {
		return false
	}
	if h := strings.ToLower(u.Hostname()); h == "nuget.org" || strings.HasSuffix(h, ".nuget.org") {
		return true
	}
	k.extra(NuGet, u.String())
	return false
}

// parsePom reads the repositories a Maven project declares. Maven asks them before
// Maven Central, which stays: each is a source beside it.
//
// Implements: REQ-SUP-015, REQ-SUP-063
func parsePom(data []byte, k sink) {
	var doc struct {
		Repositories struct {
			Repository []struct {
				URL string `xml:"url"`
			} `xml:"repository"`
		} `xml:"repositories"`
	}
	if lang.UnmarshalXML(data, &doc) != nil {
		return
	}
	for _, r := range doc.Repositories.Repository {
		if u := strings.TrimSpace(r.URL); !MavenPublic(u) {
			k.extra(Maven, u)
		}
	}
}

var (
	// gradleRepo is a maven repository of a Gradle script: maven("url"),
	// maven(url = "url"), maven { url "url" }, maven { url = uri("url") } and
	// maven { setUrl("url") }.
	gradleRepo = regexp.MustCompile(`\bmaven\s*(?:\(\s*(?:url\s*=\s*)?(?:uri\(\s*)?["']([^"']+)["']|` +
		`\{[^{}]*?\b(?:url|setUrl)\s*(?:=\s*|\(\s*)?(?:uri\(\s*)?["']([^"']+)["'])`)
	// gradleOwnBlock opens a block whose repositories serve Gradle itself - its
	// plugins, the build script's classpath - rather than the code.
	gradleOwnBlock = regexp.MustCompile(`\b(?:pluginManagement|buildscript)\s*\{`)
)

// parseGradleRepos reads the Maven repositories a Gradle build or settings script
// declares, other than Maven Central. mavenCentral(), google() and mavenLocal()
// name no repository of the project's own, and the repositories of
// pluginManagement and buildscript blocks serve Gradle's plugins, not the code.
// Each is asked beside Maven Central.
//
// Implements: REQ-SUP-015, REQ-SUP-056, REQ-SUP-063
func parseGradleRepos(data []byte, k sink) {
	src := string(data)
	for {
		loc := gradleOwnBlock.FindStringIndex(src)
		if loc == nil {
			break
		}
		depth, end := 1, len(src)
		for i := loc[1]; i < len(src); i++ {
			if src[i] == '{' {
				depth++
			} else if src[i] == '}' {
				if depth--; depth == 0 {
					end = i + 1
					break
				}
			}
		}
		src = src[:loc[0]] + src[end:]
	}
	for _, m := range gradleRepo.FindAllStringSubmatch(src, -1) {
		u := strings.TrimSpace(m[1] + m[2])
		if strings.HasPrefix(u, "http") && !MavenPublic(u) {
			k.extra(Maven, u)
		}
	}
}

// parseClojureRepos reads the Maven repositories a Clojure manifest declares:
// deps.edn's and bb.edn's :mvn/repos, Leiningen's and Boot's :repositories,
// shadow-cljs's :repositories or :maven {:repositories}. Maven Central and Clojars
// are the public indexes and are not recorded as the repository's; the others are
// asked beside them.
//
// Implements: REQ-SUP-056, REQ-SUP-063
func parseClojureRepos(base string, data []byte, k sink) {
	forms := edn.Read(data)
	var repos []*edn.Node
	switch base {
	case "deps.edn", "bb.edn", "shadow-cljs.edn":
		if len(forms) > 0 {
			top := forms[0]
			repos = append(repos, top.Get("mvn/repos"), top.Get("repositories"))
			if mv := top.Get("maven"); mv != nil {
				repos = append(repos, mv.Get("repositories"))
			}
		}
	case "project.clj", "build.boot":
		for _, f := range forms {
			if h := f.Head(); h != "defproject" && h != "set-env!" {
				continue
			}
			for i := 1; i+1 < len(f.Kids); i++ {
				if k := f.Kids[i]; k.Kind == edn.Keyword && k.Text == "repositories" {
					repos = append(repos, edn.Unquote(f.Kids[i+1]))
				}
			}
		}
	}
	for _, r := range repos {
		if r == nil {
			continue
		}
		// {"name" {:url "..."}}, {"name" "url"}, [["name" "url"]], [["name" {:url "..."}]]
		var values []*edn.Node
		switch r.Kind {
		case edn.Map:
			for i := 1; i < len(r.Kids); i += 2 {
				values = append(values, r.Kids[i])
			}
		case edn.Vector, edn.List:
			for _, e := range r.Kids {
				if len(e.Kids) == 2 {
					values = append(values, e.Kids[1])
				}
			}
		}
		for _, v := range values {
			u := v
			if v.Kind == edn.Map {
				u = v.Get("url")
			}
			if u != nil && u.Kind == edn.String && !MavenPublic(u.Text) {
				k.extra(Maven, strings.TrimSpace(u.Text))
			}
		}
	}
}

// parseComposer reads the Composer repositories a composer.json or Composer's global
// config.json declares: those of type "composer" (Private Packagist, Satis, a
// Repman or Nexus proxy) are package indexes, asked before Packagist as Composer
// does, and Packagist stays unless `"packagist.org": false` (or the older
// `"packagist": false`) switches it off. A VCS, path or inline package repository
// is not an index. The list form and the object form (keyed by name) are both
// read, in order and by name.
//
// Implements: REQ-SUP-015, REQ-SUP-063
func parseComposer(data []byte, k sink) {
	var doc struct {
		Repositories json.RawMessage `json:"repositories"`
	}
	if json.Unmarshal(data, &doc) != nil || len(doc.Repositories) == 0 {
		return
	}
	type repository struct {
		Type string `json:"type"`
		URL  string `json:"url"`
	}
	disabled := func(name string, raw json.RawMessage) bool {
		return (name == "packagist.org" || name == "packagist") && strings.TrimSpace(string(raw)) == "false"
	}
	var repos []json.RawMessage
	if json.Unmarshal(doc.Repositories, &repos) != nil {
		var named map[string]json.RawMessage
		if json.Unmarshal(doc.Repositories, &named) != nil {
			return
		}
		for _, key := range slices.Sorted(maps.Keys(named)) {
			if disabled(key, named[key]) {
				k.off(Composer)
				continue
			}
			repos = append(repos, named[key])
		}
	}
	for _, raw := range repos {
		var r repository
		if json.Unmarshal(raw, &r) == nil && r.Type == "composer" {
			k.extra(Composer, strings.TrimSpace(r.URL))
			continue
		}
		var off map[string]json.RawMessage // [{"packagist.org": false}]
		if json.Unmarshal(raw, &off) == nil {
			for name, v := range off {
				if disabled(name, v) {
					k.off(Composer)
				}
			}
		}
	}
}

var (
	gemSource = regexp.MustCompile(`^(\s*)source\s*\(?\s*["']([^"']+)["']\s*\)?\s*(do\b)?`)
	gemDecl   = regexp.MustCompile(`^\s*gem\s*\(?\s*["']([^"']+)["']`)
	blockEnd  = regexp.MustCompile(`^(\s*)end\b`)
)

// parseGemfile reads the gem servers a Gemfile names: a `source` line serves every
// gem, a `source ... do` block only the gems declared inside it.
func parseGemfile(data []byte, add func(eco, url, scope string)) {
	block, indent := "", ""
	for _, line := range strings.Split(string(data), "\n") {
		if m := gemSource.FindStringSubmatch(line); m != nil {
			if m[3] == "" {
				add(RubyGems, m[2], "")
			} else {
				block, indent = m[2], m[1]
			}
			continue
		}
		if block == "" {
			continue
		}
		if m := blockEnd.FindStringSubmatch(line); m != nil && m[1] == indent {
			block = ""
		} else if m := gemDecl.FindStringSubmatch(line); m != nil {
			add(RubyGems, block, m[1])
		}
	}
}

// parseGemfileLock reads the remotes of Gemfile.lock's GEM sections. The first serves
// everything; a later one - a private server beside rubygems.org - only the gems
// locked under it.
func parseGemfileLock(data []byte, add func(eco, url, scope string)) {
	first, remote, gem := "", "", false
	for _, line := range strings.Split(string(data), "\n") {
		switch {
		case line != "" && line[0] != ' ':
			gem, remote = strings.TrimSpace(line) == "GEM", ""
		case !gem:
		case strings.HasPrefix(line, "  remote: "):
			remote = strings.TrimSpace(strings.TrimPrefix(line, "  remote: "))
			if first == "" {
				first = remote
				add(RubyGems, remote, "")
			}
		case remote != "" && remote != first && strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "     "):
			if name, _, ok := strings.Cut(strings.TrimSpace(line), " "); ok {
				add(RubyGems, remote, name)
			}
		}
	}
}

// parseGemrc reads the sources of a ~/.gemrc (YAML: ":sources:" and a list).
func parseGemrc(data []byte, add func(eco, url, scope string)) {
	in := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, ":sources:"):
			in = true
		case in && strings.HasPrefix(trimmed, "- "):
			add(RubyGems, strings.Trim(strings.TrimSpace(trimmed[2:]), `"'`), "")
		case trimmed != "":
			in = false
		}
	}
}

// parseBundleConfig reads Bundler's mirror of rubygems.org from ~/.bundle/config
// (BUNDLE_MIRROR__RUBYGEMS__ORG, or the URL form of the key).
func parseBundleConfig(data []byte, add func(eco, url, scope string)) {
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, ": ")
		if !ok || !strings.HasPrefix(key, "BUNDLE_MIRROR__") || !strings.Contains(strings.ToUpper(key), "RUBYGEMS__ORG") {
			continue
		}
		add(RubyGems, strings.Trim(strings.TrimSpace(value), `"'`), "")
	}
}

// parsePubspec reads the servers a pubspec names for its hosted dependencies, each
// serving only the package it is written for: `hosted: <url>`, or the older
// `hosted: {name, url}`.
func parsePubspec(data []byte, add func(eco, url, scope string)) {
	var doc map[string]any
	if yaml.Unmarshal(data, &doc) != nil {
		return
	}
	for _, section := range []string{"dependencies", "dev_dependencies", "dependency_overrides"} {
		deps, _ := doc[section].(map[string]any)
		for _, name := range slices.Sorted(maps.Keys(deps)) {
			d, _ := deps[name].(map[string]any)
			switch h := d["hosted"].(type) {
			case string:
				add(Pub, h, name)
			case map[string]any:
				if u, ok := h["url"].(string); ok {
					add(Pub, u, name)
				}
			}
		}
	}
}

// parsePubspecLock reads the servers pubspec.lock resolved hosted packages from,
// other than pub.dev (and its former name), each serving the packages locked from it.
func parsePubspecLock(data []byte, add func(eco, url, scope string)) {
	var doc struct {
		Packages map[string]struct {
			Source      string `yaml:"source"`
			Description any    `yaml:"description"`
		} `yaml:"packages"`
	}
	if yaml.Unmarshal(data, &doc) != nil {
		return
	}
	for _, name := range slices.Sorted(maps.Keys(doc.Packages)) {
		p := doc.Packages[name]
		d, _ := p.Description.(map[string]any)
		u, _ := d["url"].(string)
		if p.Source != "hosted" || u == "" {
			continue
		}
		if u = strings.TrimRight(u, "/"); u != public[Pub] && u != "https://pub.dartlang.org" {
			add(Pub, u, name)
		}
	}
}

// parseRenvLock reads the repositories renv.lock records (R.Repositories, by name) and
// scopes each to the packages installed from it. CRAN and its mirrors, Posit Package
// Manager's included, are the public index and are not recorded.
//
// Implements: REQ-SUP-015, REQ-SUP-048
func parseRenvLock(data []byte, add func(eco, url, scope string)) {
	var doc struct {
		R struct {
			Repositories []struct{ Name, URL string }
		}
		Packages map[string]struct{ Package, Repository string }
	}
	if json.Unmarshal(data, &doc) != nil {
		return
	}
	repos := map[string]string{}
	for _, r := range doc.R.Repositories {
		repos[r.Name] = r.URL
	}
	names := slices.Sorted(maps.Keys(doc.Packages))
	for _, key := range names {
		p := doc.Packages[key]
		u, ok := repos[p.Repository]
		if !ok && strings.Contains(p.Repository, "://") {
			u = p.Repository
		}
		if u != "" && !CRANMirror(u) && !strings.Contains(strings.ToLower(p.Repository), "bioc") {
			name := p.Package
			if name == "" {
				name = key
			}
			add(CRAN, u, name)
		}
	}
}

var (
	rReposArg = regexp.MustCompile(`\brepos\s*=\s*`)
	rURL      = regexp.MustCompile(`["'](https?://[^"'\s]+)["']`)
)

// parseRprofile reads the literal repository URLs of options(repos = ...) in an R
// profile: repos = "url" or repos = c(CRAN = "url", internal = "url").
//
// Implements: REQ-SUP-015, REQ-SUP-048
func parseRprofile(data []byte, add func(eco, url, scope string)) {
	src := string(data)
	for _, loc := range rReposArg.FindAllStringIndex(src, -1) {
		rest := src[loc[1]:]
		end := len(rest)
		if strings.HasPrefix(rest, "c(") {
			depth := 0
			for i, r := range rest {
				if r == '(' {
					depth++
				} else if r == ')' {
					if depth--; depth == 0 {
						end = i + 1
						break
					}
				}
			}
		} else if i := strings.IndexAny(rest, ",)\n"); i >= 0 {
			end = i
		}
		for _, m := range rURL.FindAllStringSubmatch(rest[:end], -1) {
			if !CRANMirror(m[1]) {
				add(CRAN, m[1], "")
			}
		}
	}
}

// parseCabalRepositories reads the repository stanzas of a cabal configuration or
// cabal.project ("repository name" with an indented "url:"). Hackage itself is the
// public index and is not recorded; any other repository (a mirror, head.hackage, a
// company's) serves every package, as cabal asks each configured repository.
//
// Implements: REQ-SUP-015, REQ-SUP-049
func parseCabalRepositories(data []byte, add func(eco, url, scope string)) {
	in := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		if line[0] != ' ' && line[0] != '\t' {
			in = strings.HasPrefix(trimmed, "repository ")
			continue
		}
		if k, v, ok := strings.Cut(trimmed, ":"); in && ok && strings.EqualFold(strings.TrimSpace(k), "url") {
			if u := strings.TrimSpace(v); u != "" && !HackageItself(u) {
				add(Hackage, u, "")
			}
		}
	}
}

// parseMavenSettings reads the mirrors this machine sends Maven through. A mirror
// of `*` (or `external:*`) stands in for every repository, Central and those a
// project declares; a mirror of `central` replaces Central only; a mirror of some
// other repository serves what that repository holds, beside Central. A mirror
// that says nothing is taken to mirror everything, as the most common setup does.
//
// Implements: REQ-SUP-015, REQ-SUP-063
func parseMavenSettings(data []byte, k sink) {
	var doc struct {
		Mirrors struct {
			Mirror []struct {
				URL      string `xml:"url"`
				MirrorOf string `xml:"mirrorOf"`
			} `xml:"mirror"`
		} `xml:"mirrors"`
	}
	if xml.Unmarshal(data, &doc) != nil {
		return
	}
	for _, m := range doc.Mirrors.Mirror {
		kind := Additive
		of := strings.Split(strings.ReplaceAll(m.MirrorOf, " ", ""), ",")
		switch {
		case strings.TrimSpace(m.MirrorOf) == "", slices.Contains(of, "*"), slices.ContainsFunc(of, func(s string) bool {
			return strings.HasPrefix(s, "external:")
		}):
			kind = ReplaceAll
		case slices.Contains(of, "central"):
			kind = Replace
		}
		k.put(Maven, Source{URL: strings.TrimSpace(m.URL), Kind: kind})
	}
}

// podSource matches a Podfile's `source 'url'` line.
var podSource = regexp.MustCompile(`(?m)^\s*source[\s(]+['"]([^'"]+)['"]`)

// parsePodfile reads the spec repositories a Podfile's `source` lines name. CocoaPods
// asks each of them for every pod, in order, so a private one is recorded for all of
// them: the repository cannot say which pods it holds, and naming a company's pod to
// the public CDN is what the private patterns exist to prevent. CocoaPods' own
// repository is the public index and is not recorded.
//
// Implements: REQ-SUP-015, REQ-SUP-051
func parsePodfile(data []byte, add func(eco, url, scope string)) {
	for _, m := range podSource.FindAllSubmatch(data, -1) {
		if u := string(m[1]); !CocoaPodsTrunk(u) {
			add(CocoaPods, u, "")
		}
	}
}

// parsePodfileLock reads Podfile.lock's SPEC REPOS, which say which repository each
// pod was installed from: a private one is recorded for its pods only.
//
// Implements: REQ-SUP-015, REQ-SUP-051
func parsePodfileLock(data []byte, add func(eco, url, scope string)) {
	var doc struct {
		Repos map[string][]string `yaml:"SPEC REPOS"`
	}
	if yaml.Unmarshal(data, &doc) != nil {
		return
	}
	for _, repo := range slices.Sorted(maps.Keys(doc.Repos)) {
		if CocoaPodsTrunk(repo) {
			continue
		}
		for _, name := range doc.Repos[repo] {
			root, _, _ := strings.Cut(name, "/")
			add(CocoaPods, repo, root)
		}
	}
}

// parseLuaRocksConfig reads the rocks servers of a LuaRocks configuration file
// (rocks_servers, in order). luarocks.org itself is the public index and is not
// recorded.
//
// Implements: REQ-SUP-052
func parseLuaRocksConfig(data []byte, add func(eco, url, scope string)) {
	for _, s := range luarocks.Servers(data) {
		if !LuaRocksItself(s) {
			add(LuaRocks, s, "")
		}
	}
}

// parseJuliaRegistry reads an installed Julia registry's Registry.toml: a registry
// other than General whose repository is on GitHub serves its files at
// raw.githubusercontent.com, and serves the packages it lists (each a scoped
// source, since Pkg looks a package up in every registry by its UUID).
//
// Implements: REQ-SUP-055
func parseJuliaRegistry(data []byte, add func(eco, url, scope string)) {
	var reg juliaRegistry
	if _, err := toml.Decode(string(data), &reg); err != nil || reg.Name == "General" {
		return
	}
	u, err := url.Parse(strings.TrimSpace(reg.Repo))
	if err != nil || !strings.EqualFold(u.Hostname(), "github.com") {
		return
	}
	repo := strings.TrimSuffix(strings.Trim(u.Path, "/"), ".git")
	if strings.Count(repo, "/") != 1 {
		return
	}
	base := "https://raw.githubusercontent.com/" + repo + "/HEAD"
	names := make([]string, 0, len(reg.Packages))
	for _, p := range reg.Packages {
		names = append(names, p.Name)
	}
	sort.Strings(names)
	for _, n := range names {
		add(Julia, base, n)
	}
}

// parseBazelrc reads the registries a .bazelrc names (--registry=URL, in any
// command's section, in order). The Bazel Central Registry itself is the public
// index, and a file:// registry cannot be asked over the network: neither is added.
//
// Implements: REQ-SUP-057
func parseBazelrc(data []byte, add func(eco, url, scope string)) {
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		for i, f := range fields {
			if strings.HasPrefix(f, "#") {
				break
			}
			u, ok := strings.CutPrefix(f, "--registry=")
			if !ok && f == "--registry" && i+1 < len(fields) {
				u, ok = fields[i+1], true
			}
			if u = strings.Trim(u, `"'`); ok && strings.HasPrefix(u, "http") && !BazelCentral(u) {
				add(Bazel, u, "")
			}
		}
	}
}

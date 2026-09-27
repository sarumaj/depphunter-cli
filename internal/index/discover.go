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

	"github.com/sarumaj/depphunter-cli/internal/lang/luarocks"
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
	add := func(eco, url, scope string) {
		c.Add(eco, Source{URL: url, Scope: scope, Trusted: true, Origin: OriginMachine})
	}

	add(NPM, env("NPM_CONFIG_REGISTRY"), "")
	add(PyPI, env("PIP_INDEX_URL"), "")
	for _, u := range strings.Fields(env("PIP_EXTRA_INDEX_URL")) {
		add(PyPI, u, "")
	}
	// GOPROXY is a fallback list; "direct" and "off" name no index.
	for _, p := range strings.FieldsFunc(env("GOPROXY"), func(r rune) bool { return r == ',' || r == '|' }) {
		if p = strings.TrimSpace(p); p != "" && p != "direct" && p != "off" {
			add(Go, p, "")
		}
	}
	// Composer's global configuration lives in COMPOSER_HOME, whose default depends on
	// the platform; both usual places are read below.
	if dir := env("COMPOSER_HOME"); dir != "" {
		if data, err := os.ReadFile(filepath.Join(dir, "config.json")); err == nil {
			parseComposer(data, add)
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
		parse func([]byte, func(eco, url, scope string))
	}{
		{filepath.Join(home, ".npmrc"), parseNpmrc},
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
		{filepath.Join(home, ".gemrc"), parseGemrc},
		{filepath.Join(home, ".bundle", "config"), parseBundleConfig},
		{filepath.Join(home, ".Rprofile"), parseRprofile},
		{filepath.Join(home, ".config", "cabal", "config"), parseCabalRepositories},
		{filepath.Join(home, ".cabal", "config"), parseCabalRepositories},
	} {
		if data, err := os.ReadFile(f.path); err == nil {
			f.parse(data, add)
		}
	}
}

// project reads what the repository asks for. Nothing here is trusted: it says where a
// package comes from, and a source nobody on this machine knows is worth seeing.
//
// Implements: REQ-SUP-015, REQ-SUP-018
func (c *Config) project(files []*scan.File) {
	add := func(eco, url, scope string) { c.Add(eco, Source{URL: url, Scope: scope, Origin: OriginProject}) }
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
			parsePipConf(data, add)
		case strings.HasPrefix(base, "requirements") && strings.HasSuffix(base, ".txt"):
			parseRequirements(data, add)
		case base == "pyproject.toml":
			parsePyproject(data, add)
		case base == "nuget.config":
			parseNuGetConfig(data, add)
		case base == "pom.xml":
			parsePom(data, add)
		case base == "composer.json":
			parseComposer(data, add)
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
			parseCargoConfig(data, add)
		case f.Path == "Podfile" || strings.HasSuffix(f.Path, "/Podfile"):
			parsePodfile(data, add)
		case base == "podfile.lock":
			parsePodfileLock(data, add)
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

// parsePipConf reads index-url and extra-index-url from a pip configuration file.
func parsePipConf(data []byte, add func(eco, url, scope string)) {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "index-url", "index_url":
			add(PyPI, strings.TrimSpace(value), "")
		case "extra-index-url", "extra_index_url":
			for _, u := range strings.Fields(value) {
				add(PyPI, u, "")
			}
		}
	}
}

// parseRequirements reads the index options a requirements file may carry.
func parseRequirements(data []byte, add func(eco, url, scope string)) {
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
		case "--index-url", "-i", "--extra-index-url":
			add(PyPI, value, "")
		}
	}
}

// parsePyproject reads the indexes Poetry and uv declare.
func parsePyproject(data []byte, add func(eco, url, scope string)) {
	var doc struct {
		Tool struct {
			Poetry struct {
				Source []struct{ Name, URL string }
			}
			UV struct {
				Index []struct{ Name, URL string }
			} `toml:"uv"`
		}
	}
	if _, err := toml.Decode(string(data), &doc); err != nil {
		return
	}
	for _, s := range doc.Tool.Poetry.Source {
		add(PyPI, s.URL, "")
	}
	for _, s := range doc.Tool.UV.Index {
		add(PyPI, s.URL, "")
	}
}

// parseCargoConfig reads replaced sources and alternative registries.
//
// The first source added is the one packages resolve from, so the order is fixed:
// the source crates.io is replaced with (following replace-with to the end), then
// every other source and registry by name. Ranging over the maps as they come would
// pick a different one from run to run.
func parseCargoConfig(data []byte, add func(eco, url, scope string)) {
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
	if s, ok := doc.Source[name]; ok {
		add(Cargo, s.Registry, "")
	} else if r, ok := doc.Registries[name]; ok {
		add(Cargo, r.Index, "")
	}
	for _, key := range slices.Sorted(maps.Keys(doc.Source)) {
		add(Cargo, doc.Source[key].Registry, "")
	}
	for _, key := range slices.Sorted(maps.Keys(doc.Registries)) {
		add(Cargo, doc.Registries[key].Index, "")
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

// parseNuGetConfig reads the package sources of a NuGet configuration.
func parseNuGetConfig(data []byte, add func(eco, url, scope string)) {
	var doc struct {
		PackageSources nugetSources `xml:"packageSources"`
	}
	if xml.Unmarshal(data, &doc) != nil {
		return
	}
	for _, s := range doc.PackageSources.Add {
		add(NuGet, s.Value, "")
	}
}

// parsePom reads the repositories a Maven project declares.
func parsePom(data []byte, add func(eco, url, scope string)) {
	var doc struct {
		Repositories struct {
			Repository []struct {
				URL string `xml:"url"`
			} `xml:"repository"`
		} `xml:"repositories"`
	}
	if xml.Unmarshal(data, &doc) != nil {
		return
	}
	for _, r := range doc.Repositories.Repository {
		add(Maven, strings.TrimSpace(r.URL), "")
	}
}

// parseComposer reads the Composer repositories a composer.json or Composer's global
// config.json declares: those of type "composer" (Private Packagist, Satis, a
// Repman or Nexus proxy) are package indexes. A VCS, path or inline package
// repository is not an index, and `"packagist.org": false` names none. The list form
// and the object form (keyed by name) are both read, in order and by name.
func parseComposer(data []byte, add func(eco, url, scope string)) {
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
	var repos []json.RawMessage
	if json.Unmarshal(doc.Repositories, &repos) != nil {
		var named map[string]json.RawMessage
		if json.Unmarshal(doc.Repositories, &named) != nil {
			return
		}
		for _, key := range slices.Sorted(maps.Keys(named)) {
			repos = append(repos, named[key])
		}
	}
	for _, raw := range repos {
		var r repository
		if json.Unmarshal(raw, &r) == nil && r.Type == "composer" {
			add(Composer, strings.TrimSpace(r.URL), "")
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

// parseMavenSettings reads the mirrors this machine sends Maven through.
func parseMavenSettings(data []byte, add func(eco, url, scope string)) {
	var doc struct {
		Mirrors struct {
			Mirror []struct {
				URL string `xml:"url"`
			} `xml:"mirror"`
		} `xml:"mirrors"`
	}
	if xml.Unmarshal(data, &doc) != nil {
		return
	}
	for _, m := range doc.Mirrors.Mirror {
		add(Maven, strings.TrimSpace(m.URL), "")
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

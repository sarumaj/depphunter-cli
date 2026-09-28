package index

import (
	"encoding/json"
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
	"github.com/sarumaj/depphunter-cli/internal/npmconf"
	"github.com/sarumaj/depphunter-cli/internal/scan"
	"github.com/sarumaj/depphunter-cli/internal/userconf"
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
	cfg *Config
	// m is whose configuration is read, and where each tool keeps it.
	m userconf.Machine
	// read says the machine's configuration has been read: it is this user's, it
	// does not change with the repository, and --watch discovers on every analysis.
	read bool
}

func NewDiscoverer(env func(string) string, home string) *Discoverer {
	return &Discoverer{cfg: New(), m: userconf.New(home, env)}
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
		d.cfg.machine(d.m)
		d.read = true
	}
	d.cfg.forgetProject()
	d.cfg.project(files)
	return d.cfg
}

// machine reads the configuration of whoever is running depphunter: environment first,
// then the files their package managers read, each found where that package manager
// finds it (internal/userconf).
//
// Implements: REQ-SUP-015, REQ-SUP-064
func (c *Config) machine(m userconf.Machine) {
	env, home := m.Env, m.Home
	k := sink{
		put: func(eco string, s Source) {
			s.Trusted, s.Origin = true, OriginMachine
			c.Add(eco, s)
		},
		off: func(eco string) { c.SwitchOff(eco, OriginMachine) },
	}
	add := k.add
	read := func(name string, parse func([]byte, sink)) {
		if name == "" {
			return
		}
		if data, err := os.ReadFile(name); err == nil {
			parse(data, k)
		}
	}

	// npm: the environment's settings, then the user's npmrc, then the global one.
	npmEnv := m.NpmEnv()
	add(NPM, npmEnv["registry"], "")
	for _, key := range slices.Sorted(maps.Keys(npmEnv)) {
		if scope, ok := strings.CutSuffix(key, ":registry"); ok && strings.HasPrefix(scope, "@") {
			add(NPM, npmEnv[key], scope)
		}
	}
	read(m.NpmUserConfig(), plain(parseNpmrc))
	read(m.NpmGlobalConfig(), plain(parseNpmrc))
	machineYarnBun(m, k)
	machinePip(m, k)
	c.machinePython(m, k)
	parseGoproxy(m.GoEnv("GOPROXY"), k)
	// Where the container tools pull images from: registries.conf's registries and
	// mirrors, the Docker daemon's mirrors of Docker Hub.
	c.oci = readOCIConf(m)
	// Cargo: the registries the environment defines, before config.toml's, since a
	// variable overrides the same registry's index there.
	cargoEnv := m.CargoRegistries()
	for _, name := range slices.Sorted(maps.Keys(cargoEnv)) {
		k.put(Cargo, Source{URL: cargoEnv[name], Registry: strings.ToLower(name), Kind: Additive})
	}
	read(m.CargoFile("config"), func(data []byte, k sink) { cargoConfig(data, k, cargoEnv) })
	// Composer's global configuration, in the one home Composer takes.
	read(join(m.ComposerHome(), "config.json"), parseComposer)
	// NuGet: the user's NuGet.Config, then the machine-wide ones, the same the
	// credentials are read from, so a feed with a password is also a feed. They
	// are merged with the repository's nuget.config files (see applyNuGet).
	c.m, c.nugetMachine = m, nil
	for _, name := range append(m.NuGetConfigs(), m.NuGetMachineConfigs()...) {
		if data, err := os.ReadFile(name); err == nil {
			if f, ok := nuget.ParseConfig(data); ok {
				c.nugetMachine = append(c.nugetMachine, f)
			}
		}
	}
	c.applyNuGet(nil)
	// Bundler's mirror of rubygems.org, from the environment and the user's config.
	add(RubyGems, env("BUNDLE_MIRROR__RUBYGEMS__ORG"), "")
	read(m.BundlerConfig(), plain(parseBundleConfig))
	// The server dart pub and flutter pub get install from instead of pub.dev.
	add(Pub, env("PUB_HOSTED_URL"), "")
	// The Hex API Mix and rebar3 talk to instead of hex.pm's (HEX_API_URL, HEX_API,
	// hex.config's api_url). HEX_MIRROR is not read: a mirror serves the
	// repository's signed protobuf files, not this API.
	add(Hex, m.HexAPIURL(), "")
	// The Hex repositories rebar3 asks, before hex.pm's, for every project: a
	// rebar3 project's packages are asked of them too (see hexRepositories).
	c.rebar3Repos, c.rebar3Replace = m.ReadRebar3HexRepos()
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
	machineJVM(m, k)
	machineDub(m, k)
	machineQuicklisp(m, k)
	machineCPAN(m, k)
	machineOpam(m, k)
	machineAlire(m, k)
	machineJulia(m, k)
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
		{filepath.Join(home, ".gemrc"), plain(parseGemrc)},
		{filepath.Join(home, ".Rprofile"), plain(parseRprofile)},
		{filepath.Join(home, ".config", "cabal", "config"), plain(parseCabalRepositories)},
		{filepath.Join(home, ".cabal", "config"), plain(parseCabalRepositories)},
	} {
		read(f.path, f.parse)
	}
	// Bazel's user rc and what it imports; %workspace% means no workspace here.
	readBazelrc(filepath.Join(home, ".bazelrc"), "", "", add, c.bazelHelper(false), map[string]bool{})
}

// join is filepath.Join, or "" when dir is: nothing is read under a directory that
// is not there.
func join(dir, name string) string {
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, name)
}

// machinePip reads pip's configuration as pip layers it: its configuration files in
// the order it loads them, each setting of a later file replacing the same setting of
// an earlier one, then PIP_INDEX_URL and PIP_EXTRA_INDEX_URL over them all. Only the
// last extra-index-url is kept, as in pip: the setting is replaced, not added to.
//
// Implements: REQ-SUP-015, REQ-SUP-064
func machinePip(m userconf.Machine, k sink) {
	var index string
	var extra []string
	files, ok := m.PipConfigFiles()
	for _, name := range files {
		if !ok || name == "" {
			break
		}
		data, err := os.ReadFile(name)
		if err != nil {
			continue
		}
		s := pipSettings(data)
		if s.hasIndex {
			index = s.index
		}
		if s.hasExtra {
			extra = s.extra
		}
	}
	if v := m.Env("PIP_INDEX_URL"); v != "" {
		index = v
	}
	if v := m.Env("PIP_EXTRA_INDEX_URL"); v != "" {
		extra = strings.Fields(v)
	}
	k.add(PyPI, index, "")
	for _, u := range extra {
		k.extra(PyPI, u)
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
	// The repository's nuget.config files are merged with this machine's before
	// anything else names a NuGet feed, so a feed both name stays this machine's.
	c.applyNuGet(projectNuGet(ordered))
	python := c.projectPython(ordered)
	for _, f := range ordered {
		base := strings.ToLower(path.Base(f.Path))
		if base == "nuget.config" {
			continue
		}
		data, err := os.ReadFile(f.Abs)
		if err != nil {
			continue
		}
		if python(f, base, data, k) {
			continue
		}
		switch {
		case base == ".npmrc":
			parseNpmrc(data, add)
		case base == ".yarnrc.yml" || base == strings.ToLower(c.yarnRCFilename()):
			c.projectYarnrc(data, add)
		case base == ".yarnrc":
			addNpmSettings(npmconf.ParseYarnClassic(data), add, nil)
		case base == "bunfig.toml":
			c.projectBunfig(data, add)
		case base == "pip.conf", base == "pip.ini":
			parsePipConf(data, k)
		case strings.HasPrefix(base, "requirements") && strings.HasSuffix(base, ".txt"):
			parseRequirements(data, k)
		case base == "paket.dependencies":
			parsePaketSources(data, k)
			c.lendPaket(data)
		case base == "paket.lock":
			parsePaketLock(data, k)
		case base == "pom.xml":
			parsePom(data, k)
		case base == "build.gradle", base == "build.gradle.kts", base == "settings.gradle", base == "settings.gradle.kts":
			parseGradleRepos(data, k)
		case strings.HasSuffix(base, ".sbt") && path.Base(path.Dir(f.Path)) != "project":
			parseSbtResolvers(data, k)
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
			if c.biocRelease == "" {
				c.biocRelease = renvBioconductor(data)
			}
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
			root := strings.TrimSuffix(f.Abs, filepath.FromSlash(f.Path))
			readBazelrc(f.Abs, bazelWorkspace(root, filepath.Dir(f.Abs)), root, add, c.bazelHelper(true), map[string]bool{})
		case base == "dub.settings.json":
			parseDubSettings(data, k)
		case base == "qlfile":
			parseQlfile(data, k)
		case base == "cpanfile.snapshot":
			c.readCPANSnapshot(data)
		case base == "dune-workspace":
			parseDuneWorkspace(data, k)
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

// parsePipConf reads a pip configuration file: index-url replaces PyPI, and each URL
// of extra-index-url (on the line, or on the indented lines that continue it) is
// asked beside it.
//
// Implements: REQ-SUP-015, REQ-SUP-063
func parsePipConf(data []byte, k sink) {
	s := pipSettings(data)
	k.add(PyPI, s.index, "")
	for _, u := range s.extra {
		k.extra(PyPI, u)
	}
}

// pipConf is what one pip configuration file sets.
type pipConf struct {
	index              string
	extra              []string
	hasIndex, hasExtra bool
}

// pipSettings reads index-url and extra-index-url (on the line, or on the indented
// lines that continue it) out of a pip configuration file.
func pipSettings(data []byte) pipConf {
	var s pipConf
	extra := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if extra && trimmed != "" && (line[0] == ' ' || line[0] == '\t') && !strings.Contains(trimmed, "=") {
			s.extra = append(s.extra, strings.Fields(trimmed)...)
			continue
		}
		extra = false
		key, value, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "index-url", "index_url":
			s.index, s.hasIndex = strings.TrimSpace(value), true
		case "extra-index-url", "extra_index_url":
			extra, s.hasExtra = true, true
			s.extra = strings.Fields(value)
		}
	}
	return s
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

// parseCargoConfig reads what replaces crates.io and the alternative registries.
//
// crates.io is replaced by whatever `[source.crates-io] replace-with` ends at,
// following the chain; that source serves every crate crates.io would. An
// alternative registry (`[registries.<name>]`) is not a replacement: it serves only
// the crates that declare it (lang.Target.Registry), and is recorded under its name
// for them. A [source] no chain reaches serves nothing.
//
// Implements: REQ-SUP-015, REQ-SUP-063
func parseCargoConfig(data []byte, k sink) { cargoConfig(data, k, nil) }

// cargoConfig is parseCargoConfig with the registries the environment defines
// (userconf.Machine.CargoRegistries), whose index replaces config.toml's. Those were
// recorded already; what a replace-with chain ends at is taken from them too.
func cargoConfig(data []byte, k sink, env map[string]string) {
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
	} else if u, ok := env[userconf.CargoRegistryName(name)]; ok && name != "crates-io" {
		k.add(Cargo, u, "")
	} else if r, ok := doc.Registries[name]; ok {
		k.add(Cargo, r.Index, "")
	}
	for _, key := range slices.Sorted(maps.Keys(doc.Registries)) {
		if _, ok := env[userconf.CargoRegistryName(key)]; ok {
			continue
		}
		k.put(Cargo, Source{URL: doc.Registries[key].Index, Registry: key, Kind: Additive})
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

// addNuGetFeed records a feed asked beside nuget.org, unless the feed is nuget.org
// itself (by any of its addresses): the public default, which is not recorded again.
func addNuGetFeed(feed string, k sink) {
	if u, official, ok := nugetFeed(feed); ok && !official {
		k.extra(NuGet, u)
	}
}

// nugetFeed reads a NuGet source's value: ok for an http(s) feed (a local folder is
// no index), official for nuget.org itself, by any of its addresses.
func nugetFeed(feed string) (u string, official, ok bool) {
	p, err := url.Parse(strings.TrimSpace(strings.Trim(feed, `"`)))
	if err != nil || p.Scheme != "https" && p.Scheme != "http" {
		return "", false, false
	}
	h := strings.ToLower(p.Hostname())
	return p.String(), h == "nuget.org" || strings.HasSuffix(h, ".nuget.org"), true
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
		clojureRepos(r, k)
	}
}

// clojureRepos records the repositories of one :mvn/repos or :repositories form:
// {"name" {:url "..."}}, {"name" "url"}, [["name" "url"]] or [["name" {:url "..."}]].
// Maven Central and Clojars are the public indexes and are not recorded; the others
// are asked beside them, except one named "central", which tools.deps and
// Leiningen take in place of Maven Central.
//
// Implements: REQ-SUP-056, REQ-SUP-063
func clojureRepos(r *edn.Node, k sink) {
	if r == nil {
		return
	}
	var names, values []*edn.Node
	switch r.Kind {
	case edn.Map:
		for i := 1; i < len(r.Kids); i += 2 {
			names, values = append(names, r.Kids[i-1]), append(values, r.Kids[i])
		}
	case edn.Vector, edn.List:
		for _, e := range r.Kids {
			if len(e.Kids) == 2 {
				names, values = append(names, e.Kids[0]), append(values, e.Kids[1])
			}
		}
	}
	for i, v := range values {
		u := v
		if v.Kind == edn.Map {
			u = v.Get("url")
		}
		if u == nil || u.Kind != edn.String || MavenPublic(u.Text) {
			continue
		}
		if n := names[i]; n.Kind == edn.String && n.Text == "central" {
			k.add(Maven, strings.TrimSpace(u.Text), "")
		} else {
			k.extra(Maven, strings.TrimSpace(u.Text))
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

// parseBundleConfig reads Bundler's mirror of rubygems.org from the user's config
// (~/.bundle/config, or where BUNDLE_USER_CONFIG or BUNDLE_USER_HOME put it:
// BUNDLE_MIRROR__RUBYGEMS__ORG, or the URL form of the key). Its credentials are
// read by internal/auth.
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

// renvBioconductor is the Bioconductor release renv.lock records
// (Bioconductor.Version, "3.18"), whose repositories renv restores the project's
// Bioconductor packages from; "" when it records none. The first renv.lock in path
// order that records one sets it for the repository.
//
// Implements: REQ-SUP-048
func renvBioconductor(data []byte) string {
	var doc struct {
		Bioconductor struct{ Version string }
	}
	if json.Unmarshal(data, &doc) != nil || !biocVersion.MatchString(doc.Bioconductor.Version) {
		return ""
	}
	return doc.Bioconductor.Version
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

// parseBazelrc reads the registries a .bazelrc names (--registry=URL, in any
// command's section, in order). The Bazel Central Registry itself is the public
// index, and a file:// registry cannot be asked over the network: neither is added.
// The file an import or try-import line names is handed to include, where the line
// stands, so that its registries come in Bazel's order. The scope of each
// --credential_helper (`[<scope>=]<helper>`, "" for every host) is handed to
// helper: helpers are programs, and are not run (REQ-BAZEL-011).
//
// Implements: REQ-SUP-057, REQ-BAZEL-011
func parseBazelrc(data []byte, add func(eco, url, scope string), helper func(string), include func(string)) {
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		switch {
		case (fields[0] == "import" || fields[0] == "try-import") && len(fields) > 1:
			if include != nil {
				include(strings.Trim(fields[1], `"'`))
			}
			continue
		case fields[0] == "try-import-if-bazel-version" && len(fields) > 2:
			// The version condition is not evaluated: its registries are read.
			if include != nil {
				include(strings.Trim(fields[2], `"'`))
			}
			continue
		}
		for i, f := range fields {
			if strings.HasPrefix(f, "#") {
				break
			}
			if h, ok := bazelFlag(fields, i, "--credential_helper"); ok && helper != nil {
				if scope, _, scoped := strings.Cut(h, "="); scoped {
					helper(strings.ToLower(scope))
				} else {
					helper("")
				}
			}
			u, ok := bazelFlag(fields, i, "--registry")
			if ok && strings.HasPrefix(u, "http") && !BazelCentral(u) {
				add(Bazel, u, "")
			}
		}
	}
}

// readBazelrc reads a .bazelrc and the files its import and try-import lines name:
// %workspace% is the workspace's directory (a line using it is skipped when there is
// none, as for ~/.bazelrc); any other relative path is Bazel's from the directory it
// runs in, taken to be the workspace's, else - when no such file is there, or there
// is no workspace - the importing file's directory. within, when set, is the
// directory an imported file must be in: a repository's .bazelrc does not get a file
// outside the repository read. A file missing (try-import's case, and import's,
// which Bazel fails on) or read already is passed over.
//
// Implements: REQ-SUP-057
func readBazelrc(name, workspace, within string, add func(eco, url, scope string), helper func(string), seen map[string]bool) {
	name = filepath.Clean(name)
	if seen[name] || len(seen) > 64 {
		return
	}
	seen[name] = true
	data, err := os.ReadFile(name)
	if err != nil {
		return
	}
	parseBazelrc(data, add, helper, func(p string) {
		if strings.Contains(p, "%workspace%") {
			if workspace == "" {
				return
			}
			p = strings.ReplaceAll(p, "%workspace%", filepath.ToSlash(workspace))
		}
		p = filepath.FromSlash(p)
		if !filepath.IsAbs(p) {
			rel := p
			p = filepath.Join(filepath.Dir(name), rel)
			if workspace != "" {
				if _, err := os.Stat(filepath.Join(workspace, rel)); err == nil {
					p = filepath.Join(workspace, rel)
				}
			}
		}
		if rel, err := filepath.Rel(within, filepath.Clean(p)); within != "" && (err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return
		}
		readBazelrc(p, workspace, within, add, helper, seen)
	})
}

// bazelFlag is the value of a .bazelrc flag at fields[i], `--flag=value` or
// `--flag value`, unquoted.
func bazelFlag(fields []string, i int, flag string) (string, bool) {
	v, ok := strings.CutPrefix(fields[i], flag+"=")
	if !ok && fields[i] == flag && i+1 < len(fields) {
		v, ok = fields[i+1], true
	}
	return strings.Trim(v, `"'`), ok
}

// bazelHelper is a credential helper a .bazelrc names: its scope, "" for every
// host, a domain, or `*.<domain>` for the domain and every host below it.
type bazelHelper struct {
	scope   string
	project bool
}

// bazelHelper records the credential helpers of a .bazelrc, the repository's or
// this machine's.
func (c *Config) bazelHelper(project bool) func(string) {
	return func(scope string) {
		h := bazelHelper{scope, project}
		if !slices.Contains(c.bazelHelpers, h) {
			c.bazelHelpers = append(c.bazelHelpers, h)
		}
	}
}

// bazelHelperFor is the scope of a credential helper a .bazelrc names for a
// host, the most specific first, as Bazel picks one; ok is false when none
// covers it.
//
// Implements: REQ-BAZEL-011
func (c *Config) bazelHelperFor(host string) (scope string, ok bool) {
	host = strings.ToLower(host)
	best := -1
	for _, h := range c.bazelHelpers {
		rank := -1
		switch domain, wild := strings.CutPrefix(h.scope, "*."); {
		case h.scope == "":
			rank = 0
		case !wild && h.scope == host:
			rank = 1 << 20
		case wild && (host == domain || strings.HasSuffix(host, "."+domain)):
			rank = len(domain)
		}
		if rank > best {
			best, scope = rank, h.scope
		}
	}
	return scope, best >= 0
}

// bazelWorkspace is the Bazel workspace a repository's .bazelrc belongs to: the
// nearest directory at or above it holding MODULE.bazel, REPO.bazel, WORKSPACE or
// WORKSPACE.bazel, up to the repository's root, else the root.
func bazelWorkspace(root, dir string) string {
	for d := dir; ; d = filepath.Dir(d) {
		for _, marker := range []string{"MODULE.bazel", "REPO.bazel", "WORKSPACE", "WORKSPACE.bazel"} {
			if _, err := os.Stat(filepath.Join(d, marker)); err == nil {
				return d
			}
		}
		if rel, err := filepath.Rel(root, d); err != nil || rel == "." || strings.HasPrefix(rel, "..") || filepath.Dir(d) == d {
			return root
		}
	}
}

// dubSettings is what a dub settings.json says about registries.
type dubSettings struct {
	RegistryURLs []string `json:"registryUrls"`
	SkipRegistry string   `json:"skipRegistry"`
}

// machineDub reads the registries dub asks on this machine, in dub's order: those of
// `DUB_REGISTRY` (`;`-separated), then the `registryUrls` of its settings files, the
// user's before the system's, each asked before code.dlang.org. The `skipRegistry`
// of the file with priority that sets one is honoured: `standard` switches
// code.dlang.org off, `configured` the settings' registries too, `all` every one.
//
// Implements: REQ-SUP-060, REQ-SUP-064
func machineDub(m userconf.Machine, k sink) {
	var urls []string
	skip := ""
	for _, name := range m.DubSettings() {
		data, err := os.ReadFile(name)
		if err != nil {
			continue
		}
		var s dubSettings
		if json.Unmarshal(data, &s) != nil {
			continue
		}
		urls = append(urls, s.RegistryURLs...)
		if skip == "" {
			skip = s.SkipRegistry
		}
	}
	if skip != "all" {
		for _, u := range strings.Split(m.Env("DUB_REGISTRY"), ";") {
			addDubRegistry(k, u)
		}
	}
	if skip != "all" && skip != "configured" {
		for _, u := range urls {
			addDubRegistry(k, u)
		}
	}
	if skip == "standard" || skip == "configured" || skip == "all" {
		k.off(Dub)
	}
}

// parseDubSettings reads a dub settings.json - a package's dub.settings.json, which
// dub reads for the root package - as machineDub reads the machine's.
//
// Implements: REQ-SUP-060
func parseDubSettings(data []byte, k sink) {
	var s dubSettings
	if json.Unmarshal(data, &s) != nil {
		return
	}
	if s.SkipRegistry != "configured" && s.SkipRegistry != "all" {
		for _, u := range s.RegistryURLs {
			addDubRegistry(k, u)
		}
	}
	if s.SkipRegistry == "standard" || s.SkipRegistry == "configured" || s.SkipRegistry == "all" {
		k.off(Dub)
	}
}

// addDubRegistry records a dub registry asked before code.dlang.org. Only a registry
// served over HTTP is: dub's file:// and mvn+ package suppliers have no API to ask,
// and code.dlang.org is the public index already.
func addDubRegistry(k sink, u string) {
	u = strings.TrimSpace(u)
	if !strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "http://") {
		return
	}
	if strings.TrimRight(u, "/") == public[Dub] {
		return
	}
	k.extra(Dub, u)
}

// ultralispDist is the dist a qlfile's ultralisp lines install from, as Qlot names
// it (over HTTPS here).
const ultralispDist = "https://dist.ultralisp.org/"

// parseQlfile reads the dists a qlfile adds: `dist <url> [version]` and `dist <name>
// <url> [version]` are asked beside the Quicklisp dist; `ultralisp <project>` names
// the Ultralisp dist as that project's. The Quicklisp dist itself is the public
// index.
//
// Implements: REQ-SUP-062
func parseQlfile(data []byte, k sink) {
	for _, line := range strings.Split(string(data), "\n") {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		switch strings.ToLower(f[0]) {
		case "dist":
			u := f[1]
			if !strings.Contains(u, "/") && len(f) > 2 {
				u = f[2] // dist <name> <url>
			}
			if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
				if !quicklispDist(u) {
					k.extra(Quicklisp, u)
				}
			}
		case "ultralisp":
			if !strings.EqualFold(f[1], ":all") {
				k.add(Quicklisp, ultralispDist, strings.ToLower(f[1]))
			}
		}
	}
}

// quicklispDist reports whether a dist URL is the Quicklisp dist itself, over
// either scheme.
func quicklispDist(u string) bool {
	u = strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
	return u == "beta.quicklisp.org/dist/quicklisp.txt"
}

// machineQuicklisp reads the dists installed in this machine's Quicklisp: each
// distinfo.txt's distinfo-subscription-url, where the dist's current version is
// published, is asked beside the Quicklisp dist.
//
// Implements: REQ-SUP-062, REQ-SUP-064
func machineQuicklisp(m userconf.Machine, k sink) {
	for _, name := range m.QuicklispDists() {
		data, err := os.ReadFile(name)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			key, v, ok := strings.Cut(line, ":")
			if !ok || strings.TrimSpace(key) != "distinfo-subscription-url" {
				continue
			}
			if v = strings.TrimSpace(v); (strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://")) && !quicklispDist(v) {
				k.extra(Quicklisp, v)
			}
		}
	}
}

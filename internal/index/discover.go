package index

import (
	"encoding/xml"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

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
	if home == "" {
		return
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
		{filepath.Join(home, ".nuget", "NuGet", "NuGet.Config"), parseNuGetConfig},
	} {
		if data, err := os.ReadFile(f.path); err == nil {
			f.parse(data, add)
		}
	}
}

// project reads what the repository asks for. Nothing here is trusted: it says where a
// package comes from, and a source nobody on this machine knows is worth seeing.
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
		case base == "config.toml" && strings.HasSuffix(path.Dir(f.Path), ".cargo"):
			parseCargoConfig(data, add)
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

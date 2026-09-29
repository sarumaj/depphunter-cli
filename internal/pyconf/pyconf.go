// Package pyconf reads the index settings of the Python package managers beside
// pip: uv (pyproject.toml's [tool.uv], uv.toml, UV_* variables), Poetry
// ([[tool.poetry.source]], its config.toml and auth.toml, POETRY_* variables),
// Pipenv (Pipfile's [[source]], Pipfile.lock) and PDM ([[tool.pdm.source]], its
// config.toml and pdm.toml, PDM_PYPI_* variables). It says what a file names, as
// written; whether an index may be trusted and whether a credential may be sent is
// for the caller (internal/index for the indexes, internal/auth for this machine's
// credentials) to decide.
package pyconf

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// Kind is how an index relates to PyPI, in the tool's own words.
type Kind uint8

const (
	// Extra is asked beside PyPI (or what replaces it): uv's index, pip's
	// extra-index-url, a Pipfile source after the first, a PDM source.
	Extra Kind = iota
	// Default replaces PyPI: uv's default index, a Poetry source of priority
	// "default", the first Pipfile source, PDM's "pypi".
	Default
	// Primary replaces PyPI together with the other primary sources, asked in
	// order: Poetry's primary sources.
	Primary
	// Explicit serves only the packages pinned to it: uv's explicit index,
	// Poetry's explicit source.
	Explicit
	// Supplemental is asked after the primary sources (or PyPI), for what none
	// of them has: Poetry's supplemental sources and its legacy secondary ones.
	Supplemental
)

// Index is one index a configuration names, with its credential as written.
type Index struct {
	Name, URL string
	Kind      Kind
	// Include are PDM's include_packages patterns: the packages matching one are
	// asked of this index alone.
	Include            []string
	Username, Password string
}

// Settings are what one file says: its indexes in order, and the index each pinned
// package is to come from (package -> index name).
type Settings struct {
	Indexes []Index
	Pins    map[string]string
	// NoImplicitPyPI says PyPI is asked only where an index of the file names it:
	// Poetry drops its implicit PyPI source when the file declares a primary
	// source or PyPI itself.
	NoImplicitPyPI bool
}

// Named is the index of the given name ("" and false for none). uv and PDM compare
// names as written, Poetry and Pipenv too; case is kept.
func (s Settings) Named(name string) (Index, bool) {
	for _, i := range s.Indexes {
		if i.Name == name && name != "" {
			return i, true
		}
	}
	return Index{}, false
}

func (s *Settings) pin(packageName, index string) {
	if packageName == "" || index == "" {
		return
	}
	if s.Pins == nil {
		s.Pins = map[string]string{}
	}
	s.Pins[packageName] = index
}

// EnvironmentName is a name as uv's and Poetry's variables spell it: upper case, every
// character other than a letter or digit as "_" (UV_INDEX_<NAME>_USERNAME,
// POETRY_HTTP_BASIC_<NAME>_PASSWORD).
func EnvironmentName(name string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r - 'a' + 'A'
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		}
		return '_'
	}, name)
}

// ---------------------------------------------------------------- uv

type uvIndex struct {
	Name, URL         string
	Default, Explicit bool
}

type uvOptions struct {
	Index         []uvIndex
	IndexURL      string   `toml:"index-url"`
	ExtraIndexURL []string `toml:"extra-index-url"`
	Sources       map[string]any
}

// UV reads uv's index settings out of a uv.toml, or (pyproject) out of the
// [tool.uv] table of a pyproject.toml: the [[index]] entries in order (default =
// true replacing PyPI, explicit = true serving only the packages pinned to it), the
// legacy index-url and extra-index-url, and - in a pyproject.toml - the packages
// [tool.uv.sources] pins to an index by name ({ index = "name" }, or a list of such
// entries with markers).
func UV(data []byte, pyproject bool) (Settings, bool) {
	var o uvOptions
	if pyproject {
		var doc struct{ Tool struct{ UV uvOptions } }
		if _, err := toml.Decode(string(data), &doc); err != nil {
			return Settings{}, false
		}
		o = doc.Tool.UV
	} else if _, err := toml.Decode(string(data), &o); err != nil {
		return Settings{}, false
	}
	var s Settings
	if o.IndexURL != "" {
		s.Indexes = append(s.Indexes, Index{URL: o.IndexURL, Kind: Default})
	}
	for _, i := range o.Index {
		kind := Extra
		switch {
		case i.Explicit:
			kind = Explicit
		case i.Default:
			kind = Default
		}
		s.Indexes = append(s.Indexes, Index{Name: i.Name, URL: i.URL, Kind: kind})
	}
	for _, u := range o.ExtraIndexURL {
		s.Indexes = append(s.Indexes, Index{URL: u})
	}
	if !pyproject {
		return s, true
	}
	for _, packageName := range slices.Sorted(maps.Keys(o.Sources)) {
		var specs []map[string]any
		switch v := o.Sources[packageName].(type) {
		case map[string]any:
			specs = append(specs, v)
		case []map[string]any:
			specs = v
		case []any:
			for _, e := range v {
				if m, ok := e.(map[string]any); ok {
					specs = append(specs, m)
				}
			}
		}
		for _, spec := range specs {
			if index, ok := spec["index"].(string); ok {
				s.pin(packageName, index)
				break
			}
		}
	}
	return s, true
}

// UVEnvironment reads uv's index variables, the first to win first: UV_DEFAULT_INDEX (or
// the legacy UV_INDEX_URL) replacing PyPI, then the space-separated UV_INDEX (and
// the legacy UV_EXTRA_INDEX_URL) asked beside it. An entry of either may be
// name=url.
func UVEnvironment(environment func(string) string) []Index {
	var out []Index
	add := func(v string, kind Kind) {
		for _, e := range strings.Fields(v) {
			i := Index{URL: e, Kind: kind}
			if name, u, ok := strings.Cut(e, "="); ok && name != "" && !strings.ContainsAny(name, ":/?") {
				i.Name, i.URL = name, u
			}
			out = append(out, i)
			if kind == Default {
				return // one default
			}
		}
	}
	add(environment("UV_DEFAULT_INDEX"), Default)
	add(environment("UV_INDEX_URL"), Default)
	add(environment("UV_INDEX"), Extra)
	add(environment("UV_EXTRA_INDEX_URL"), Extra)
	return out
}

// UVMachine are the indexes this machine's uv configuration names, the one
// that wins first: UV_DEFAULT_INDEX, UV_INDEX_URL, UV_INDEX and UV_EXTRA_INDEX_URL,
// then the user's uv.toml and the system's (userconf UVConfigFiles).
func UVMachine(m userconf.Machine) []Index {
	out := UVEnvironment(m.Environment)
	for _, f := range m.UVConfigFiles() {
		if data, err := os.ReadFile(f); err == nil {
			if s, ok := UV(data, false); ok {
				out = append(out, s.Indexes...)
			}
		}
	}
	return out
}

// UVCredential is what UV_INDEX_<NAME>_USERNAME and UV_INDEX_<NAME>_PASSWORD hold
// for the index of that name.
func UVCredential(environment func(string) string, name string) (user, pass string) {
	if name == "" || environment == nil {
		return "", ""
	}
	n := EnvironmentName(name)
	return environment("UV_INDEX_" + n + "_USERNAME"), environment("UV_INDEX_" + n + "_PASSWORD")
}

// ---------------------------------------------------------------- Poetry

// Poetry reads the [[tool.poetry.source]] entries of a pyproject.toml by priority, in
// the order Poetry asks them - "default" (Poetry 1.x, or the legacy default = true)
// first, then "primary" (the priority of a source that states none), then the
// legacy "secondary" (or secondary = true) and "supplemental"; "explicit" ones serve
// only their pins - and the dependencies of [tool.poetry.dependencies] and of every
// group that name a source with source = "<name>". A source named PyPI without a
// URL is PyPI itself, at its place in the order; its URL is left empty. Declaring
// PyPI, or any primary or default source, drops Poetry's implicit PyPI.
func Poetry(data []byte) (Settings, bool) {
	var doc struct {
		Tool struct {
			Poetry struct {
				Source []struct {
					Name, URL, Priority string
					Default, Secondary  bool
				}
				Dependencies map[string]any
				Group        map[string]struct{ Dependencies map[string]any }
			}
		}
	}
	if _, err := toml.Decode(string(data), &doc); err != nil {
		return Settings{}, false
	}
	p := doc.Tool.Poetry
	var s Settings
	var defaults, primary, secondary, supplemental []Index
	for _, source := range p.Source {
		i := Index{Name: source.Name, URL: source.URL, Kind: Primary}
		switch priority := strings.ToLower(source.Priority); {
		case priority == "default" || source.Default && priority == "":
			i.Kind = Default
			defaults = append(defaults, i)
		case priority == "explicit":
			i.Kind = Explicit
			primary = append(primary, i)
		case priority == "secondary" || source.Secondary && priority == "":
			i.Kind = Supplemental
			secondary = append(secondary, i)
		case priority == "supplemental":
			i.Kind = Supplemental
			supplemental = append(supplemental, i)
		default:
			primary = append(primary, i)
		}
		if i.Kind == Default || i.Kind == Primary || PoetryPyPI(i) {
			s.NoImplicitPyPI = true
		}
	}
	s.Indexes = slices.Concat(defaults, primary, secondary, supplemental)
	dependencies := []map[string]any{p.Dependencies}
	for _, g := range slices.Sorted(maps.Keys(p.Group)) {
		dependencies = append(dependencies, p.Group[g].Dependencies)
	}
	for _, d := range dependencies {
		for _, name := range slices.Sorted(maps.Keys(d)) {
			if spec, ok := d[name].(map[string]any); ok {
				if source, ok := spec["source"].(string); ok {
					s.pin(name, source)
				}
			}
		}
	}
	return s, true
}

// PoetryPyPI reports whether a Poetry source is PyPI itself: named PyPI (in any
// case) with no URL.
func PoetryPyPI(i Index) bool { return i.URL == "" && strings.EqualFold(i.Name, "pypi") }

// Credential is a user name and password.
type Credential struct{ Username, Password string }

// PoetryMachine is what this machine's Poetry configuration says: the URL of each
// repository config.toml and the POETRY_REPOSITORIES_<NAME>_URL variables define,
// and the http-basic credential of each name (config.toml, auth.toml over it, the
// POETRY_HTTP_BASIC_<NAME>_USERNAME/_PASSWORD variables over both, each half on
// its own), all by EnvironmentName. A password Poetry keeps in the system keyring is not
// read.
func PoetryMachine(m userconf.Machine) (repositories map[string]string, credentials map[string]Credential) {
	repositories, credentials = map[string]string{}, map[string]Credential{}
	directory := m.PoetryConfigDirectory()
	for _, f := range []string{"config.toml", "auth.toml"} {
		if directory == "" {
			break
		}
		var doc struct {
			Repositories map[string]struct{ URL string }
			HTTPBasic    map[string]Credential `toml:"http-basic"`
		}
		if _, err := toml.DecodeFile(filepath.Join(directory, f), &doc); err != nil {
			continue
		}
		for name, r := range doc.Repositories {
			if r.URL != "" {
				repositories[EnvironmentName(name)] = r.URL
			}
		}
		for name, c := range doc.HTTPBasic {
			credentials[EnvironmentName(name)] = c
		}
	}
	for name, u := range m.PoetryRepositoryVariables() {
		repositories[EnvironmentName(name)] = u
	}
	names := slices.Collect(maps.Keys(repositories))
	for name := range credentials {
		names = append(names, name)
	}
	for _, name := range names {
		c := credentials[name]
		if user := m.Environment("POETRY_HTTP_BASIC_" + name + "_USERNAME"); user != "" {
			c.Username = user
		}
		if pass := m.Environment("POETRY_HTTP_BASIC_" + name + "_PASSWORD"); pass != "" {
			c.Password = pass
		}
		if c.Username != "" || c.Password != "" {
			credentials[name] = c
		}
	}
	return repositories, credentials
}

// PoetryCredential is this machine's credential for the Poetry source of a name:
// the file's (see PoetryMachine) or, for a name no file mentions,
// POETRY_HTTP_BASIC_<NAME>_USERNAME/_PASSWORD.
func PoetryCredential(m userconf.Machine, credentials map[string]Credential, name string) Credential {
	n := EnvironmentName(name)
	if c, ok := credentials[n]; ok {
		return c
	}
	if m.Environment == nil || name == "" {
		return Credential{}
	}
	return Credential{m.Environment("POETRY_HTTP_BASIC_" + n + "_USERNAME"), m.Environment("POETRY_HTTP_BASIC_" + n + "_PASSWORD")}
}

// ---------------------------------------------------------------- Pipenv

// Pipfile reads a Pipfile's [[source]] entries - the first replacing PyPI, the
// others asked beside it, as Pipenv passes them to pip - and the packages of every
// category ([packages], [dev-packages] and any other) pinned to a source with
// index = "<name>". URLs are as written, ${VAR} references included.
func Pipfile(data []byte) (Settings, bool) {
	var doc map[string]any
	if _, err := toml.Decode(string(data), &doc); err != nil {
		return Settings{}, false
	}
	var s Settings
	sources, _ := doc["source"].([]map[string]any)
	for i, source := range sources {
		name, _ := source["name"].(string)
		u, _ := source["url"].(string)
		kind := Extra
		if i == 0 {
			kind = Default
		}
		s.Indexes = append(s.Indexes, Index{Name: name, URL: u, Kind: kind})
	}
	for _, category := range slices.Sorted(maps.Keys(doc)) {
		switch category {
		case "source", "requires", "pipenv", "scripts":
			continue
		}
		packages, _ := doc[category].(map[string]any)
		for _, name := range slices.Sorted(maps.Keys(packages)) {
			if spec, ok := packages[name].(map[string]any); ok {
				index, _ := spec["index"].(string)
				s.pin(name, index)
			}
		}
	}
	return s, true
}

// PipfileLock reads Pipfile.lock: the sources of _meta.sources, as Pipfile's, and
// the locked packages of every category that name one with "index".
func PipfileLock(data []byte) (Settings, bool) {
	var doc map[string]json.RawMessage
	if json.Unmarshal(data, &doc) != nil {
		return Settings{}, false
	}
	var metadata struct {
		Sources []struct{ Name, URL string }
	}
	if json.Unmarshal(doc["_meta"], &metadata) != nil {
		return Settings{}, false
	}
	var s Settings
	for i, source := range metadata.Sources {
		kind := Extra
		if i == 0 {
			kind = Default
		}
		s.Indexes = append(s.Indexes, Index{Name: source.Name, URL: source.URL, Kind: kind})
	}
	for _, category := range slices.Sorted(maps.Keys(doc)) {
		if category == "_meta" {
			continue
		}
		var packages map[string]struct{ Index string }
		if json.Unmarshal(doc[category], &packages) != nil {
			continue
		}
		for _, name := range slices.Sorted(maps.Keys(packages)) {
			s.pin(name, packages[name].Index)
		}
	}
	return s, true
}

// ---------------------------------------------------------------- PDM

// PDMPyPI is the name of PDM's default index: a source of that name replaces PyPI.
const PDMPyPI = "pypi"

// PDM reads the [[tool.pdm.source]] entries of a pyproject.toml: one named "pypi"
// replaces PyPI, every other is asked beside it, and a find_links source is no
// index. include_packages is kept (Index.Include); exclude_packages is not read.
func PDM(data []byte) (Settings, bool) {
	var doc struct {
		Tool struct {
			PDM struct {
				Source []struct {
					Name, URL, Type string
					Username        string
					Password        string
					Include         []string `toml:"include_packages"`
				}
			} `toml:"pdm"`
		}
	}
	if _, err := toml.Decode(string(data), &doc); err != nil {
		return Settings{}, false
	}
	var s Settings
	for _, source := range doc.Tool.PDM.Source {
		if source.Type != "" && source.Type != "index" {
			continue
		}
		kind := Extra
		if source.Name == PDMPyPI {
			kind = Default
		}
		s.Indexes = append(s.Indexes, Index{Name: source.Name, URL: source.URL, Kind: kind, Include: source.Include,
			Username: source.Username, Password: source.Password})
	}
	return s, true
}

// PDMConfig reads the index settings of a PDM configuration file (the global
// config.toml, or a project's pdm.toml): [pypi] url, username and password for the
// index replacing PyPI (named "pypi"), and [pypi.<name>] for each other index,
// asked beside it (a find_links one is no index). Indexes come in name order,
// "pypi" first.
func PDMConfig(data []byte) (Settings, bool) {
	var doc struct{ PyPI map[string]any }
	if _, err := toml.Decode(string(data), &doc); err != nil {
		return Settings{}, false
	}
	stringField := func(m map[string]any, k string) string { v, _ := m[k].(string); return v }
	s := Settings{}
	if u := stringField(doc.PyPI, "url"); u != "" || stringField(doc.PyPI, "username") != "" {
		s.Indexes = append(s.Indexes, Index{Name: PDMPyPI, URL: u, Kind: Default,
			Username: stringField(doc.PyPI, "username"), Password: stringField(doc.PyPI, "password")})
	}
	for _, name := range slices.Sorted(maps.Keys(doc.PyPI)) {
		t, ok := doc.PyPI[name].(map[string]any)
		if !ok || name == PDMPyPI {
			continue
		}
		if typeName := stringField(t, "type"); typeName != "" && typeName != "index" {
			continue
		}
		s.Indexes = append(s.Indexes, Index{Name: name, URL: stringField(t, "url"),
			Username: stringField(t, "username"), Password: stringField(t, "password")})
	}
	return s, true
}

// PDMMachine is this machine's PDM configuration: the global config.toml
// (userconf PDMConfigFile) with PDM_PYPI_URL, PDM_PYPI_USERNAME and
// PDM_PYPI_PASSWORD over its [pypi] settings.
func PDMMachine(m userconf.Machine) Settings {
	var s Settings
	if f := m.PDMConfigFile(); f != "" {
		if data, err := os.ReadFile(f); err == nil {
			s, _ = PDMConfig(data)
		}
	}
	u, user, pass := m.Environment("PDM_PYPI_URL"), m.Environment("PDM_PYPI_USERNAME"), m.Environment("PDM_PYPI_PASSWORD")
	if u == "" && user == "" && pass == "" {
		return s
	}
	if len(s.Indexes) == 0 || s.Indexes[0].Name != PDMPyPI {
		s.Indexes = append([]Index{{Name: PDMPyPI, Kind: Default}}, s.Indexes...)
	}
	p := &s.Indexes[0]
	if u != "" {
		p.URL = u
	}
	if user != "" {
		p.Username = user
	}
	if pass != "" {
		p.Password = pass
	}
	return s
}

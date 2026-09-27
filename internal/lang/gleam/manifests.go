package gleam

import (
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// dependency is one entry of gleam.toml's [dependencies] or [dev-dependencies],
// or of manifest.toml's [requirements]: a Hex requirement ("~> 1.0",
// ">= 0.34.0 and < 2.0.0"), a { path = ".." } or a { git = "..", ref = ".." }.
type dependency struct {
	name, req, path, git, ref string
	dev                       bool
}

// config is what gleam.toml says about a package.
type config struct {
	name string
	deps map[string]*dependency
}

// locked is a package of manifest.toml: source "hex" with its version, "git" with
// its repo and commit, or "local" with its path; requirements name other packages
// of the same manifest.
type locked struct {
	name, version, source, repo, commit, path, otpApp string
	requirements                                      []string
}

// manifest is manifest.toml: every package the build uses, directly or not, and
// the requirements it was resolved from.
type manifest struct {
	packages     map[string]*locked
	requirements map[string]*dependency
}

// readConfig reads gleam.toml. A file that is not TOML gives what it could read:
// nothing.
//
// Implements: REQ-GLEAM-005
func readConfig(src []byte) *config {
	var raw map[string]any
	c := &config{deps: map[string]*dependency{}}
	if _, err := toml.Decode(string(src), &raw); err != nil {
		return c
	}
	c.name, _ = raw["name"].(string)
	for _, sec := range []string{"dependencies", "dev-dependencies", "dev_dependencies"} {
		table, _ := raw[sec].(map[string]any)
		for name, v := range table {
			if d := readDependency(name, v); d != nil && c.deps[name] == nil {
				d.dev = sec != "dependencies"
				c.deps[name] = d
			}
		}
	}
	return c
}

// readDependency reads a requirement written as a string or as a table.
func readDependency(name string, v any) *dependency {
	d := &dependency{name: name}
	switch v := v.(type) {
	case string:
		d.req = strings.TrimSpace(v)
	case map[string]any:
		str := func(k string) string { s, _ := v[k].(string); return strings.TrimSpace(s) }
		d.req, d.path, d.git, d.ref = str("version"), str("path"), str("git"), str("ref")
	default:
		return nil
	}
	return d
}

// readManifest reads manifest.toml. Only a manifest Gleam wrote counts: one whose
// packages list build tools or whose requirements table exists.
//
// Implements: REQ-GLEAM-006
func readManifest(src []byte) *manifest {
	var raw struct {
		Packages []struct {
			Name         string   `toml:"name"`
			Version      string   `toml:"version"`
			Source       string   `toml:"source"`
			Repo         string   `toml:"repo"`
			Commit       string   `toml:"commit"`
			Path         string   `toml:"path"`
			OTPApp       string   `toml:"otp_app"`
			BuildTools   []string `toml:"build_tools"`
			Requirements []string `toml:"requirements"`
		} `toml:"packages"`
		Requirements map[string]any `toml:"requirements"`
	}
	if _, err := toml.Decode(string(src), &raw); err != nil {
		return nil
	}
	m := &manifest{packages: map[string]*locked{}, requirements: map[string]*dependency{}}
	gleam := raw.Requirements != nil
	for _, p := range raw.Packages {
		if p.Name == "" {
			continue
		}
		gleam = gleam || len(p.BuildTools) > 0
		m.packages[p.Name] = &locked{name: p.Name, version: p.Version, source: p.Source, repo: p.Repo,
			commit: p.Commit, path: p.Path, otpApp: p.OTPApp, requirements: p.Requirements}
	}
	for name, v := range raw.Requirements {
		if d := readDependency(name, v); d != nil {
			m.requirements[name] = d
		}
	}
	if !gleam {
		return nil
	}
	return m
}

// extractConfig makes each dependency of gleam.toml an import of the package it
// names.
//
// Implements: REQ-GLEAM-005
func extractConfig(src []byte) *lang.Extraction {
	c := readConfig(src)
	lines := keyLines(src)
	ex := &lang.Extraction{}
	for _, name := range sortedKeys(c.deps) {
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: name, Module: name, Name: kindDep, Line: lines[name]})
	}
	if c.name != "" {
		ex.Symbols = []lang.Symbol{{Name: c.name, Kind: "package", Line: lines["name"]}}
	}
	return ex
}

// extractManifest makes each package of manifest.toml an import, so what the
// build installs is on the map even when no module imports it.
//
// Implements: REQ-GLEAM-006
func extractManifest(src []byte) *lang.Extraction {
	m := readManifest(src)
	ex := &lang.Extraction{}
	if m == nil {
		return ex
	}
	lines := packageLines(src)
	for _, name := range sortedKeys(m.packages) {
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: name, Module: name, Name: kindLocked, Line: lines[name]})
	}
	return ex
}

var (
	keyLine     = regexp.MustCompile(`^\s*"?([A-Za-z0-9_\-]+)"?\s*=`)
	inlineKey   = regexp.MustCompile(`[{,]\s*"?([A-Za-z0-9_\-]+)"?\s*=`)
	packageLine = regexp.MustCompile(`name\s*=\s*"([^"]+)"`)
)

// keyLines is the first line each key is written on, inline tables included.
func keyLines(src []byte) map[string]int {
	out := map[string]int{}
	for i, l := range strings.Split(string(src), "\n") {
		keys := inlineKey.FindAllStringSubmatch(l, -1)
		if m := keyLine.FindStringSubmatch(l); m != nil {
			keys = append([][]string{m}, keys...)
		}
		for _, k := range keys {
			if _, ok := out[k[1]]; !ok {
				out[k[1]] = i + 1
			}
		}
	}
	return out
}

// packageLines is the line of each `{ name = "x", ... }` entry of manifest.toml.
func packageLines(src []byte) map[string]int {
	out := map[string]int{}
	for i, l := range strings.Split(string(src), "\n") {
		if m := packageLine.FindStringSubmatch(l); m != nil {
			if _, ok := out[m[1]]; !ok {
				out[m[1]] = i + 1
			}
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

package gleam

import (
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// dependency is one entry of gleam.toml's [dependencies] or [dev-dependencies],
// or of manifest.toml's [requirements]: a Hex requirement ("~> 1.0",
// ">= 0.34.0 and < 2.0.0"), a { path = ".." } or a { git = "..", ref = ".." }.
type dependency struct {
	name, requirement, path, git, reference string
	dev                                     bool
}

// config is what gleam.toml says about a package.
type config struct {
	name         string
	dependencies map[string]*dependency
}

// locked is a package of manifest.toml: source "hex" with its version, "git" with
// its repository and commit, or "local" with its path; requirements name other packages
// of the same manifest.
type locked struct {
	name, version, source, repository, commit, path, otpApp string
	requirements                                            []string
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
func readConfig(source []byte) *config {
	var raw map[string]any
	c := &config{dependencies: map[string]*dependency{}}
	if _, err := toml.Decode(string(source), &raw); err != nil {
		return c
	}
	c.name, _ = raw["name"].(string)
	for _, section := range []string{"dependencies", "dev-dependencies", "dev_dependencies"} {
		table, _ := raw[section].(map[string]any)
		for name, v := range table {
			if d := readDependency(name, v); d != nil && c.dependencies[name] == nil {
				d.dev = section != "dependencies"
				c.dependencies[name] = d
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
		d.requirement = strings.TrimSpace(v)
	case map[string]any:
		stringField := func(k string) string { s, _ := v[k].(string); return strings.TrimSpace(s) }
		d.requirement, d.path, d.git, d.reference = stringField("version"), stringField("path"), stringField("git"), stringField("ref")
	default:
		return nil
	}
	return d
}

// readManifest reads manifest.toml. Only a manifest Gleam wrote counts: one whose
// packages list build tools or whose requirements table exists.
//
// Implements: REQ-GLEAM-006
func readManifest(source []byte) *manifest {
	var raw struct {
		Packages []struct {
			Name         string   `toml:"name"`
			Version      string   `toml:"version"`
			Source       string   `toml:"source"`
			Repository   string   `toml:"repo"`
			Commit       string   `toml:"commit"`
			Path         string   `toml:"path"`
			OTPApp       string   `toml:"otp_app"`
			BuildTools   []string `toml:"build_tools"`
			Requirements []string `toml:"requirements"`
		} `toml:"packages"`
		Requirements map[string]any `toml:"requirements"`
	}
	if _, err := toml.Decode(string(source), &raw); err != nil {
		return nil
	}
	m := &manifest{packages: map[string]*locked{}, requirements: map[string]*dependency{}}
	gleam := raw.Requirements != nil
	for _, p := range raw.Packages {
		if p.Name == "" {
			continue
		}
		gleam = gleam || len(p.BuildTools) > 0
		m.packages[p.Name] = &locked{name: p.Name, version: p.Version, source: p.Source, repository: p.Repository,
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
func extractConfig(source []byte) *lang.Extraction {
	c := readConfig(source)
	lines := keyLines(source)
	extraction := &lang.Extraction{}
	for _, name := range lang.SortedKeys(c.dependencies) {
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: name, Module: name, Name: kindDependency, Line: lines[name]})
	}
	if c.name != "" {
		extraction.Symbols = []lang.Symbol{{Name: c.name, Kind: "package", Line: lines["name"]}}
	}
	return extraction
}

// extractManifest makes each package of manifest.toml an import, so what the
// build installs is on the map even when no module imports it.
//
// Implements: REQ-GLEAM-006
func extractManifest(source []byte) *lang.Extraction {
	m := readManifest(source)
	extraction := &lang.Extraction{}
	if m == nil {
		return extraction
	}
	lines := packageLines(source)
	for _, name := range lang.SortedKeys(m.packages) {
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: name, Module: name, Name: kindLocked, Line: lines[name]})
	}
	return extraction
}

var (
	keyLine     = regexp.MustCompile(`^\s*"?([A-Za-z0-9_\-]+)"?\s*=`)
	inlineKey   = regexp.MustCompile(`[{,]\s*"?([A-Za-z0-9_\-]+)"?\s*=`)
	packageLine = regexp.MustCompile(`name\s*=\s*"([^"]+)"`)
)

// keyLines is the first line each key is written on, inline tables included.
func keyLines(source []byte) map[string]int {
	out := map[string]int{}
	for i, l := range strings.Split(string(source), "\n") {
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
func packageLines(source []byte) map[string]int {
	out := map[string]int{}
	for i, l := range strings.Split(string(source), "\n") {
		if m := packageLine.FindStringSubmatch(l); m != nil {
			if _, ok := out[m[1]]; !ok {
				out[m[1]] = i + 1
			}
		}
	}
	return out
}

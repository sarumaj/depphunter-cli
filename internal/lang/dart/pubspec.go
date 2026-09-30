package dart

import (
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/yamlnode"
)

// A pub package is a directory with a pubspec.yaml: its name, what it depends on
// (dependencies, dev_dependencies and dependency_overrides, each hosted, git, path or
// sdk), the pub workspace it roots or belongs to, and what pubspec.lock resolved.

// dependency is one entry of a pubspec's dependency sections.
type dependency struct {
	name       string
	section    string // dependencies, dev_dependencies, dependency_overrides
	constraint string // "^1.2.3", ">=1.0.0 <2.0.0", "1.2.3", "any"; "" for none
	source     string // hosted, git, path, sdk
	path       string // a path dependency's directory, as written
	url        string // a git URL, or a hosted dependency's server
	reference  string // a git reference
	spec       string // how the entry is shown
	line       int
}

type pubspec struct {
	name          string
	dependencies  []*dependency
	workspace     []string // member directories, as written
	resolution    string   // "workspace" for a member of a pub workspace
	melos         bool     // the pubspec configures melos (melos 7 keeps it here)
	melosPackages []string // and the package globs it lists
	memberLine    map[string]int
}

// readPubspec reads a pubspec.yaml (or pubspec_overrides.yaml, which has only
// dependency_overrides).
//
// Implements: REQ-DART-006
func readPubspec(source []byte) (*pubspec, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(source, &doc); err != nil {
		return nil, err
	}
	p := &pubspec{memberLine: map[string]int{}}
	for _, entry := range yamlnode.Pairs(&doc) {
		key, value := entry.Key.Value, entry.Value
		switch key {
		case "name":
			p.name = value.Value
		case "resolution":
			p.resolution = value.Value
		case "workspace":
			for _, m := range yamlnode.Items(value) {
				if m.Kind == yaml.ScalarNode && m.Value != "" {
					p.workspace = append(p.workspace, m.Value)
					p.memberLine[m.Value] = m.Line
				}
			}
		case "melos":
			p.melos = true
			for _, glob := range yamlnode.Scalars(yamlnode.Get(value, "packages")) {
				p.melosPackages = append(p.melosPackages, glob.Value)
			}
		case "dependencies", "dev_dependencies", "dependency_overrides":
			for _, dependency := range yamlnode.Pairs(value) {
				p.dependencies = append(p.dependencies, readDependency(key, dependency.Key, dependency.Value))
			}
		}
	}
	return p, nil
}

// readDependency reads one dependency: a bare constraint, or a map naming its source.
func readDependency(section string, k, v *yaml.Node) *dependency {
	d := &dependency{name: k.Value, section: section, source: "hosted", line: k.Line}
	switch v.Kind {
	case yaml.ScalarNode:
		if v.Tag != "!!null" {
			d.constraint = v.Value
		}
	case yaml.MappingNode:
		d.constraint = yamlnode.Scalar(v, "version")
		if n := yamlnode.Get(v, "path"); n != nil {
			d.source, d.path = "path", n.Value
		}
		if n := yamlnode.Get(v, "sdk"); n != nil {
			d.source, d.url = "sdk", n.Value
		}
		if n := yamlnode.Get(v, "git"); n != nil {
			d.source = "git"
			if n.Kind == yaml.ScalarNode {
				d.url = n.Value
			} else {
				d.url, d.reference = yamlnode.Scalar(n, "url"), yamlnode.Scalar(n, "ref")
			}
		}
		if n := yamlnode.Get(v, "hosted"); n != nil {
			if n.Kind == yaml.ScalarNode {
				d.url = n.Value
			} else {
				d.url = yamlnode.Scalar(n, "url")
			}
		}
	}
	d.spec = d.name
	switch {
	case d.source == "path":
		d.spec += " (path: " + d.path + ")"
	case d.source == "sdk":
		d.spec += " (sdk: " + d.url + ")"
	case d.source == "git" && d.reference != "":
		d.spec += " (git: " + d.url + " " + d.reference + ")"
	case d.source == "git":
		d.spec += " (git: " + d.url + ")"
	case d.url != "":
		d.spec += ": " + orAny(d.constraint) + " (hosted: " + d.url + ")"
	default:
		d.spec += ": " + orAny(d.constraint)
	}
	if section == "dependency_overrides" {
		d.spec = "override " + d.spec
	}
	return d
}

func orAny(c string) string {
	if c == "" {
		return "any"
	}
	return c
}

// locked is a package pubspec.lock records.
type locked struct {
	name, version     string
	source            string // hosted, git, path, sdk
	dependency        string // "direct main", "direct dev", "direct overridden", "transitive"
	path              string // a path package's directory, relative to the lock when relative
	relative          bool
	url               string // a git URL, or the hosted server
	resolvedReference string // the commit a git package was resolved to
}

// readLock reads pubspec.lock: one flat list of every package the solve selected,
// direct or not, with its exact version and where it came from. It records no edges
// between packages.
//
// Implements: REQ-DART-007
func readLock(source []byte) map[string]*locked {
	var doc struct {
		Packages map[string]struct {
			Dependency  string    `yaml:"dependency"`
			Description yaml.Node `yaml:"description"`
			Source      string    `yaml:"source"`
			Version     string    `yaml:"version"`
		} `yaml:"packages"`
	}
	if yaml.Unmarshal(source, &doc) != nil {
		return nil
	}
	out := make(map[string]*locked, len(doc.Packages))
	for name, p := range doc.Packages {
		l := &locked{name: name, version: p.Version, source: p.Source, dependency: p.Dependency}
		d := &p.Description
		switch p.Source {
		case "path":
			l.path = yamlnode.Scalar(d, "path")
			l.relative = yamlnode.Scalar(d, "relative") == "true"
		case "git":
			l.url, l.resolvedReference = yamlnode.Scalar(d, "url"), yamlnode.Scalar(d, "resolved-ref")
		case "hosted":
			l.url = yamlnode.Scalar(d, "url")
		}
		out[name] = l
	}
	return out
}

// pubPinned is pub's reading of a version constraint: a bare version ("1.2.3",
// "1.2.3+4", "2.0.0-dev.1") allows only itself; a caret, a range, "any" and no
// constraint at all float.
//
// Implements: REQ-DART-008
func pubPinned(constraint string) bool {
	c := strings.TrimSpace(constraint)
	return c != "" && c != "any" && lang.Pinned(c) && strings.Count(core(c), ".") == 2
}

func core(v string) string {
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		return v[:i]
	}
	return v
}

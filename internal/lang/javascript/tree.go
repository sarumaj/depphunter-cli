package javascript

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// A lock file records not only which version of a package is installed but what that
// package itself pulls in, which is the whole transitive graph - already on disk, with
// no registry to ask. tree collects it: package name -> the names it depends on.
//
// Versions are not part of the key. The map draws one building per package, so two
// versions of the same package are one node, and an edge between names is what the
// graph can hold anyway.
type tree struct {
	deps   map[string]map[string]bool
	locked map[string]string // package -> a version some lock file pinned it to
}

func newTree() *tree {
	return &tree{deps: map[string]map[string]bool{}, locked: map[string]string{}}
}

func (t *tree) add(pkg string, deps ...string) {
	if pkg == "" {
		return
	}
	m := t.deps[pkg]
	if m == nil {
		m = map[string]bool{}
		t.deps[pkg] = m
	}
	for _, d := range deps {
		if d != "" && d != pkg {
			m[d] = true
		}
	}
}

// Dependencies implements lang.Transitive.
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecoNPM {
		return nil
	}
	var out []lang.Target
	for dep := range r.tree.deps[t.Package] {
		version := r.tree.locked[dep]
		out = append(out, lang.Target{
			Ecosystem: ecoNPM, Package: dep, Version: version,
			// It is in a lock file, which is what pins an npm package.
			Pinned: version != "",
		})
	}
	return out
}

// addPackageLockTree reads the dependency edges of package-lock.json, v1 through v3.
func (t *tree) addPackageLockTree(abs string) {
	var lock struct {
		// v2 and v3 list every installed path under "packages".
		Packages map[string]struct {
			Version              string
			Dependencies         map[string]string
			OptionalDependencies map[string]string
		}
		// v1 nests them under "dependencies", with "requires" for the edges.
		Dependencies map[string]struct {
			Version  string
			Requires map[string]string
		}
	}
	if readJSON(abs, &lock) != nil {
		return
	}
	for key, p := range lock.Packages {
		name := lockName(key)
		if name == "" {
			continue // the project itself, whose direct imports are already known
		}
		if p.Version != "" {
			t.locked[name] = p.Version
		}
		for _, m := range []map[string]string{p.Dependencies, p.OptionalDependencies} {
			for dep := range m {
				t.add(name, dep)
			}
		}
	}
	for name, p := range lock.Dependencies {
		if p.Version != "" {
			t.locked[name] = p.Version
		}
		for dep := range p.Requires {
			t.add(name, dep)
		}
	}
}

// lockName reads the package name from a package-lock path key: "node_modules/x" and
// the nested "node_modules/a/node_modules/b" both name their last segment.
func lockName(key string) string {
	i := strings.LastIndex(key, "node_modules/")
	if i < 0 {
		return ""
	}
	return key[i+len("node_modules/"):]
}

// addPnpmTree reads the dependency edges of pnpm-lock.yaml. v5 to v8 keep them under
// "packages", v9 moved them to "snapshots"; both key entries by name and version.
func (t *tree) addPnpmTree(abs string) {
	data, err := os.ReadFile(abs)
	if err != nil {
		return
	}
	var doc struct {
		Packages map[string]struct {
			Name, Version string
			Dependencies  map[string]string `yaml:"dependencies"`
		} `yaml:"packages"`
		Snapshots map[string]struct {
			Dependencies map[string]string `yaml:"dependencies"`
		} `yaml:"snapshots"`
	}
	if yaml.Unmarshal(data, &doc) != nil {
		return
	}
	for key, p := range doc.Packages {
		name, version := pnpmKey(key)
		if p.Name != "" { // v5 spells them out instead of packing them into the key
			name, version = p.Name, p.Version
		}
		if version != "" {
			t.locked[name] = version
		}
		for dep := range p.Dependencies {
			t.add(name, dep)
		}
	}
	for key, s := range doc.Snapshots {
		name, _ := pnpmKey(key)
		for dep := range s.Dependencies {
			t.add(name, dep)
		}
	}
}

// pnpmKey splits "/lodash@4.17.21" or "react-dom@18.3.1(react@18.3.1)" into name and
// version. The parenthesised part is peer-dependency context, not the version.
func pnpmKey(key string) (name, version string) {
	key = strings.TrimPrefix(key, "/")
	if i := strings.Index(key, "("); i >= 0 {
		key = key[:i]
	}
	i := strings.LastIndex(key, "@")
	if i <= 0 { // a scope's "@" is at index 0 and is not the separator
		return key, ""
	}
	name, version = key[:i], key[i+1:]
	if strings.Contains(version, "/") { // "/@scope/pkg/1.2.3" of the oldest lock files
		return key, ""
	}
	return name, version
}

// addYarnTree reads the dependency edges of a classic yarn.lock, whose entries name
// their resolved version and then the ranges they in turn require.
func (t *tree) addYarnTree(abs string) {
	data, err := os.ReadFile(abs)
	if err != nil {
		return
	}
	var names []string
	inDeps := false
	for _, line := range strings.Split(string(data), "\n") {
		switch {
		case strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#"):
		case !strings.HasPrefix(line, " "):
			inDeps = false
			names = names[:0]
			for _, k := range strings.Split(strings.TrimSuffix(strings.TrimSpace(line), ":"), ",") {
				if n := yarnEntryName(strings.Trim(strings.TrimSpace(k), `"`)); n != "" {
					names = append(names, n)
				}
			}
		case strings.HasPrefix(strings.TrimSpace(line), "version"):
			v := strings.Trim(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "version")), `:" `)
			for _, n := range names {
				t.locked[n] = v
			}
		case strings.TrimSpace(line) == "dependencies:" || strings.TrimSpace(line) == "optionalDependencies:":
			inDeps = true
		case inDeps && strings.HasPrefix(line, "    "):
			dep := strings.Trim(strings.Fields(strings.TrimSpace(line))[0], `"`)
			for _, n := range names {
				t.add(n, dep)
			}
		default:
			inDeps = false
		}
	}
}

// yarnEntryName takes the package name out of a "name@range" descriptor.
func yarnEntryName(descriptor string) string {
	i := strings.LastIndex(descriptor, "@")
	if i <= 0 {
		return descriptor
	}
	return descriptor[:i]
}

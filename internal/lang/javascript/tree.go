package javascript

import (
	"strings"

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
//
// Implements: REQ-SUP-009
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
//
// Implements: REQ-SUP-009
func (t *tree) addPackageLockTree(lock *packageLock) {
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

// addPnpmTree reads the dependency edges of pnpm-lock.yaml.
//
// Implements: REQ-SUP-009
func (t *tree) addPnpmTree(doc *pnpmLock) {
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

// pnpmKey splits a package key into name and version. Version 6 onwards writes
// "/lodash@4.17.21" and "react-dom@18.3.1(react@18.3.1)", where the parenthesised
// part is peer-dependency context; version 5 wrote "/lodash/4.17.21" and
// "/@scope/pkg/1.2.3", where the last segment is the version.
func pnpmKey(key string) (name, version string) {
	key = strings.TrimPrefix(key, "/")
	if i := strings.Index(key, "("); i >= 0 {
		key = key[:i]
	}
	if i := strings.LastIndex(key, "@"); i > 0 { // a scope's "@" is at index 0
		return key[:i], key[i+1:]
	}
	if i := strings.LastIndex(key, "/"); i > 0 && startsWithDigit(key[i+1:]) {
		return key[:i], key[i+1:]
	}
	return key, ""
}

func startsWithDigit(s string) bool { return s != "" && s[0] >= '0' && s[0] <= '9' }

// addYarnTree reads the dependency edges of a classic yarn.lock, whose entries name
// their resolved version and then the ranges they in turn require.
//
// Implements: REQ-SUP-009
func (t *tree) addYarnTree(data []byte) {
	var names []string
	inDeps := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		fields := strings.Fields(trimmed)
		switch {
		case len(fields) == 0 || strings.HasPrefix(trimmed, "#"):
		case !strings.HasPrefix(line, " "): // the descriptors of a new entry
			inDeps, names = false, names[:0]
			for _, k := range strings.Split(strings.TrimSuffix(trimmed, ":"), ",") {
				if n := yarnEntryName(strings.Trim(strings.TrimSpace(k), `"`)); n != "" {
					names = append(names, n)
				}
			}
		// A dependency block is indented one level deeper than the entry's own keys,
		// which is the only thing telling "version-guard" apart from "version".
		case strings.HasPrefix(line, "    "):
			if inDeps {
				for _, n := range names {
					t.add(n, strings.Trim(fields[0], `"`))
				}
			}
		case trimmed == "dependencies:" || trimmed == "optionalDependencies:":
			inDeps = true
		default:
			inDeps = false
			if k, v := yarnField(line); k == "version" && v != "" {
				for _, n := range names {
					t.locked[n] = v
				}
			}
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

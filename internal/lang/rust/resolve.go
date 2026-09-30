package rust

import (
	"path"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"golang.org/x/mod/semver"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

var stdCrates = map[string]bool{"std": true, "core": true, "alloc": true, "proc_macro": true, "test": true}

// dependency is one Cargo dependency as the code names it (dashes become underscores).
type dependency struct {
	packageName string // package name on crates.io (differs from the key when renamed)
	version     string
	path        string // project-relative directory of a path dependency
	// registry is the alternative registry the dependency declares: its name
	// (registry = "corp") or its index URL (registry-index = "...").
	registry string
}

type crate struct {
	directory    string // directory holding Cargo.toml
	dependencies map[string]dependency
}

type resolver struct {
	files   map[string]bool
	crates  []*crate          // deepest first, so a file finds its own crate
	members map[string]string // normalized package name -> crate dir (workspace / path crates)
	// locked is every version of each package Cargo.lock holds. A lock file often
	// holds two of one crate - syn 1 and syn 2, a windows-sys per major - and which
	// of them a dependency means is decided by its own requirement (see pick).
	locked map[string][]string
	tree   map[string][]locked // "package version" -> the crates it depends on
	// sources is the registry Cargo.lock says each "package version" came from,
	// when it is not crates.io: its index URL.
	sources map[string]string
	// gits is the checkout Cargo.lock says each "package version" came from when
	// its source is a git repository: "<repository URL>#<commit>" (Target.Git).
	gits map[string]string
}

// locked is one crate in Cargo.lock: its name, and its version when the lock names
// one - which it does exactly when that crate is there in more than one version.
type locked struct{ name, version string }

// Dependencies implements lang.Transitive: Cargo.lock resolves the whole crate graph,
// so the answer needs nothing but the file the project already carries.
//
// Implements: REQ-SUP-009
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != lang.EcosystemCrates {
		return nil
	}
	var out []lang.Target
	for _, dependency := range r.tree[t.Package+" "+r.pick(t.Package, t.Version)] {
		version := dependency.version
		if version == "" {
			version = r.pick(dependency.name, "")
		}
		out = append(out, lang.Target{
			Ecosystem: lang.EcosystemCrates, Package: dependency.name, Version: version, Pinned: version != "",
			Registry: r.sources[dependency.name+" "+version], Git: r.gits[dependency.name+" "+version],
		})
	}
	return out
}

// pick is the locked version of a package that a requirement means: the only one
// there is, the one it names exactly, or the newest one Cargo's default (caret)
// reading of it accepts. "" when the lock holds none.
//
// Implements: REQ-RS-007
func (r *resolver) pick(packageName, requirement string) string {
	versions := r.locked[packageName]
	if len(versions) <= 1 {
		if len(versions) == 1 {
			return versions[0]
		}
		return ""
	}
	trimmed := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(requirement), "^="))
	best := ""
	for _, v := range versions {
		if v == trimmed {
			return v
		}
		if (trimmed == "" || caret(trimmed, v)) && (best == "" || semver.Compare("v"+v, "v"+best) > 0) {
			best = v
		}
	}
	if best == "" { // a requirement this does not read (">=1, <3", "~1.2"): the newest
		for _, v := range versions {
			if best == "" || semver.Compare("v"+v, "v"+best) > 0 {
				best = v
			}
		}
	}
	return best
}

// caret reports whether version v satisfies the requirement req read as Cargo reads a
// bare one: the same leftmost non-zero component, and no older than req.
func caret(requirement, v string) bool {
	rp, vp := strings.Split(requirement, "."), strings.Split(v, ".")
	for i, part := range rp {
		if i >= len(vp) {
			return false
		}
		if part != vp[i] {
			return false
		}
		if part != "0" {
			break
		}
	}
	full := requirement + strings.Repeat(".0", max(0, 3-len(rp)))
	return semver.Compare("v"+v, "v"+full) >= 0
}

func normalize(name string) string { return strings.ReplaceAll(name, "-", "_") }

// Implements: REQ-RS-004, REQ-RS-006, REQ-RS-007
func newResolver(all []*scan.File) *resolver {
	r := &resolver{files: map[string]bool{}, members: map[string]string{}, locked: map[string][]string{},
		tree: map[string][]locked{}, sources: map[string]string{}, gits: map[string]string{}}
	workspaceDependencies := map[string]dependency{}
	type manifest struct {
		f   *scan.File
		doc map[string]any
	}
	var manifests []manifest
	for _, f := range all {
		r.files[f.Path] = true
		switch path.Base(f.Path) {
		case "Cargo.toml":
			var doc map[string]any
			if _, err := toml.DecodeFile(f.AbsolutePath, &doc); err == nil {
				manifests = append(manifests, manifest{f, doc})
				if workspace, ok := doc["workspace"].(map[string]any); ok {
					for k, v := range table(workspace["dependencies"]) {
						workspaceDependencies[normalize(k)] = parseDependency(k, v, path.Dir(f.Path))
					}
				}
			}
		case "Cargo.lock":
			var lock struct {
				Package []struct {
					Name, Version, Source string
					// Each entry lists what that crate needs, as "name" or
					// "name version": the transitive graph, already resolved.
					Dependencies []string
				}
			}
			if _, err := toml.DecodeFile(f.AbsolutePath, &lock); err == nil {
				for _, p := range lock.Package {
					r.locked[p.Name] = append(r.locked[p.Name], p.Version)
					if registrySpec := lockRegistry(p.Source); registrySpec != "" {
						r.sources[p.Name+" "+p.Version] = registrySpec
					}
					if g := lockGit(p.Source); g != "" {
						r.gits[p.Name+" "+p.Version] = g
					}
					for _, d := range p.Dependencies {
						// "name", or "name version" and possibly " (source)".
						fields := strings.Fields(d)
						if len(fields) == 0 || fields[0] == p.Name {
							continue
						}
						dependency := locked{name: fields[0]}
						if len(fields) > 1 {
							dependency.version = fields[1]
						}
						key := p.Name + " " + p.Version
						r.tree[key] = append(r.tree[key], dependency)
					}
				}
			}
		}
	}
	for _, m := range manifests {
		directory := path.Dir(m.f.Path)
		c := &crate{directory: directory, dependencies: map[string]dependency{}}
		if packageName, ok := m.doc["package"].(map[string]any); ok {
			if name, ok := packageName["name"].(string); ok {
				r.members[normalize(name)] = directory
			}
		}
		add := func(t map[string]any) {
			for k, v := range t {
				d := parseDependency(k, v, directory)
				if w, ok := v.(map[string]any); ok && w["workspace"] == true {
					if wd, ok := workspaceDependencies[normalize(k)]; ok {
						d = wd
					}
				}
				c.dependencies[normalize(k)] = d
			}
		}
		for _, key := range []string{"dependencies", "dev-dependencies", "build-dependencies"} {
			add(table(m.doc[key]))
		}
		for _, t := range table(m.doc["target"]) { // [target.'cfg(...)'.dependencies]
			for _, key := range []string{"dependencies", "dev-dependencies", "build-dependencies"} {
				add(table(table(t)[key]))
			}
		}
		r.crates = append(r.crates, c)
	}
	sort.Slice(r.crates, func(i, j int) bool { return len(r.crates[i].directory) > len(r.crates[j].directory) })
	return r
}

func table(v any) map[string]any {
	t, _ := v.(map[string]any)
	return t
}

// Implements: REQ-RS-004, REQ-RS-005
func parseDependency(key string, v any, directory string) dependency {
	d := dependency{packageName: key}
	switch v := v.(type) {
	case string:
		d.version = v
	case map[string]any:
		if s, ok := v["version"].(string); ok {
			d.version = s
		}
		if s, ok := v["package"].(string); ok {
			d.packageName = s
		}
		if s, ok := v["path"].(string); ok {
			d.path = path.Clean(path.Join(directory, s))
		}
		if s, ok := v["registry"].(string); ok {
			d.registry = s
		} else if s, ok := v["registry-index"].(string); ok {
			d.registry = s
		}
	}
	return d
}

// lockRegistry is the index URL of a Cargo.lock `source` naming a registry other
// than crates.io ("registry+<url>", "sparse+<url>"), else "".
//
// Implements: REQ-RS-010
func lockRegistry(source string) string {
	var u string
	switch {
	case strings.HasPrefix(source, "registry+"):
		u = strings.TrimPrefix(source, "registry+")
	case strings.HasPrefix(source, "sparse+"):
		u = strings.TrimPrefix(source, "sparse+")
	default:
		return "" // crates.io needs no name; a git or path source is no registry
	}
	u = strings.TrimRight(u, "/")
	if u == "https://github.com/rust-lang/crates.io-index" || u == "https://index.crates.io" {
		return ""
	}
	return u
}

// lockGit is the checkout a Cargo.lock `source` of a git dependency names,
// "git+<url>?rev=…#<commit>" read as "<url>#<commit>", else "".
//
// Implements: REQ-FND-026
func lockGit(source string) string {
	rest, ok := strings.CutPrefix(source, "git+")
	if !ok {
		return ""
	}
	u, commit, ok := strings.Cut(rest, "#")
	if !ok || !lang.Commit(commit) {
		return ""
	}
	u, _, _ = strings.Cut(u, "?")
	return u + "#" + commit
}

// target is the crates target of a registry dependency: the version Cargo.lock
// holds for it, and the registry it comes from when that is not crates.io.
//
// Implements: REQ-RS-007, REQ-RS-010
func (r *resolver) target(d dependency) lang.Target {
	t := lang.Target{Ecosystem: lang.EcosystemCrates, Package: d.packageName, Version: d.version, Registry: d.registry}
	if exact := r.pick(d.packageName, d.version); exact != "" {
		t.Version, t.Requested, t.Pinned = exact, d.version, true
		if t.Registry == "" {
			t.Registry = r.sources[d.packageName+" "+exact]
		}
		t.Git = r.gits[d.packageName+" "+exact]
	}
	return t
}

func (r *resolver) crateOf(file string) *crate {
	for _, c := range r.crates {
		if lang.Within(file, c.directory) {
			return c
		}
	}
	return nil
}

// moduleDirectory is where a file's child modules live: src/lib.rs and a/mod.rs own their
// directory, a/b.rs owns a/b/.
//
// Implements: REQ-RS-002
func moduleDirectory(file string) string {
	switch base := path.Base(file); base {
	case "lib.rs", "main.rs", "mod.rs":
		return path.Dir(file)
	default:
		return path.Join(path.Dir(file), strings.TrimSuffix(base, ".rs"))
	}
}

// Implements: REQ-RS-002, REQ-RS-003, REQ-RS-004, REQ-RS-005, REQ-RS-007, REQ-RS-008
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	segments := strings.Split(rawImport.Module, "::")
	c := r.crateOf(file)
	switch first := segments[0]; {
	case first == "crate":
		if c != nil {
			t, _ := r.probe(path.Join(c.directory, "src"), segments[1:])
			return t
		}
		return lang.Target{}
	case first == "self":
		t, _ := r.probe(moduleDirectory(file), segments[1:])
		return t
	case first == "super":
		directory := moduleDirectory(file)
		for len(segments) > 0 && segments[0] == "super" {
			directory, segments = path.Dir(directory), segments[1:]
		}
		t, _ := r.probe(directory, segments)
		return t
	case stdCrates[first]:
		return lang.Target{Ecosystem: ecosystemStd, Package: first}
	}
	// Edition 2018 paths may name a module of the current file directly.
	if t, ok := r.probe(moduleDirectory(file), segments[:1]); ok {
		if t2, ok := r.probe(moduleDirectory(file), segments); ok {
			return t2
		}
		return t
	}
	// Crates are lower case by convention; `use Enum::*` names a local item.
	if first := segments[0]; first != "" && first[0] >= 'A' && first[0] <= 'Z' {
		return lang.Target{}
	}
	name := normalize(segments[0])
	if c != nil {
		if d, ok := c.dependencies[name]; ok {
			if d.path != "" {
				return r.local(d.path, segments[1:])
			}
			if directory, ok := r.members[normalize(d.packageName)]; ok {
				return r.local(directory, segments[1:])
			}
			// Cargo reads a bare "1.2.3" as ^1.2.3, so a manifest never pins on its
			// own: only Cargo.lock says which version is built.
			return r.target(d)
		}
	}
	if directory, ok := r.members[name]; ok {
		return r.local(directory, segments[1:])
	}
	return lang.Target{Ecosystem: lang.EcosystemCrates, Package: segments[0], Unresolved: true}
}

// local resolves a path inside another project crate, falling back to the crate itself.
//
// Implements: REQ-RS-004
func (r *resolver) local(crateDirectory string, segments []string) lang.Target {
	if t, ok := r.probe(path.Join(crateDirectory, "src"), segments); ok && len(segments) > 0 {
		return t
	}
	return lang.Target{Local: crateDirectory}
}

// probe finds the module file for the longest prefix of segments under directory; with no
// segments it returns the crate or module root file.
//
// Implements: REQ-RS-002, REQ-RS-003
func (r *resolver) probe(directory string, segments []string) (lang.Target, bool) {
	if len(segments) == 0 {
		for _, root := range []string{"lib.rs", "main.rs", "mod.rs"} {
			if p := path.Join(directory, root); r.files[p] {
				return lang.Target{Local: p}, true
			}
		}
		if r.files[directory+".rs"] { // 2018 layout: a.rs owns a/
			return lang.Target{Local: directory + ".rs"}, true
		}
		return lang.Target{}, false
	}
	for n := len(segments); n > 0; n-- {
		p := path.Join(append([]string{directory}, segments[:n]...)...)
		for _, candidate := range []string{p + ".rs", path.Join(p, "mod.rs")} {
			if r.files[candidate] {
				return lang.Target{Local: candidate}, true
			}
		}
	}
	return lang.Target{}, false
}

// Crates answers what the Cargo manifests of the project declare, for a plugin
// whose files name Rust crates: the shader plugin's WESL and naga_oil imports
// (bevy_pbr::...) name the crates that ship those shaders.
type Crates struct{ r *resolver }

// NewCrates reads the Cargo.toml and Cargo.lock files of the file list.
func NewCrates(all []*scan.File) Crates { return Crates{newResolver(all)} }

// Crate is the target a crate name has for file, as Rust code there would get it:
// a path dependency or a crate of the workspace as its directory, a crates.io
// dependency with the version Cargo.lock holds. The Cargo.toml governing file is
// asked first, then every other manifest of the project (shaders often live in an
// assets/ directory outside the crate that loads them). ok is false when no
// manifest declares the crate and no crate of the project has that name.
//
// Implements: REQ-SHADER-007
func (c Crates) Crate(file, name string) (lang.Target, bool) {
	crates := c.r.crates
	if own := c.r.crateOf(file); own != nil {
		crates = append([]*crate{own}, crates...)
	}
	name = normalize(name)
	for _, cr := range crates {
		d, ok := cr.dependencies[name]
		if !ok {
			continue
		}
		if d.path != "" {
			return lang.Target{Local: d.path}, true
		}
		if directory, ok := c.r.members[normalize(d.packageName)]; ok {
			return lang.Target{Local: directory}, true
		}
		return c.r.target(d), true
	}
	if directory, ok := c.r.members[name]; ok {
		return lang.Target{Local: directory}, true
	}
	return lang.Target{}, false
}

// Locked is the version of a package Cargo.lock holds (the newest when it holds
// several), or "".
func (c Crates) Locked(name string) string { return c.r.pick(name, "") }

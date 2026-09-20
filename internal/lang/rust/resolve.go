package rust

import (
	"path"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

var stdCrates = map[string]bool{"std": true, "core": true, "alloc": true, "proc_macro": true, "test": true}

// dep is one Cargo dependency as the code names it (dashes become underscores).
type dep struct {
	pkg     string // package name on crates.io (differs from the key when renamed)
	version string
	path    string // project-relative directory of a path dependency
}

type crate struct {
	dir  string // directory holding Cargo.toml
	deps map[string]dep
}

type resolver struct {
	files   map[string]bool
	crates  []*crate            // deepest first, so a file finds its own crate
	members map[string]string   // normalized package name -> crate dir (workspace / path crates)
	locked  map[string]string   // package -> version from Cargo.lock
	tree    map[string][]string // package -> the crates it depends on, from Cargo.lock
}

// Dependencies implements lang.Transitive: Cargo.lock resolves the whole crate graph,
// so the answer needs nothing but the file the project already carries.
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecoCrates {
		return nil
	}
	var out []lang.Target
	for _, dep := range r.tree[t.Package] {
		version := r.locked[dep]
		out = append(out, lang.Target{
			Ecosystem: ecoCrates, Package: dep, Version: version, Pinned: version != "",
		})
	}
	return out
}

func norm(name string) string { return strings.ReplaceAll(name, "-", "_") }

func newResolver(all []*scan.File) *resolver {
	r := &resolver{files: map[string]bool{}, members: map[string]string{}, locked: map[string]string{}, tree: map[string][]string{}}
	workspaceDeps := map[string]dep{}
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
			if _, err := toml.DecodeFile(f.Abs, &doc); err == nil {
				manifests = append(manifests, manifest{f, doc})
				if ws, ok := doc["workspace"].(map[string]any); ok {
					for k, v := range table(ws["dependencies"]) {
						workspaceDeps[norm(k)] = parseDep(k, v, path.Dir(f.Path))
					}
				}
			}
		case "Cargo.lock":
			var lock struct {
				Package []struct {
					Name, Version string
					// Each entry lists what that crate needs, as "name" or
					// "name version": the transitive graph, already resolved.
					Dependencies []string
				}
			}
			if _, err := toml.DecodeFile(f.Abs, &lock); err == nil {
				for _, p := range lock.Package {
					r.locked[p.Name] = p.Version
					for _, d := range p.Dependencies {
						if name, _, _ := strings.Cut(d, " "); name != "" && name != p.Name {
							r.tree[p.Name] = append(r.tree[p.Name], name)
						}
					}
				}
			}
		}
	}
	for _, m := range manifests {
		dir := path.Dir(m.f.Path)
		c := &crate{dir: dir, deps: map[string]dep{}}
		if pkg, ok := m.doc["package"].(map[string]any); ok {
			if name, ok := pkg["name"].(string); ok {
				r.members[norm(name)] = dir
			}
		}
		add := func(t map[string]any) {
			for k, v := range t {
				d := parseDep(k, v, dir)
				if w, ok := v.(map[string]any); ok && w["workspace"] == true {
					if wd, ok := workspaceDeps[norm(k)]; ok {
						d = wd
					}
				}
				c.deps[norm(k)] = d
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
	sort.Slice(r.crates, func(i, j int) bool { return len(r.crates[i].dir) > len(r.crates[j].dir) })
	return r
}

func table(v any) map[string]any {
	t, _ := v.(map[string]any)
	return t
}

func parseDep(key string, v any, dir string) dep {
	d := dep{pkg: key}
	switch v := v.(type) {
	case string:
		d.version = v
	case map[string]any:
		if s, ok := v["version"].(string); ok {
			d.version = s
		}
		if s, ok := v["package"].(string); ok {
			d.pkg = s
		}
		if s, ok := v["path"].(string); ok {
			d.path = path.Clean(path.Join(dir, s))
		}
	}
	return d
}

func (r *resolver) crateOf(file string) *crate {
	for _, c := range r.crates {
		if c.dir == "." || strings.HasPrefix(file, c.dir+"/") {
			return c
		}
	}
	return nil
}

// moduleDir is where a file's child modules live: src/lib.rs and a/mod.rs own their
// directory, a/b.rs owns a/b/.
func moduleDir(file string) string {
	switch base := path.Base(file); base {
	case "lib.rs", "main.rs", "mod.rs":
		return path.Dir(file)
	default:
		return path.Join(path.Dir(file), strings.TrimSuffix(base, ".rs"))
	}
}

func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	segments := strings.Split(imp.Module, "::")
	c := r.crateOf(file)
	switch first := segments[0]; {
	case first == "crate":
		if c != nil {
			t, _ := r.probe(path.Join(c.dir, "src"), segments[1:])
			return t
		}
		return lang.Target{}
	case first == "self":
		t, _ := r.probe(moduleDir(file), segments[1:])
		return t
	case first == "super":
		dir := moduleDir(file)
		for len(segments) > 0 && segments[0] == "super" {
			dir, segments = path.Dir(dir), segments[1:]
		}
		t, _ := r.probe(dir, segments)
		return t
	case stdCrates[first]:
		return lang.Target{Ecosystem: ecoStd, Package: first}
	}
	// Edition 2018 paths may name a module of the current file directly.
	if t, ok := r.probe(moduleDir(file), segments[:1]); ok {
		if t2, ok := r.probe(moduleDir(file), segments); ok {
			return t2
		}
		return t
	}
	// Crates are lower case by convention; `use Enum::*` names a local item.
	if first := segments[0]; first != "" && first[0] >= 'A' && first[0] <= 'Z' {
		return lang.Target{}
	}
	name := norm(segments[0])
	if c != nil {
		if d, ok := c.deps[name]; ok {
			if d.path != "" {
				return r.local(d.path, segments[1:])
			}
			if dir, ok := r.members[norm(d.pkg)]; ok {
				return r.local(dir, segments[1:])
			}
			// Cargo reads a bare "1.2.3" as ^1.2.3, so a manifest never pins on its
			// own: only Cargo.lock says which version is built.
			t := lang.Target{Ecosystem: ecoCrates, Package: d.pkg, Version: d.version}
			if exact := r.locked[d.pkg]; exact != "" {
				t.Version, t.Requested, t.Pinned = exact, d.version, true
			}
			return t
		}
	}
	if dir, ok := r.members[name]; ok {
		return r.local(dir, segments[1:])
	}
	return lang.Target{Ecosystem: ecoCrates, Package: segments[0], Unresolved: true}
}

// local resolves a path inside another project crate, falling back to the crate itself.
func (r *resolver) local(crateDir string, segments []string) lang.Target {
	if t, ok := r.probe(path.Join(crateDir, "src"), segments); ok && len(segments) > 0 {
		return t
	}
	return lang.Target{Local: crateDir}
}

// probe finds the module file for the longest prefix of segments under dir; with no
// segments it returns the crate or module root file.
func (r *resolver) probe(dir string, segments []string) (lang.Target, bool) {
	if len(segments) == 0 {
		for _, root := range []string{"lib.rs", "main.rs", "mod.rs"} {
			if p := path.Join(dir, root); r.files[p] {
				return lang.Target{Local: p}, true
			}
		}
		if r.files[dir+".rs"] { // 2018 layout: a.rs owns a/
			return lang.Target{Local: dir + ".rs"}, true
		}
		return lang.Target{}, false
	}
	for n := len(segments); n > 0; n-- {
		p := path.Join(append([]string{dir}, segments[:n]...)...)
		for _, candidate := range []string{p + ".rs", path.Join(p, "mod.rs")} {
			if r.files[candidate] {
				return lang.Target{Local: candidate}, true
			}
		}
	}
	return lang.Target{}, false
}

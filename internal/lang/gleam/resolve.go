package gleam

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/beam"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// pkg is a Gleam package of the repository: a directory with a gleam.toml, or
// the directory above a src/, test/ or dev/ holding modules without one.
type pkg struct {
	dir      string
	name     string
	deps     map[string]*dependency
	manifest *manifest
	modules  map[string]string // module path -> file (src/ before test/ and dev/)
	npm      map[string]string // package.json dependencies, for JavaScript externals
	otpApps  map[string]string // an Erlang application a manifest package is -> the package
}

type resolver struct {
	files     map[string]bool
	dirs      map[string]bool
	pkgs      map[string]*pkg   // by directory
	erlFiles  map[string]string // Erlang module -> the .erl file defining it
	installed map[string]string // package dir + "\x00" + module -> Hex package, from build/packages
	lang.NoteList
}

// The notes a resolver keeps reach --explain only through lang.Noter.
var _ lang.Noter = (*resolver)(nil)

// Implements: REQ-GLEAM-004, REQ-GLEAM-005, REQ-GLEAM-006
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{files: map[string]bool{}, dirs: map[string]bool{}, pkgs: map[string]*pkg{},
		erlFiles: map[string]string{}, installed: map[string]string{}}
	abs := map[string]string{}
	for _, f := range all {
		r.files[f.Path] = true
		abs[f.Path] = f.Abs
		for d := path.Dir(f.Path); d != "." && !r.dirs[d]; d = path.Dir(d) {
			r.dirs[d] = true
		}
	}
	read := func(rel string) ([]byte, bool) {
		if a, ok := abs[rel]; ok {
			data, err := os.ReadFile(a)
			return data, err == nil
		}
		if root == "" {
			return nil, false
		}
		// manifest.toml is often ignored by git in libraries; read what is on disk.
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		return data, err == nil
	}
	newPkg := func(dir string) *pkg {
		p := &pkg{dir: dir, deps: map[string]*dependency{}, modules: map[string]string{}, npm: map[string]string{},
			otpApps: map[string]string{}}
		r.pkgs[dir] = p
		return p
	}
	var sources []string
	for _, f := range all {
		switch {
		case inBuild(f.Path):
		case path.Base(f.Path) == "gleam.toml":
			dir := path.Dir(f.Path)
			p := newPkg(dir)
			if src, ok := read(f.Path); ok {
				c := readConfig(src)
				p.name, p.deps = c.name, c.deps
			}
			if src, ok := read(path.Join(dir, "manifest.toml")); ok {
				if p.manifest = readManifest(src); p.manifest != nil {
					// Implements: REQ-TRC-017
					if _, listed := abs[path.Join(dir, "manifest.toml")]; !listed {
						r.NoteIgnored(path.Join(dir, "manifest.toml"))
					}
					for _, l := range p.manifest.packages {
						if l.otpApp != "" {
							p.otpApps[l.otpApp] = l.name
						}
					}
				}
			}
			if src, ok := read(path.Join(dir, "package.json")); ok {
				readNPM(src, p.npm)
			}
			if root != "" {
				r.readInstalled(root, dir)
			}
		case path.Ext(f.Path) == ".gleam":
			sources = append(sources, f.Path)
		case path.Ext(f.Path) == ".erl":
			m := strings.TrimSuffix(path.Base(f.Path), ".erl")
			if prev, ok := r.erlFiles[m]; !ok || f.Path < prev {
				r.erlFiles[m] = f.Path
			}
		}
	}
	sort.Strings(sources)
	for _, f := range sources {
		dir, rootDir, module := r.locate(f)
		if module == "" {
			continue
		}
		p := r.pkgs[dir]
		if p == nil {
			p = newPkg(dir)
		}
		if prev, dup := p.modules[module]; !dup || rootDir == "src" && !strings.HasPrefix(prev, path.Join(dir, "src")+"/") {
			p.modules[module] = f
		}
	}
	return r
}

// locate finds a module file's package directory, the root it is under (src,
// test or dev) and its module path: the nearest gleam.toml above it, else the
// directory above its nearest src/, test/ or dev/.
func (r *resolver) locate(file string) (dir, rootDir, module string) {
	for d := path.Dir(file); ; d = path.Dir(d) {
		if _, ok := r.pkgs[d]; ok {
			rel := strings.TrimPrefix(file, d+"/")
			if d == "." {
				rel = file
			}
			first, rest, ok := strings.Cut(rel, "/")
			if ok && (first == "src" || first == "test" || first == "dev") {
				return d, first, strings.TrimSuffix(rest, ".gleam")
			}
			return d, "", ""
		}
		if d == "." {
			break
		}
	}
	segments := strings.Split(file, "/")
	for i := len(segments) - 2; i >= 0; i-- {
		if s := segments[i]; s == "src" || s == "test" || s == "dev" {
			dir := "."
			if i > 0 {
				dir = strings.Join(segments[:i], "/")
			}
			return dir, s, strings.TrimSuffix(strings.Join(segments[i+1:], "/"), ".gleam")
		}
	}
	return "", "", ""
}

// packageOf is the package a file belongs to: the nearest one above it.
func (r *resolver) packageOf(file string) *pkg {
	for d := path.Dir(file); ; d = path.Dir(d) {
		if p := r.pkgs[d]; p != nil {
			return p
		}
		if d == "." || d == "/" {
			return &pkg{dir: ".", deps: map[string]*dependency{}, modules: map[string]string{}, npm: map[string]string{}, otpApps: map[string]string{}}
		}
	}
}

// readInstalled learns which Hex package provides which module from what gleam
// downloaded beside the package: build/packages/<name>/src/**.gleam. The build
// directory is not in the repository, so this is read from disk, and only when
// present.
//
// Implements: REQ-GLEAM-004
func (r *resolver) readInstalled(root, dir string) {
	base := filepath.Join(root, filepath.FromSlash(dir), "build", "packages")
	entries, err := os.ReadDir(base)
	if err != nil {
		return
	}
	budget := 50_000
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		src := filepath.Join(base, name, "src")
		filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
			if err != nil || budget <= 0 {
				return filepath.SkipDir
			}
			if budget--; d.IsDir() || filepath.Ext(p) != ".gleam" {
				return nil
			}
			rel, _ := filepath.Rel(src, p)
			key := dir + "\x00" + strings.TrimSuffix(filepath.ToSlash(rel), ".gleam")
			if _, ok := r.installed[key]; !ok {
				r.installed[key] = name
			}
			return nil
		})
	}
}

func readNPM(src []byte, into map[string]string) {
	var pj struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if json.Unmarshal(src, &pj) != nil {
		return
	}
	for _, m := range []map[string]string{pj.Dependencies, pj.DevDependencies} {
		for k, v := range m {
			if _, ok := into[k]; !ok {
				into[k] = v
			}
		}
	}
}

// Implements: REQ-GLEAM-004, REQ-GLEAM-005, REQ-GLEAM-006, REQ-GLEAM-007
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	switch imp.Name {
	case kindModule:
		return r.module(file, imp.Module)
	case kindErlang:
		return r.erlang(file, imp.Module)
	case kindJS:
		return r.javascript(file, imp.Module)
	case kindDep:
		p := r.pkgs[path.Dir(file)]
		if p == nil || p.deps[imp.Module] == nil {
			return lang.Target{}
		}
		return r.hexTarget(p, imp.Module)
	case kindLocked:
		// Only the manifest of a gleam.toml's package is Gleam's.
		p := r.pkgs[path.Dir(file)]
		if p == nil || p.manifest == nil || p.manifest.packages[imp.Module] == nil {
			return lang.Target{}
		}
		return r.hexTarget(p, imp.Module)
	}
	return lang.Target{}
}

// module resolves an import: to a module of the importing package (src/, test/,
// dev/); of a path dependency; to the package build/packages says provides it;
// to the declared or locked package its leading segments name joined by `_`
// (gleam/erlang/process -> gleam_erlang, lustre/element -> lustre); a gleam/
// module of the standard library to gleam_stdlib; a missing module of the
// package's own namespace is dropped; else an unresolved package named by the
// same rules.
//
// Implements: REQ-GLEAM-004, REQ-GLEAM-011
func (r *resolver) module(file, mod string) lang.Target {
	if mod == "gleam" {
		return lang.Target{} // the prelude, built into the compiler
	}
	p := r.packageOf(file)
	if f, ok := p.modules[mod]; ok {
		return lang.Target{Local: f}
	}
	for _, name := range sortedKeys(p.deps) {
		if q := r.pathPackage(p, name); q != nil {
			if f, ok := q.modules[mod]; ok {
				return lang.Target{Local: f}
			}
		}
	}
	if name, ok := r.installed[p.dir+"\x00"+mod]; ok {
		return r.hexTarget(p, name)
	}
	name, known := beam.GleamPackage(mod, p.knows)
	if known {
		return r.hexTarget(p, name)
	}
	segments := strings.Split(mod, "/")
	if name == p.name || segments[0] == p.name {
		return lang.Target{} // its own namespace: a module it does not have
	}
	return lang.Target{Ecosystem: ecoHex, Package: name, Unresolved: true}
}

// knows reports whether the package declares or locks a package.
func (p *pkg) knows(name string) bool {
	if p.deps[name] != nil {
		return true
	}
	if p.manifest != nil {
		return p.manifest.packages[name] != nil || p.manifest.requirements[name] != nil
	}
	return false
}

// pathPackage is the repository package a path dependency (or a manifest's local
// package) of p names.
func (r *resolver) pathPackage(p *pkg, name string) *pkg {
	rel := ""
	if d := p.deps[name]; d != nil && d.path != "" {
		rel = d.path
	} else if p.manifest != nil {
		if l := p.manifest.packages[name]; l != nil && l.source == "local" {
			rel = l.path
		}
	}
	if rel == "" {
		return nil
	}
	dir := path.Join(p.dir, rel)
	if !inside(dir) {
		return nil
	}
	return r.pkgs[dir]
}

// erlang resolves the module of @external(erlang, "mod", "f"): a compiled Gleam
// module (gleam@list) as that module; a project .erl file; a package of that name
// or Erlang application; Erlang/OTP; a package its prefix names
// (gleam_otp_external -> gleam_otp); else an unresolved package named after it.
//
// Implements: REQ-GLEAM-007, REQ-GLEAM-011
func (r *resolver) erlang(file, mod string) lang.Target {
	if strings.Contains(mod, "@") {
		return r.module(file, strings.ReplaceAll(mod, "@", "/"))
	}
	if f, ok := r.erlFiles[mod]; ok {
		return lang.Target{Local: f}
	}
	p := r.packageOf(file)
	if strings.HasPrefix(mod, "Elixir.") {
		return lang.Target{} // an Elixir module: no package is named after it
	}
	if p.knows(mod) {
		return r.hexTarget(p, mod)
	}
	if name, ok := p.otpApps[mod]; ok {
		return r.hexTarget(p, name)
	}
	if beam.OTPModule(mod) {
		return lang.Target{Ecosystem: ecoOTP, Package: mod}
	}
	for i := len(mod) - 1; i > 0; i-- {
		if mod[i] == '_' && p.knows(mod[:i]) {
			return r.hexTarget(p, mod[:i])
		}
	}
	return lang.Target{Ecosystem: ecoHex, Package: mod, Unresolved: true}
}

// javascript resolves the module of @external(javascript, "spec", "f"). A relative
// path is taken from the Gleam module's place in the compiled output, which
// mirrors src/: a file of the package, or - when it climbs out of the package, as
// "../../gleam_stdlib/gleam/list.mjs" does - a file of the package it names. A
// bare specifier is an npm package, declared by package.json or unresolved.
//
// Implements: REQ-GLEAM-007
func (r *resolver) javascript(file, spec string) lang.Target {
	if strings.HasPrefix(spec, "/") || strings.HasPrefix(spec, "node:") || spec == "" {
		return lang.Target{}
	}
	if !strings.HasPrefix(spec, "./") && !strings.HasPrefix(spec, "../") {
		name := npmName(spec)
		p := r.packageOf(file)
		if v, ok := p.npm[name]; ok {
			return lang.Target{Ecosystem: ecoNPM, Package: name, Version: v, Pinned: lang.PinnedSemver(v)}
		}
		return lang.Target{Ecosystem: ecoNPM, Package: name, Unresolved: true}
	}
	if f := path.Join(path.Dir(file), spec); r.files[f] {
		return lang.Target{Local: f}
	}
	dir, _, module := r.locate(file)
	p := r.pkgs[dir]
	if module == "" || p == nil {
		return lang.Target{}
	}
	const self = "\x00self"
	v := path.Join(self, path.Dir(module), spec)
	if rest, ok := strings.CutPrefix(v, self+"/"); ok {
		for _, root := range []string{"src", "test", "dev"} {
			if f := path.Join(p.dir, root, rest); r.files[f] {
				return lang.Target{Local: f}
			}
		}
		return lang.Target{}
	}
	name, rest, ok := strings.Cut(v, "/")
	if !ok || name == ".." || name == self {
		return lang.Target{}
	}
	if q := r.pathPackage(p, name); q != nil {
		if f := path.Join(q.dir, "src", rest); r.files[f] {
			return lang.Target{Local: f}
		}
	}
	if name == p.name {
		return lang.Target{}
	}
	return r.hexTarget(p, name)
}

// npmName is the package a JavaScript module specifier names: "@scope/pkg/x" is
// @scope/pkg, "react-dom/client" is react-dom.
func npmName(spec string) string {
	parts := strings.Split(spec, "/")
	if strings.HasPrefix(spec, "@") && len(parts) > 1 {
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

// hexTarget is a package as the package's manifest.toml locks it or its
// gleam.toml declares it. The manifest pins a Hex package at its version (the
// requirement kept as requested when it is a range), a git package at its commit
// and makes a local one an edge to that package's gleam.toml. Without it the Hex
// rule of the BEAM plugin applies: "== 1.2.3" and a bare version pin, ranges and
// "~>" float as written; a git ref that is a commit pins, a branch or tag
// floats; a path dependency is an edge.
//
// Implements: REQ-GLEAM-005, REQ-GLEAM-006, REQ-GLEAM-008
func (r *resolver) hexTarget(p *pkg, name string) lang.Target {
	d := p.deps[name]
	if d == nil && p.manifest != nil {
		d = p.manifest.requirements[name]
	}
	if p.manifest != nil {
		if l := p.manifest.packages[name]; l != nil {
			return r.lockTarget(p, l, d)
		}
	}
	if d == nil {
		return lang.Target{Ecosystem: ecoHex, Package: name, Unresolved: true}
	}
	switch {
	case d.path != "":
		return r.localTarget(p, d.path)
	case d.git != "":
		pinned := lang.Commit(d.ref)
		return lang.Target{Ecosystem: ecoHex, Package: name, Version: d.ref, Origin: d.git, Pinned: pinned, Floating: !pinned}
	}
	v, pinned := beam.HexPinned(d.req)
	return lang.Target{Ecosystem: ecoHex, Package: name, Version: v, Pinned: pinned, Floating: d.req == ""}
}

func (r *resolver) lockTarget(p *pkg, l *locked, d *dependency) lang.Target {
	switch l.source {
	case "local":
		return r.localTarget(p, l.path)
	case "git":
		t := lang.Target{Ecosystem: ecoHex, Package: l.name, Version: l.commit, Origin: l.repo, Pinned: lang.Commit(l.commit)}
		if d != nil && d.ref != "" && d.ref != l.commit {
			t.Requested = d.ref
		}
		return t
	}
	t := lang.Target{Ecosystem: ecoHex, Package: l.name, Version: l.version, Pinned: l.version != ""}
	if d != nil && d.req != "" && d.req != l.version {
		if v, _ := beam.HexPinned(d.req); v != l.version {
			t.Requested = d.req
		}
	}
	return t
}

// localTarget is a path dependency: its gleam.toml, else its directory; a path
// outside the repository is dropped.
func (r *resolver) localTarget(p *pkg, rel string) lang.Target {
	dir := path.Join(p.dir, rel)
	if !inside(dir) {
		return lang.Target{}
	}
	if f := path.Join(dir, "gleam.toml"); r.files[f] {
		return lang.Target{Local: f}
	}
	if r.dirs[dir] {
		return lang.Target{Local: dir}
	}
	return lang.Target{}
}

// inside reports whether a cleaned relative path stays in the repository.
func inside(p string) bool { return p != ".." && !strings.HasPrefix(p, "../") && !path.IsAbs(p) }

// Dependencies implements lang.Transitive from manifest.toml, whose every package
// lists the packages it requires; each is pinned by the same manifest.
//
// Implements: REQ-GLEAM-006
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecoHex {
		return nil
	}
	var owner *pkg
	var entry *locked
	for _, dir := range sortedKeys(r.pkgs) {
		p := r.pkgs[dir]
		if p.manifest == nil {
			continue
		}
		if l := p.manifest.packages[t.Package]; l != nil && l.source != "local" && (entry == nil || l.version == t.Version && entry.version != t.Version) {
			owner, entry = p, l
		}
	}
	if entry == nil {
		return nil
	}
	var out []lang.Target
	for _, name := range entry.requirements {
		l := owner.manifest.packages[name]
		if l == nil || l.source == "local" {
			continue
		}
		out = append(out, r.lockTarget(owner, l, nil))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Package < out[j].Package })
	return out
}

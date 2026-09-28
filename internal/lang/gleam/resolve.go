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

// gleamPackage is a Gleam package of the repository: a directory with a gleam.toml, or
// the directory above a src/, test/ or dev/ holding modules without one.
type gleamPackage struct {
	directory    string
	name         string
	dependencies map[string]*dependency
	manifest     *manifest
	modules      map[string]string // module path -> file (src/ before test/ and dev/)
	npm          map[string]string // package.json dependencies, for JavaScript externals
	otpApps      map[string]string // an Erlang application a manifest package is -> the package
}

type resolver struct {
	files       map[string]bool
	directories map[string]bool
	packages    map[string]*gleamPackage // by directory
	erlFiles    map[string]string        // Erlang module -> the .erl file defining it
	installed   map[string]string        // package dir + "\x00" + module -> Hex package, from build/packages
	lang.NoteList
}

// The notes a resolver keeps reach --explain only through lang.Noter.
var _ lang.Noter = (*resolver)(nil)

// Implements: REQ-GLEAM-004, REQ-GLEAM-005, REQ-GLEAM-006
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{files: map[string]bool{}, directories: map[string]bool{}, packages: map[string]*gleamPackage{},
		erlFiles: map[string]string{}, installed: map[string]string{}}
	absolute := map[string]string{}
	for _, f := range all {
		r.files[f.Path] = true
		absolute[f.Path] = f.AbsolutePath
		for d := path.Dir(f.Path); d != "." && !r.directories[d]; d = path.Dir(d) {
			r.directories[d] = true
		}
	}
	read := func(relative string) ([]byte, bool) {
		if a, ok := absolute[relative]; ok {
			data, err := os.ReadFile(a)
			return data, err == nil
		}
		if root == "" {
			return nil, false
		}
		// manifest.toml is often ignored by git in libraries; read what is on disk.
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		return data, err == nil
	}
	newPackage := func(directory string) *gleamPackage {
		p := &gleamPackage{directory: directory, dependencies: map[string]*dependency{}, modules: map[string]string{}, npm: map[string]string{},
			otpApps: map[string]string{}}
		r.packages[directory] = p
		return p
	}
	var sources []string
	for _, f := range all {
		switch {
		case inBuild(f.Path):
		case path.Base(f.Path) == "gleam.toml":
			directory := path.Dir(f.Path)
			p := newPackage(directory)
			if source, ok := read(f.Path); ok {
				c := readConfig(source)
				p.name, p.dependencies = c.name, c.dependencies
			}
			if source, ok := read(path.Join(directory, "manifest.toml")); ok {
				if p.manifest = readManifest(source); p.manifest != nil {
					// Implements: REQ-TRC-017
					if _, listed := absolute[path.Join(directory, "manifest.toml")]; !listed {
						r.NoteIgnored(path.Join(directory, "manifest.toml"))
					}
					for _, l := range p.manifest.packages {
						if l.otpApp != "" {
							p.otpApps[l.otpApp] = l.name
						}
					}
				}
			}
			if source, ok := read(path.Join(directory, "package.json")); ok {
				readNPM(source, p.npm)
			}
			if root != "" {
				r.readInstalled(root, directory)
			}
		case path.Ext(f.Path) == ".gleam":
			sources = append(sources, f.Path)
		case path.Ext(f.Path) == ".erl":
			m := strings.TrimSuffix(path.Base(f.Path), ".erl")
			if previous, ok := r.erlFiles[m]; !ok || f.Path < previous {
				r.erlFiles[m] = f.Path
			}
		}
	}
	sort.Strings(sources)
	for _, f := range sources {
		directory, rootDirectory, module := r.locate(f)
		if module == "" {
			continue
		}
		p := r.packages[directory]
		if p == nil {
			p = newPackage(directory)
		}
		if previous, duplicate := p.modules[module]; !duplicate || rootDirectory == "src" && !strings.HasPrefix(previous, path.Join(directory, "src")+"/") {
			p.modules[module] = f
		}
	}
	return r
}

// locate finds a module file's package directory, the root it is under (src,
// test or dev) and its module path: the nearest gleam.toml above it, else the
// directory above its nearest src/, test/ or dev/.
func (r *resolver) locate(file string) (directory, rootDirectory, module string) {
	for d := path.Dir(file); ; d = path.Dir(d) {
		if _, ok := r.packages[d]; ok {
			relative := strings.TrimPrefix(file, d+"/")
			if d == "." {
				relative = file
			}
			first, rest, ok := strings.Cut(relative, "/")
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
			directory := "."
			if i > 0 {
				directory = strings.Join(segments[:i], "/")
			}
			return directory, s, strings.TrimSuffix(strings.Join(segments[i+1:], "/"), ".gleam")
		}
	}
	return "", "", ""
}

// packageOf is the package a file belongs to: the nearest one above it.
func (r *resolver) packageOf(file string) *gleamPackage {
	for d := path.Dir(file); ; d = path.Dir(d) {
		if p := r.packages[d]; p != nil {
			return p
		}
		if d == "." || d == "/" {
			return &gleamPackage{directory: ".", dependencies: map[string]*dependency{}, modules: map[string]string{}, npm: map[string]string{}, otpApps: map[string]string{}}
		}
	}
}

// readInstalled learns which Hex package provides which module from what gleam
// downloaded beside the package: build/packages/<name>/src/**.gleam. The build
// directory is not in the repository, so this is read from disk, and only when
// present.
//
// Implements: REQ-GLEAM-004
func (r *resolver) readInstalled(root, directory string) {
	base := filepath.Join(root, filepath.FromSlash(directory), "build", "packages")
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
		source := filepath.Join(base, name, "src")
		filepath.WalkDir(source, func(p string, d os.DirEntry, err error) error {
			if err != nil || budget <= 0 {
				return filepath.SkipDir
			}
			if budget--; d.IsDir() || filepath.Ext(p) != ".gleam" {
				return nil
			}
			relative, _ := filepath.Rel(source, p)
			key := directory + "\x00" + strings.TrimSuffix(filepath.ToSlash(relative), ".gleam")
			if _, ok := r.installed[key]; !ok {
				r.installed[key] = name
			}
			return nil
		})
	}
}

func readNPM(source []byte, into map[string]string) {
	var pj struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if json.Unmarshal(source, &pj) != nil {
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
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	switch rawImport.Name {
	case kindModule:
		return r.module(file, rawImport.Module)
	case kindErlang:
		return r.erlang(file, rawImport.Module)
	case kindJS:
		return r.javascript(file, rawImport.Module)
	case kindDependency:
		p := r.packages[path.Dir(file)]
		if p == nil || p.dependencies[rawImport.Module] == nil {
			return lang.Target{}
		}
		return r.hexTarget(p, rawImport.Module)
	case kindLocked:
		// Only the manifest of a gleam.toml's package is Gleam's.
		p := r.packages[path.Dir(file)]
		if p == nil || p.manifest == nil || p.manifest.packages[rawImport.Module] == nil {
			return lang.Target{}
		}
		return r.hexTarget(p, rawImport.Module)
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
func (r *resolver) module(file, module string) lang.Target {
	if module == "gleam" {
		return lang.Target{} // the prelude, built into the compiler
	}
	p := r.packageOf(file)
	if f, ok := p.modules[module]; ok {
		return lang.Target{Local: f}
	}
	for _, name := range sortedKeys(p.dependencies) {
		if q := r.pathPackage(p, name); q != nil {
			if f, ok := q.modules[module]; ok {
				return lang.Target{Local: f}
			}
		}
	}
	if name, ok := r.installed[p.directory+"\x00"+module]; ok {
		return r.hexTarget(p, name)
	}
	name, known := beam.GleamPackage(module, p.knows)
	if known {
		return r.hexTarget(p, name)
	}
	segments := strings.Split(module, "/")
	if name == p.name || segments[0] == p.name {
		return lang.Target{} // its own namespace: a module it does not have
	}
	return lang.Target{Ecosystem: ecosystemHex, Package: name, Unresolved: true}
}

// knows reports whether the package declares or locks a package.
func (p *gleamPackage) knows(name string) bool {
	if p.dependencies[name] != nil {
		return true
	}
	if p.manifest != nil {
		return p.manifest.packages[name] != nil || p.manifest.requirements[name] != nil
	}
	return false
}

// pathPackage is the repository package a path dependency (or a manifest's local
// package) of p names.
func (r *resolver) pathPackage(p *gleamPackage, name string) *gleamPackage {
	relative := ""
	if d := p.dependencies[name]; d != nil && d.path != "" {
		relative = d.path
	} else if p.manifest != nil {
		if l := p.manifest.packages[name]; l != nil && l.source == "local" {
			relative = l.path
		}
	}
	if relative == "" {
		return nil
	}
	directory := path.Join(p.directory, relative)
	if !inside(directory) {
		return nil
	}
	return r.packages[directory]
}

// erlang resolves the module of @external(erlang, "mod", "f"): a compiled Gleam
// module (gleam@list) as that module; a project .erl file; a package of that name
// or Erlang application; Erlang/OTP; a package its prefix names
// (gleam_otp_external -> gleam_otp); else an unresolved package named after it.
//
// Implements: REQ-GLEAM-007, REQ-GLEAM-011
func (r *resolver) erlang(file, module string) lang.Target {
	if strings.Contains(module, "@") {
		return r.module(file, strings.ReplaceAll(module, "@", "/"))
	}
	if f, ok := r.erlFiles[module]; ok {
		return lang.Target{Local: f}
	}
	p := r.packageOf(file)
	if strings.HasPrefix(module, "Elixir.") {
		return lang.Target{} // an Elixir module: no package is named after it
	}
	if p.knows(module) {
		return r.hexTarget(p, module)
	}
	if name, ok := p.otpApps[module]; ok {
		return r.hexTarget(p, name)
	}
	if beam.OTPModule(module) {
		return lang.Target{Ecosystem: ecosystemOTP, Package: module}
	}
	for i := len(module) - 1; i > 0; i-- {
		if module[i] == '_' && p.knows(module[:i]) {
			return r.hexTarget(p, module[:i])
		}
	}
	return lang.Target{Ecosystem: ecosystemHex, Package: module, Unresolved: true}
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
			return lang.Target{Ecosystem: ecosystemNPM, Package: name, Version: v, Pinned: lang.PinnedSemver(v)}
		}
		return lang.Target{Ecosystem: ecosystemNPM, Package: name, Unresolved: true}
	}
	if f := path.Join(path.Dir(file), spec); r.files[f] {
		return lang.Target{Local: f}
	}
	directory, _, module := r.locate(file)
	p := r.packages[directory]
	if module == "" || p == nil {
		return lang.Target{}
	}
	const self = "\x00self"
	v := path.Join(self, path.Dir(module), spec)
	if rest, ok := strings.CutPrefix(v, self+"/"); ok {
		for _, root := range []string{"src", "test", "dev"} {
			if f := path.Join(p.directory, root, rest); r.files[f] {
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
		if f := path.Join(q.directory, "src", rest); r.files[f] {
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
func (r *resolver) hexTarget(p *gleamPackage, name string) lang.Target {
	d := p.dependencies[name]
	if d == nil && p.manifest != nil {
		d = p.manifest.requirements[name]
	}
	if p.manifest != nil {
		if l := p.manifest.packages[name]; l != nil {
			return r.lockTarget(p, l, d)
		}
	}
	if d == nil {
		return lang.Target{Ecosystem: ecosystemHex, Package: name, Unresolved: true}
	}
	switch {
	case d.path != "":
		return r.localTarget(p, d.path)
	case d.git != "":
		pinned := lang.Commit(d.reference)
		return lang.Target{Ecosystem: ecosystemHex, Package: name, Version: d.reference, Origin: d.git, Pinned: pinned, Floating: !pinned}
	}
	v, pinned := beam.HexPinned(d.requirement)
	return lang.Target{Ecosystem: ecosystemHex, Package: name, Version: v, Pinned: pinned, Floating: d.requirement == ""}
}

func (r *resolver) lockTarget(p *gleamPackage, l *locked, d *dependency) lang.Target {
	switch l.source {
	case "local":
		return r.localTarget(p, l.path)
	case "git":
		t := lang.Target{Ecosystem: ecosystemHex, Package: l.name, Version: l.commit, Origin: l.repository, Pinned: lang.Commit(l.commit)}
		if d != nil && d.reference != "" && d.reference != l.commit {
			t.Requested = d.reference
		}
		return t
	}
	t := lang.Target{Ecosystem: ecosystemHex, Package: l.name, Version: l.version, Pinned: l.version != ""}
	if d != nil && d.requirement != "" && d.requirement != l.version {
		if v, _ := beam.HexPinned(d.requirement); v != l.version {
			t.Requested = d.requirement
		}
	}
	return t
}

// localTarget is a path dependency: its gleam.toml, else its directory; a path
// outside the repository is dropped.
func (r *resolver) localTarget(p *gleamPackage, relative string) lang.Target {
	directory := path.Join(p.directory, relative)
	if !inside(directory) {
		return lang.Target{}
	}
	if f := path.Join(directory, "gleam.toml"); r.files[f] {
		return lang.Target{Local: f}
	}
	if r.directories[directory] {
		return lang.Target{Local: directory}
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
	if t.Ecosystem != ecosystemHex {
		return nil
	}
	var owner *gleamPackage
	var entry *locked
	for _, directory := range sortedKeys(r.packages) {
		p := r.packages[directory]
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

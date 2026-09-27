package nix

import (
	"os"
	"path"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// flakeDir is a directory with flake.nix and/or flake.lock.
type flakeDir struct {
	inputs map[string]string // input name -> RawImport.Module of its declaration
	lock   *lockFile
}

type resolver struct {
	files   map[string]bool
	dirs    map[string]bool
	flakes  map[string]*flakeDir
	pins    map[string]pinsFile      // directory of a sources.json -> its pins
	nixpkgs map[string]lang.Target   // project directory -> the nixpkgs it builds with
	deps    map[string][]lang.Target // lock target key -> its inputs' targets
	// inNixpkgs is set when the repository is nixpkgs itself, whose package lists
	// name its own packages.
	inNixpkgs bool
}

func readable(f *scan.File) bool { return !f.Binary && !f.TooLarge && f.Size <= lang.MaxParseSize }

func newResolver(all []*scan.File) *resolver {
	r := &resolver{files: map[string]bool{}, dirs: map[string]bool{}, flakes: map[string]*flakeDir{},
		pins: map[string]pinsFile{}, nixpkgs: map[string]lang.Target{}, deps: map[string][]lang.Target{}}
	flake := func(d string) *flakeDir {
		if r.flakes[d] == nil {
			r.flakes[d] = &flakeDir{inputs: map[string]string{}}
		}
		return r.flakes[d]
	}
	for _, f := range all {
		r.files[f.Path] = true
		for d := path.Dir(f.Path); d != "." && !r.dirs[d]; d = path.Dir(d) {
			r.dirs[d] = true
		}
		c := class(f.Path)
		if c == "" || !readable(f) {
			continue
		}
		src, err := os.ReadFile(f.Abs)
		if err != nil {
			continue
		}
		d := path.Dir(f.Path)
		switch c {
		case "flake":
			fd := flake(d)
			for _, im := range extract(src, true).Imports {
				if im.Name == kInput {
					name, _, _ := strings.Cut(im.Module, "\n")
					fd.inputs[name] = im.Module
				}
			}
		case "lock":
			flake(d).lock = readLock(src)
		case "niv", "npins":
			r.pins[d] = readPins(c, src)
		}
	}
	r.inNixpkgs = r.files["pkgs/top-level/all-packages.nix"]
	for _, d := range sortedKeys(r.flakes) {
		fd := r.flakes[d]
		if fd.lock != nil {
			r.lockDeps(d, fd.lock)
		}
		if t, ok := r.flakeNixpkgs(d, fd); ok {
			r.nixpkgs[d] = t
		}
	}
	for _, d := range sortedKeys(r.pins) {
		if p, ok := r.pins[d]["nixpkgs"]; ok {
			if _, has := r.nixpkgs[path.Dir(d)]; !has {
				r.nixpkgs[path.Dir(d)] = pinTarget(p)
			}
		}
	}
	return r
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func key(t lang.Target) string { return t.Ecosystem + "\x00" + t.Package + "\x00" + t.Version }

// lockDeps records, for each node of a lock, the nodes its inputs name (follows
// resolved), for --resolve-depth.
func (r *resolver) lockDeps(dir string, l *lockFile) {
	for _, k := range l.keys() {
		t, local := l.target(dir, k)
		if local != "" || t.Package == "" {
			continue
		}
		kk := key(t)
		if _, done := r.deps[kk]; done {
			continue
		}
		var out []lang.Target
		for _, name := range sortedKeys(l.Nodes[k].Inputs) {
			if nk, ok := l.input(k, name); ok {
				if d, local := l.target(dir, nk); local == "" && d.Package != "" {
					out = append(out, d)
				}
			}
		}
		r.deps[kk] = out
	}
}

// Dependencies lists what a locked input itself depends on, from flake.lock.
//
// Implements: REQ-NIX-005
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecoNix {
		return nil
	}
	return r.deps[key(t)]
}

// flakeNixpkgs is the nixpkgs a flake builds with: its input named nixpkgs, else
// the one input that is NixOS/nixpkgs.
func (r *resolver) flakeNixpkgs(dir string, fd *flakeDir) (lang.Target, bool) {
	names := []string{"nixpkgs"}
	for _, n := range sortedKeys(fd.inputs) {
		if n != "nixpkgs" {
			names = append(names, n)
		}
	}
	if fd.lock != nil {
		for _, n := range sortedKeys(fd.lock.Nodes[fd.lock.Root].Inputs) {
			if n != "nixpkgs" {
				names = append(names, n)
			}
		}
	}
	for _, n := range names {
		t := r.input(dir, n, 0)
		if t.Package == "nixpkgs" || strings.EqualFold(t.Package, "github.com/NixOS/nixpkgs") {
			return t, true
		}
	}
	return lang.Target{}, false
}

// input resolves a flake's input by name: through the lock beside flake.nix, else
// by the declaration's own reference (a follows of another top-level input taken
// through to it).
func (r *resolver) input(dir, name string, depth int) lang.Target {
	fd := r.flakes[dir]
	if fd == nil || depth > 8 {
		return lang.Target{}
	}
	if fd.lock != nil {
		if k, ok := fd.lock.input(fd.lock.Root, name); ok {
			t, local := fd.lock.target(dir, k)
			if local != "" {
				return r.localFlake(local)
			}
			return t
		}
	}
	decl, ok := fd.inputs[name]
	if !ok {
		return lang.Target{}
	}
	_, rest, _ := strings.Cut(decl, "\n")
	enc, follows, _ := strings.Cut(rest, "\n")
	if follows != "" {
		if strings.Contains(follows, "/") {
			return lang.Target{} // an input of an input: only the lock knows it
		}
		return r.input(dir, follows, depth+1)
	}
	ref := decodeRef(enc)
	if ref.typ == "path" {
		return r.localFlake(localDir(dir, ref.url))
	}
	t, _ := ref.target()
	return t
}

// localFlake is a path input: the flake.nix of that directory, else the directory.
func (r *resolver) localFlake(d string) lang.Target {
	switch {
	case d == "" || d == ".":
		return lang.Target{}
	case r.files[d+"/flake.nix"]:
		return lang.Target{Local: d + "/flake.nix"}
	case r.dirs[d]:
		return lang.Target{Local: d}
	}
	return lang.Target{}
}

// nearest is the closest directory at or above dir for which has is true, else
// "" (and false).
func nearest(dir string, has func(string) bool) (string, bool) {
	for d := dir; ; d = path.Dir(d) {
		if has(d) {
			return d, true
		}
		if d == "." || d == "/" {
			return "", false
		}
	}
}

// Implements: REQ-NIX-002, REQ-NIX-004, REQ-NIX-005, REQ-NIX-007, REQ-NIX-008, REQ-NIX-009
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	dir := path.Dir(file)
	switch imp.Name {
	case kImport, kPath:
		p := localDir(dir, imp.Module)
		switch {
		case p == "":
		case r.files[p]:
			return lang.Target{Local: p}
		case imp.Name == kImport && r.files[p+"/default.nix"]:
			return lang.Target{Local: p + "/default.nix"}
		case r.dirs[p] && p != dir:
			return lang.Target{Local: p}
		}
		return lang.Target{}
	case kChannel:
		// NIX_PATH decides what <nixpkgs> is on each machine.
		return lang.Target{Ecosystem: ecoNix, Package: imp.Module, Floating: true}
	case kFetch:
		ref := decodeRef(imp.Module)
		if ref.typ == "path" {
			return r.localFlake(localDir(dir, ref.url))
		}
		t, _ := ref.target()
		return t
	case kInput:
		name, _, _ := strings.Cut(imp.Module, "\n")
		return r.input(dir, name, 0)
	case kUse:
		d, ok := nearest(dir, func(d string) bool { fd := r.flakes[d]; return fd != nil && fd.inputs[imp.Module] != "" })
		if !ok {
			return lang.Target{}
		}
		return r.input(d, imp.Module, 0)
	case kPin:
		sdir, name, _ := strings.Cut(imp.Module, "\n")
		if p, ok := r.pins[localDir(dir, sdir)][name]; ok {
			return pinTarget(p)
		}
		return lang.Target{}
	case kPkg:
		if r.inNixpkgs {
			return r.byName(imp.Module)
		}
		t := lang.Target{Ecosystem: ecoNixpkgs, Package: imp.Module}
		if d, ok := nearest(dir, func(d string) bool { _, ok := r.nixpkgs[d]; return ok }); ok {
			n := r.nixpkgs[d]
			t.Version, t.Pinned, t.Floating = n.Version, n.Pinned, n.Floating
		}
		return t
	case "lock":
		if fd := r.flakes[dir]; fd != nil && fd.lock != nil {
			t, local := fd.lock.target(dir, imp.Module)
			if local != "" {
				return r.localFlake(local)
			}
			return t
		}
	case "niv", "npins":
		if p, ok := r.pins[dir][imp.Module]; ok {
			return pinTarget(p)
		}
	}
	return lang.Target{}
}

// byName is, inside nixpkgs, the file defining a top-level package by the
// pkgs/by-name convention (pkgs/by-name/he/hello/package.nix); other attributes
// are dropped, since the repository defines them itself.
//
// Implements: REQ-NIX-007
func (r *resolver) byName(attr string) lang.Target {
	if strings.Contains(attr, ".") || len(attr) < 2 {
		return lang.Target{}
	}
	p := "pkgs/by-name/" + strings.ToLower(attr[:2]) + "/" + attr + "/package.nix"
	if r.files[p] {
		return lang.Target{Local: p}
	}
	return lang.Target{}
}

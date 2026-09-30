package nix

import (
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// flakeDirectory is a directory with flake.nix and/or flake.lock.
type flakeDirectory struct {
	inputs map[string]string // input name -> RawImport.Module of its declaration
	lock   *lockFile
}

type resolver struct {
	lang.Layout
	flakes       map[string]*flakeDirectory
	pins         map[string]pinsFile      // directory of a sources.json -> its pins
	nixpkgs      map[string]lang.Target   // project directory -> the nixpkgs it builds with
	dependencies map[string][]lang.Target // lock target key -> its inputs' targets
	// inNixpkgs is set when the repository is nixpkgs itself, whose package lists
	// name its own packages.
	inNixpkgs bool
}

func newResolver(all []*scan.File) *resolver {
	r := &resolver{Layout: lang.NewLayout(), flakes: map[string]*flakeDirectory{},
		pins: map[string]pinsFile{}, nixpkgs: map[string]lang.Target{}, dependencies: map[string][]lang.Target{}}
	flake := func(d string) *flakeDirectory {
		if r.flakes[d] == nil {
			r.flakes[d] = &flakeDirectory{inputs: map[string]string{}}
		}
		return r.flakes[d]
	}
	for _, f := range all {
		r.Add(f.Path)
		c := class(f.Path)
		if c == "" {
			continue
		}
		source, ok := lang.ReadScanned(f)
		if !ok {
			continue
		}
		d := path.Dir(f.Path)
		switch c {
		case "flake":
			found := flake(d)
			for _, rawImport := range extract(source, true).Imports {
				if rawImport.Name == kInput {
					name, _, _ := strings.Cut(rawImport.Module, "\n")
					found.inputs[name] = rawImport.Module
				}
			}
		case "lock":
			flake(d).lock = readLock(source)
		case "niv", "npins":
			r.pins[d] = readPins(c, source)
		}
	}
	r.inNixpkgs = r.Files["pkgs/top-level/all-packages.nix"]
	for _, d := range lang.SortedKeys(r.flakes) {
		flake := r.flakes[d]
		if flake.lock != nil {
			r.lockDependencies(d, flake.lock)
		}
		if t, ok := r.flakeNixpkgs(d, flake); ok {
			r.nixpkgs[d] = t
		}
	}
	for _, d := range lang.SortedKeys(r.pins) {
		if p, ok := r.pins[d]["nixpkgs"]; ok {
			if _, has := r.nixpkgs[path.Dir(d)]; !has {
				r.nixpkgs[path.Dir(d)] = pinTarget(p)
			}
		}
	}
	return r
}

func key(t lang.Target) string { return t.Ecosystem + "\x00" + t.Package + "\x00" + t.Version }

// lockDependencies records, for each node of a lock, the nodes its inputs name (follows
// resolved), for --resolve-depth.
func (r *resolver) lockDependencies(directory string, l *lockFile) {
	for _, k := range l.keys() {
		t, local := l.target(directory, k)
		if local != "" || t.Package == "" {
			continue
		}
		kk := key(t)
		if _, done := r.dependencies[kk]; done {
			continue
		}
		var out []lang.Target
		for _, name := range lang.SortedKeys(l.Nodes[k].Inputs) {
			if nk, ok := l.input(k, name); ok {
				if d, local := l.target(directory, nk); local == "" && d.Package != "" {
					out = append(out, d)
				}
			}
		}
		r.dependencies[kk] = out
	}
}

// Dependencies lists what a locked input itself depends on, from flake.lock.
//
// Implements: REQ-NIX-005
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecosystemNix {
		return nil
	}
	return r.dependencies[key(t)]
}

// flakeNixpkgs is the nixpkgs a flake builds with: its input named nixpkgs, else
// the one input that is NixOS/nixpkgs.
func (r *resolver) flakeNixpkgs(directory string, flake *flakeDirectory) (lang.Target, bool) {
	names := []string{"nixpkgs"}
	for _, n := range lang.SortedKeys(flake.inputs) {
		if n != "nixpkgs" {
			names = append(names, n)
		}
	}
	if flake.lock != nil {
		for _, n := range lang.SortedKeys(flake.lock.Nodes[flake.lock.Root].Inputs) {
			if n != "nixpkgs" {
				names = append(names, n)
			}
		}
	}
	for _, n := range names {
		t := r.input(directory, n, 0)
		if t.Package == "nixpkgs" || strings.EqualFold(t.Package, "github.com/NixOS/nixpkgs") {
			return t, true
		}
	}
	return lang.Target{}, false
}

// input resolves a flake's input by name: through the lock beside flake.nix, else
// by the declaration's own reference (a follows of another top-level input taken
// through to it).
func (r *resolver) input(directory, name string, depth int) lang.Target {
	flake := r.flakes[directory]
	if flake == nil || depth > 8 {
		return lang.Target{}
	}
	if flake.lock != nil {
		if k, ok := flake.lock.input(flake.lock.Root, name); ok {
			t, local := flake.lock.target(directory, k)
			if local != "" {
				return r.localFlake(local)
			}
			return t
		}
	}
	declaration, ok := flake.inputs[name]
	if !ok {
		return lang.Target{}
	}
	_, rest, _ := strings.Cut(declaration, "\n")
	encoded, follows, _ := strings.Cut(rest, "\n")
	if follows != "" {
		if strings.Contains(follows, "/") {
			return lang.Target{} // an input of an input: only the lock knows it
		}
		return r.input(directory, follows, depth+1)
	}
	reference := decodeReference(encoded)
	if reference.typeName == "path" {
		return r.localFlake(localDirectory(directory, reference.url))
	}
	t, _ := reference.target()
	return t
}

// localFlake is a path input: the flake.nix of that directory, else the directory.
func (r *resolver) localFlake(d string) lang.Target {
	switch {
	case d == "" || d == ".":
		return lang.Target{}
	case r.Files[d+"/flake.nix"]:
		return lang.Target{Local: d + "/flake.nix"}
	case r.Directories[d]:
		return lang.Target{Local: d}
	}
	return lang.Target{}
}

// nearest is the closest directory at or above directory for which has is true, else
// "" (and false).
func nearest(directory string, has func(string) bool) (string, bool) {
	for d := range lang.DirectoryAndAncestors(directory) {
		if has(d) {
			return d, true
		}
	}
	return "", false
}

// Implements: REQ-NIX-002, REQ-NIX-004, REQ-NIX-005, REQ-NIX-007, REQ-NIX-008, REQ-NIX-009
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	directory := path.Dir(file)
	switch rawImport.Name {
	case kImport, kPath:
		p := localDirectory(directory, rawImport.Module)
		switch {
		case p == "":
		case r.Files[p]:
			return lang.Target{Local: p}
		case rawImport.Name == kImport && r.Files[p+"/default.nix"]:
			return lang.Target{Local: p + "/default.nix"}
		case r.Directories[p] && p != directory:
			return lang.Target{Local: p}
		}
		return lang.Target{}
	case kChannel:
		// NIX_PATH decides what <nixpkgs> is on each machine.
		return lang.Target{Ecosystem: ecosystemNix, Package: rawImport.Module, Floating: true}
	case kFetch:
		reference := decodeReference(rawImport.Module)
		if reference.typeName == "path" {
			return r.localFlake(localDirectory(directory, reference.url))
		}
		t, _ := reference.target()
		return t
	case kInput:
		name, _, _ := strings.Cut(rawImport.Module, "\n")
		return r.input(directory, name, 0)
	case kUse:
		d, ok := nearest(directory, func(d string) bool { flake := r.flakes[d]; return flake != nil && flake.inputs[rawImport.Module] != "" })
		if !ok {
			return lang.Target{}
		}
		return r.input(d, rawImport.Module, 0)
	case kPin:
		relativeDirectory, name, _ := strings.Cut(rawImport.Module, "\n")
		if p, ok := r.pins[localDirectory(directory, relativeDirectory)][name]; ok {
			return pinTarget(p)
		}
		return lang.Target{}
	case kPackage:
		if r.inNixpkgs {
			return r.byName(rawImport.Module)
		}
		t := lang.Target{Ecosystem: ecosystemNixpkgs, Package: rawImport.Module}
		if d, ok := nearest(directory, func(d string) bool { _, ok := r.nixpkgs[d]; return ok }); ok {
			n := r.nixpkgs[d]
			t.Version, t.Pinned, t.Floating = n.Version, n.Pinned, n.Floating
		}
		return t
	case "lock":
		if flake := r.flakes[directory]; flake != nil && flake.lock != nil {
			t, local := flake.lock.target(directory, rawImport.Module)
			if local != "" {
				return r.localFlake(local)
			}
			return t
		}
	case "niv", "npins":
		if p, ok := r.pins[directory][rawImport.Module]; ok {
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
func (r *resolver) byName(attribute string) lang.Target {
	if strings.Contains(attribute, ".") || len(attribute) < 2 {
		return lang.Target{}
	}
	p := "pkgs/by-name/" + strings.ToLower(attribute[:2]) + "/" + attribute + "/package.nix"
	if r.Files[p] {
		return lang.Target{Local: p}
	}
	return lang.Target{}
}

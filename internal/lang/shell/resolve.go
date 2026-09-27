package shell

import (
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

type resolver struct {
	files map[string]bool
	// ends maps the last two or more path segments of every project file to that
	// file, or to "" when several files end the same way.
	ends map[string]string
}

func newResolver(all []*scan.File) *resolver {
	r := &resolver{files: map[string]bool{}, ends: map[string]string{}}
	for _, f := range all {
		r.files[f.Path] = true
		for i := strings.IndexByte(f.Path, '/'); i >= 0; {
			end := f.Path[i+1:]
			if !strings.Contains(end, "/") {
				break
			}
			if _, seen := r.ends[end]; seen {
				r.ends[end] = ""
			} else {
				r.ends[end] = f.Path
			}
			j := strings.IndexByte(end, '/')
			i += j + 1
		}
		if strings.Contains(f.Path, "/") {
			if _, seen := r.ends[f.Path]; seen {
				r.ends[f.Path] = ""
			} else {
				r.ends[f.Path] = f.Path
			}
		}
	}
	return r
}

// Resolve maps a sourced or run path to the project file, and an installed package
// to its ecosystem's package.
//
// Implements: REQ-SHELL-004, REQ-SHELL-005, REQ-SHELL-006, REQ-SHELL-007, REQ-SHELL-008
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	dir := path.Dir(file)
	switch imp.Name {
	case kindSource, kindExec, kindFile:
		return r.local(file, r.candidates(dir, imp.Module))
	case kindBats:
		var candidates []string
		for _, c := range r.candidates(dir, imp.Module) {
			candidates = append(candidates, c+".bash", c)
		}
		return r.local(file, candidates)
	case kindEnvrc:
		var candidates []string
		for _, c := range r.candidates(dir, imp.Module) {
			candidates = append(candidates, c, path.Join(c, ".envrc"))
		}
		return r.local(file, candidates)
	case kindDotenv:
		return r.local(file, r.candidates(dir, imp.Module))
	case kindUp:
		for d := dir; d != "."; {
			d = path.Dir(d)
			if p := path.Join(d, imp.Module); r.files[p] {
				return lang.Target{Local: p}
			}
		}
		return lang.Target{}
	}
	eco, version, ok := strings.Cut(imp.Name, "@")
	if !ok {
		return lang.Target{}
	}
	return pkgTarget(eco, imp.Module, version)
}

// candidates are the project paths a module may name, most likely first. A path
// relative to the working directory is tried beside the script, then at the root:
// scripts are run from both, and the file cannot say which. A path below a
// directory the environment names is the one project file ending in it.
//
// Implements: REQ-SHELL-004, REQ-SHELL-009
func (r *resolver) candidates(dir, module string) []string {
	kind, rel, _ := strings.Cut(module, ":")
	var bases []string
	switch kind {
	case "any":
		if p := r.ends[path.Clean(rel)]; p != "" {
			return []string{p}
		}
	case "dir":
		bases = []string{dir}
	case "root":
		bases = []string{"."}
	case "cwd":
		bases = []string{dir, "."}
	}
	var out []string
	for _, b := range bases {
		if p := path.Join(b, rel); p != ".." && !strings.HasPrefix(p, "../") {
			out = append(out, p)
		}
	}
	return out
}

// local is the first candidate that is a project file other than the importer.
func (r *resolver) local(file string, candidates []string) lang.Target {
	for _, p := range candidates {
		if r.files[p] {
			if p == file {
				return lang.Target{}
			}
			return lang.Target{Local: p}
		}
	}
	return lang.Target{}
}

// pkgTarget applies each ecosystem's pinning rule to the version a command asked
// for: pip's ==1.2.3 and gem's 1.2.3 pin; npm, Go and cargo need a complete
// 1.2.3; no version, or latest, floats; a variable pins nothing.
//
// Implements: REQ-SHELL-008
func pkgTarget(eco, name, version string) lang.Target {
	t := lang.Target{Ecosystem: eco, Package: name, Version: version}
	switch {
	case version == "":
		t.Floating = true
		return t
	case version == "latest":
		t.Floating = true
		return t
	case strings.Contains(version, "$"):
		return t
	}
	switch eco {
	case ecoPyPI:
		t.Pinned = lang.Pinned(version)
		if t.Pinned {
			t.Version = strings.TrimPrefix(version, "==")
		}
	case ecoGems:
		v := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(version), "="))
		if t.Pinned = lang.Pinned(v); t.Pinned {
			t.Version = v
		}
	case ecoCrates:
		v := strings.TrimPrefix(version, "=")
		if t.Pinned = lang.PinnedSemver(v); t.Pinned {
			t.Version = v
		}
	default: // npm, go
		t.Pinned = lang.PinnedSemver(version)
	}
	return t
}

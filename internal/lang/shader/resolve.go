package shader

import (
	"bytes"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/cpp"
	"github.com/sarumaj/depphunter-cli/internal/lang/rust"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

type resolver struct {
	files   map[string]bool
	folded  map[string]string   // lower-case path -> path
	byBase  map[string][]string // lower-case file name -> paths
	modules map[string][]string // naga_oil #define_import_path -> the files declaring it
	inc     func() cpp.Includes
	crates  func() *rust.Crates // nil without a Cargo.toml
}

var defineImportPath = regexp.MustCompile(`(?m)^[ \t]*#[ \t]*define_import_path[ \t]+([\w:]+)`)

// Implements: REQ-SHADER-004, REQ-SHADER-006
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{files: map[string]bool{}, folded: map[string]string{}, byBase: map[string][]string{}, modules: map[string][]string{}}
	cargo := false
	for _, f := range all {
		r.files[f.Path] = true
		lower := strings.ToLower(f.Path)
		r.folded[lower] = f.Path
		r.byBase[path.Base(lower)] = append(r.byBase[path.Base(lower)], f.Path)
		cargo = cargo || path.Base(f.Path) == "Cargo.toml"
		if exts[strings.ToLower(path.Ext(f.Path))] != wgsl || f.Binary || f.TooLarge || f.Size > lang.MaxParseSize {
			continue
		}
		if src, err := os.ReadFile(f.Abs); err == nil && bytes.Contains(src, []byte("define_import_path")) {
			for _, m := range defineImportPath.FindAllSubmatch(src, -1) {
				r.modules[string(m[1])] = append(r.modules[string(m[1])], f.Path)
			}
		}
	}
	r.inc = sync.OnceValue(func() cpp.Includes { return cpp.NewIncludes(root, all) })
	r.crates = sync.OnceValue(func() *rust.Crates {
		if !cargo {
			return nil
		}
		c := rust.NewCrates(all)
		return &c
	})
	return r
}

// Implements: REQ-SHADER-004, REQ-SHADER-006, REQ-SHADER-007
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	if imp.Name == nagaImport || imp.Name == weslImport {
		return r.module(file, imp.Module)
	}
	return r.include(file, imp)
}

// drop answers the cpp resolution for every header that is not the project's: a
// shader includes nothing from C's libraries.
func drop(string, bool) (lang.Target, bool) { return lang.Target{}, true }

// include follows an #include of GLSL or HLSL: an Unreal virtual path, else as the
// cpp plugin resolves a project header, else against the includer's directory and
// its parents, ignoring case last (HLSL is mostly written on Windows), else the
// file whose path ends in it, ignoring case.
//
// Implements: REQ-SHADER-004, REQ-SHADER-009
func (r *resolver) include(file string, imp lang.RawImport) lang.Target {
	name := strings.ReplaceAll(imp.Module, `\`, "/")
	if strings.HasPrefix(name, "/") {
		return r.virtual(file, name)
	}
	if t := r.inc().Resolve(file, imp, drop); t.Local != "" {
		return t
	}
	if p := r.upward(file, name); p != "" {
		return lang.Target{Local: p}
	}
	if p := r.suffix(file, name); p != "" {
		return lang.Target{Local: p}
	}
	return lang.Target{}
}

// upward finds name relative to the includer's directory or one of its parents,
// exactly, else ignoring case.
func (r *resolver) upward(file, name string) string {
	for _, fold := range []bool{false, true} {
		dir := path.Dir(file)
		for {
			p := path.Join(dir, name)
			if !strings.HasPrefix(p, "../") {
				if r.files[p] {
					return p
				}
				if q, ok := r.folded[strings.ToLower(p)]; ok && fold {
					return q
				}
			}
			if dir == "." || dir == "/" {
				break
			}
			dir = path.Dir(dir)
		}
	}
	return ""
}

// virtual follows an include by a virtual path: Unreal maps /Engine/ to the
// engine's Shaders/ directory, /Plugin/Name/ to the plugin's and /Project/ to the
// project's; GL_ARB_shading_language_include names strings from the root.
//
// Implements: REQ-SHADER-004
func (r *resolver) virtual(file, name string) lang.Target {
	root, rest, _ := strings.Cut(strings.TrimPrefix(name, "/"), "/")
	switch root {
	case "Engine":
		if p := r.suffix(file, "Engine/Shaders/"+rest); p != "" {
			return lang.Target{Local: p}
		}
		return lang.Target{Ecosystem: ecoUnreal, Package: "Engine"}
	case "Plugin":
		plugin, rest, _ := strings.Cut(rest, "/")
		if p := r.suffix(file, plugin+"/Shaders/"+rest); p != "" {
			return lang.Target{Local: p}
		}
		if plugin == "" || rest == "" {
			return lang.Target{}
		}
		return lang.Target{Ecosystem: ecoUnreal, Package: plugin}
	case "Project":
		if p := r.suffix(file, "Shaders/"+rest); p != "" {
			return lang.Target{Local: p}
		}
		return lang.Target{}
	}
	if p := path.Clean(strings.TrimPrefix(name, "/")); r.files[p] {
		return lang.Target{Local: p}
	}
	if p := r.suffix(file, strings.TrimPrefix(name, "/")); p != "" {
		return lang.Target{Local: p}
	}
	return lang.Target{}
}

// suffix finds the project file whose path ends in name, ignoring case: the only
// one, or the one sharing the most directories with file.
func (r *resolver) suffix(file, name string) string {
	name = strings.ToLower(path.Clean(name))
	if name == "." || strings.HasPrefix(name, "../") {
		return ""
	}
	var best string
	bestLen, tie := -1, false
	for _, p := range r.byBase[path.Base(name)] {
		if l := strings.ToLower(p); l != name && !strings.HasSuffix(l, "/"+name) {
			continue
		}
		switch n := commonDirs(file, p); {
		case n > bestLen:
			best, bestLen, tie = p, n, false
		case n == bestLen:
			tie = true
		}
	}
	if tie {
		return ""
	}
	return best
}

func commonDirs(a, b string) int {
	as, bs := strings.Split(path.Dir(a), "/"), strings.Split(path.Dir(b), "/")
	n := 0
	for n < len(as) && n < len(bs) && as[n] == bs[n] && as[n] != "." {
		n++
	}
	return n
}

// moduleExts are the extensions of a WGSL module's file.
var moduleExts = []string{".wesl", ".wgsl"}

// module follows a WGSL import: a quoted file, a path below the package root
// (package::), the importer's parent (super::), a module some file declares with
// #define_import_path, or a crate the Cargo manifests declare.
//
// Implements: REQ-SHADER-006, REQ-SHADER-007
func (r *resolver) module(file, mod string) lang.Target {
	if strings.HasPrefix(mod, `"`) {
		return r.quoted(file, mod)
	}
	segments := strings.Split(mod, "::")
	switch segments[0] {
	case "package":
		for dir := path.Dir(file); ; dir = path.Dir(dir) {
			if p := r.probe(dir, segments[1:]); p != "" {
				return lang.Target{Local: p}
			}
			if dir == "." || dir == "/" {
				return lang.Target{}
			}
		}
	case "super":
		dir := path.Dir(file)
		for segments = segments[1:]; len(segments) > 0 && segments[0] == "super"; segments = segments[1:] {
			dir = path.Dir(dir)
		}
		if p := r.probe(dir, segments); p != "" {
			return lang.Target{Local: p}
		}
		return lang.Target{}
	case "self":
		base := path.Base(file)
		if p := r.probe(path.Join(path.Dir(file), strings.TrimSuffix(base, path.Ext(base))), segments[1:]); p != "" {
			return lang.Target{Local: p}
		}
		return lang.Target{}
	}
	for k := len(segments); k >= 1; k-- {
		if files := r.modules[strings.Join(segments[:k], "::")]; len(files) > 0 {
			return lang.Target{Local: closest(file, files)}
		}
	}
	return r.crate(file, segments)
}

// crate resolves a module path whose first segment names a crate: a crate of the
// project to the module's file in it (below src/, else the crate's directory), a
// declared dependency to its crates.io package, and Bevy's crates (bevy_pbr,
// bevy_render ...), which the bevy crate depends on, to themselves when a manifest
// declares bevy.
//
// Implements: REQ-SHADER-007
func (r *resolver) crate(file string, segments []string) lang.Target {
	c := r.crates()
	if c == nil || segments[0] == "" {
		return lang.Target{}
	}
	if t, ok := c.Crate(file, segments[0]); ok {
		if t.Local == "" {
			return t
		}
		for _, root := range []string{"src", "shaders", "src/shaders", ""} {
			if p := r.probe(path.Join(t.Local, root), segments[1:]); p != "" {
				return lang.Target{Local: p}
			}
		}
		return t
	}
	if !strings.HasPrefix(segments[0], "bevy_") {
		return lang.Target{}
	}
	bevy, ok := c.Crate(file, "bevy")
	if !ok || bevy.Local != "" {
		return lang.Target{}
	}
	// Bevy's crates are released together: the lock's version, else bevy's
	// requirement.
	if v := c.Locked(segments[0]); v != "" {
		return lang.Target{Ecosystem: ecoCrates, Package: segments[0], Version: v, Pinned: true}
	}
	v := bevy.Requested
	if v == "" {
		v = bevy.Version
	}
	return lang.Target{Ecosystem: ecoCrates, Package: segments[0], Version: v}
}

// probe finds the module file of the longest prefix of segments below dir.
func (r *resolver) probe(dir string, segments []string) string {
	for k := len(segments); k >= 1; k-- {
		p := path.Join(append([]string{dir}, segments[:k]...)...)
		for _, ext := range moduleExts {
			if r.files[p+ext] {
				return p + ext
			}
		}
	}
	return ""
}

// quoted follows naga_oil's import of a file ("shaders/util.wgsl", which Bevy reads
// below an assets/ directory; embedded://crate/path): beside the importer, below
// an assets/ directory or directly under one of its parents, else the only file so
// ending.
func (r *resolver) quoted(file, mod string) lang.Target {
	name, _, _ := strings.Cut(strings.TrimPrefix(mod, `"`), `"`)
	if _, rest, ok := strings.Cut(name, "://"); ok {
		// embedded://bevy_pbr/render/pbr.wgsl is a file below the crate's src/.
		name = strings.TrimPrefix(rest, "/")
		if crate, sub, ok := strings.Cut(name, "/"); ok && r.crates() != nil {
			if t, ok := r.crates().Crate(file, crate); ok && t.Local != "" && r.files[path.Join(t.Local, "src", sub)] {
				return lang.Target{Local: path.Join(t.Local, "src", sub)}
			}
		}
	}
	name = strings.TrimPrefix(name, "/")
	if name == "" {
		return lang.Target{}
	}
	for dir := path.Dir(file); ; dir = path.Dir(dir) {
		for _, p := range []string{path.Join(dir, name), path.Join(dir, "assets", name)} {
			if r.files[p] && !strings.HasPrefix(p, "../") {
				return lang.Target{Local: p}
			}
		}
		if dir == "." || dir == "/" {
			break
		}
	}
	if p := r.suffix(file, name); p != "" {
		return lang.Target{Local: p}
	}
	return lang.Target{}
}

// closest is the file sharing the most directories with from; the first listed on a
// tie.
func closest(from string, files []string) string {
	best, bestLen := files[0], -1
	for _, f := range files {
		if n := commonDirs(from, f); n > bestLen {
			best, bestLen = f, n
		}
	}
	return best
}

package d

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// packageDirs are the directories dub fetches packages into, most specific
// first: the repository's .dub/packages (dub fetch --cache=local), then
// $DUB_HOME/packages, $DPATH/dub/packages, else ~/.dub/packages (on Windows
// %LOCALAPPDATA%\dub\packages).
//
// Implements: REQ-DLANG-008
func packageDirs(root string) []string {
	var out []string
	if root != "" {
		out = append(out, filepath.Join(root, ".dub", "packages"))
	}
	if h := os.Getenv("DUB_HOME"); h != "" {
		return append(out, filepath.Join(h, "packages"))
	}
	if h := os.Getenv("DPATH"); h != "" {
		return append(out, filepath.Join(h, "dub", "packages"))
	}
	if runtime.GOOS == "windows" {
		if h := os.Getenv("LOCALAPPDATA"); h != "" {
			return append(out, filepath.Join(h, "dub", "packages"))
		}
	}
	if h, err := os.UserHomeDir(); err == nil {
		out = append(out, filepath.Join(h, ".dub", "packages"))
	}
	return out
}

// maxInstalledFiles bounds the modules read from one installed package.
const maxInstalledFiles = 20000

// readInstalled indexes the modules of the packages the repository declares or
// selects that dub fetched onto this machine: packages/<name>/<version>/<name>/
// (dub 1.31 and later) or packages/<name>-<version>/<name>/. The version
// dub.selections.json selects is read when it is there, else the newest.
//
// Implements: REQ-DLANG-008
func (r *resolver) readInstalled(root string, projects []*project) {
	wanted := map[string]string{} // name -> selected version
	for _, p := range projects {
		for _, d := range p.recipe.deps {
			if b := base(d.name); b != "" {
				if _, ok := wanted[b]; !ok {
					wanted[b] = ""
				}
			}
		}
		for name, s := range p.sel {
			if s.path == "" && s.repo == "" {
				wanted[name] = s.version
			} else if _, ok := wanted[name]; !ok {
				wanted[name] = ""
			}
		}
	}
	for _, p := range projects {
		delete(wanted, p.recipe.name)
	}
	if len(wanted) == 0 {
		return
	}
	for _, dir := range packageDirs(root) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		found := map[string][]string{} // name -> version directories (package root = <dir>/<name>)
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			n := e.Name()
			if _, ok := wanted[n]; ok {
				// New layout: <name>/<version>/<name>/.
				if vs, err := os.ReadDir(filepath.Join(dir, n)); err == nil {
					for _, v := range vs {
						if v.IsDir() {
							found[n] = append(found[n], filepath.Join(dir, n, v.Name()))
						}
					}
				}
				continue
			}
			// Old layout: <name>-<version>/<name>/.
			for name := range wanted {
				if strings.HasPrefix(n, name+"-") {
					if st, err := os.Stat(filepath.Join(dir, n, name)); err == nil && st.IsDir() {
						found[name] = append(found[name], filepath.Join(dir, n))
					}
				}
			}
		}
		for _, name := range sortedKeys(found) {
			if r.recipes[name] != nil {
				continue // a more specific directory had it
			}
			pkg := pickVersion(found[name], name, wanted[name])
			if pkg == "" {
				continue
			}
			r.indexInstalled(name, filepath.Join(pkg, name))
		}
	}
}

// pickVersion is the version directory of the selected version, else the
// newest; an old-layout directory's version follows the name and a dash.
func pickVersion(dirs []string, name, want string) string {
	version := func(d string) string {
		b := filepath.Base(d)
		if v, ok := strings.CutPrefix(b, name+"-"); ok {
			return v
		}
		return b
	}
	sort.Slice(dirs, func(i, j int) bool { return compareVersions(version(dirs[i]), version(dirs[j])) > 0 })
	for _, d := range dirs {
		if want != "" && version(d) == want {
			return d
		}
	}
	if len(dirs) == 0 {
		return ""
	}
	return dirs[0]
}

// compareVersions orders dub versions by their numeric segments; a branch
// (~master) sorts below every release.
func compareVersions(a, b string) int {
	if strings.HasPrefix(a, "~") != strings.HasPrefix(b, "~") {
		if strings.HasPrefix(a, "~") {
			return -1
		}
		return 1
	}
	pa, pb := strings.FieldsFunc(a, notDigit), strings.FieldsFunc(b, notDigit)
	for i := 0; i < len(pa) && i < len(pb); i++ {
		if len(pa[i]) != len(pb[i]) {
			if len(pa[i]) < len(pb[i]) {
				return -1
			}
			return 1
		}
		if pa[i] != pb[i] {
			if pa[i] < pb[i] {
				return -1
			}
			return 1
		}
	}
	return len(pa) - len(pb)
}

func notDigit(r rune) bool { return r < '0' || r > '9' }

// readRecipeDir reads the recipe in dir: dub.json, else dub.sdl.
func readRecipeDir(dir string) *recipe {
	if data, err := os.ReadFile(filepath.Join(dir, "dub.json")); err == nil {
		return readJSONRecipe(data)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "dub.sdl")); err == nil {
		return readSDLRecipe(data)
	}
	return nil
}

// indexInstalled records an installed package's recipe (with its sub-package
// directories' recipes as inline ones) and maps the modules under its import
// directories to it.
func (r *resolver) indexInstalled(name, dir string) {
	rec := readRecipeDir(dir)
	if rec == nil {
		return
	}
	type part struct {
		dir string
		rec *recipe
	}
	parts := []part{{dir, rec}}
	for _, s := range rec.subs {
		if s.inline != nil {
			parts = append(parts, part{dir, s.inline})
			continue
		}
		sd := filepath.Join(dir, filepath.FromSlash(s.path))
		if sub := readRecipeDir(sd); sub != nil {
			parts = append(parts, part{sd, sub})
			rec.subs = append(rec.subs, subPackage{inline: sub})
		}
	}
	r.recipes[name] = rec
	files := 0
	for _, pt := range parts {
		var dirs []string
		for _, set := range []struct {
			list []string
			ok   bool
		}{{pt.rec.importPaths, pt.rec.importsSet}, {pt.rec.sourcePaths, pt.rec.sourcesSet}} {
			if set.ok {
				for _, q := range set.list {
					dirs = append(dirs, filepath.Join(pt.dir, filepath.FromSlash(q)))
				}
				continue
			}
			for _, def := range []string{"source", "src"} {
				if st, err := os.Stat(filepath.Join(pt.dir, def)); err == nil && st.IsDir() {
					dirs = append(dirs, filepath.Join(pt.dir, def))
				}
			}
		}
		for _, d := range dirs {
			filepath.WalkDir(d, func(p string, e fs.DirEntry, err error) error {
				if err != nil || files >= maxInstalledFiles {
					return filepath.SkipDir
				}
				if e.IsDir() {
					if p != d && strings.HasPrefix(e.Name(), ".") {
						return filepath.SkipDir
					}
					return nil
				}
				ext := filepath.Ext(p)
				if ext != ".d" && ext != ".di" {
					return nil
				}
				rel, err := filepath.Rel(d, p)
				if err != nil {
					return nil
				}
				files++
				mod := strings.TrimSuffix(filepath.ToSlash(rel), ext)
				mod = strings.ReplaceAll(strings.TrimSuffix(mod, "/package"), "/", ".")
				if _, ok := r.installed[mod]; !ok && mod != "" && mod != "package" {
					r.installed[mod] = name
				}
				return nil
			})
		}
	}
}

package d

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// packageDirectories are the directories dub fetches packages into, most specific
// first: the repository's .dub/packages (dub fetch --cache=local), then
// $DUB_HOME/packages, $DPATH/dub/packages, else ~/.dub/packages (on Windows
// %LOCALAPPDATA%\dub\packages); relative variables are taken against the
// repository root.
//
// Implements: REQ-DLANG-008
func packageDirectories(root string) []packageDirectory {
	var out []packageDirectory
	if root != "" {
		out = append(out, packageDirectory{lang.OpenRoot(root), filepath.Join(root, ".dub", "packages")})
	}
	if h, files := lang.FromEnvironment(root, os.Getenv("DUB_HOME")); h != "" {
		return append(out, packageDirectory{files, filepath.Join(h, "packages")})
	}
	if h, files := lang.FromEnvironment(root, os.Getenv("DPATH")); h != "" {
		return append(out, packageDirectory{files, filepath.Join(h, "dub", "packages")})
	}
	if runtime.GOOS == "windows" {
		if h, files := lang.FromEnvironment(root, os.Getenv("LOCALAPPDATA")); h != "" {
			return append(out, packageDirectory{files, filepath.Join(h, "dub", "packages")})
		}
	}
	if h, err := os.UserHomeDir(); err == nil {
		out = append(out, packageDirectory{lang.Machine, filepath.Join(h, ".dub", "packages")})
	}
	return out
}

// packageDirectory is a directory dub fetches packages into and how it is read:
// through the repository's Root when it lies there, else through Machine.
type packageDirectory struct {
	files     lang.Root
	directory string
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
		for _, d := range p.recipe.dependencyList {
			if b := base(d.name); b != "" {
				if _, ok := wanted[b]; !ok {
					wanted[b] = ""
				}
			}
		}
		for name, s := range p.selections {
			if s.path == "" && s.repository == "" {
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
	for _, d := range packageDirectories(root) {
		files, directory := d.files, d.directory
		entries, err := files.ReadDir(directory)
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
				if entries, err := files.ReadDir(filepath.Join(directory, n)); err == nil {
					for _, v := range entries {
						if v.IsDir() {
							found[n] = append(found[n], filepath.Join(directory, n, v.Name()))
						}
					}
				}
				continue
			}
			// Old layout: <name>-<version>/<name>/.
			for name := range wanted {
				if strings.HasPrefix(n, name+"-") {
					if files.IsDirectory(filepath.Join(directory, n, name)) {
						found[name] = append(found[name], filepath.Join(directory, n))
					}
				}
			}
		}
		for _, name := range sortedKeys(found) {
			if r.recipes[name] != nil {
				continue // a more specific directory had it
			}
			packageName := pickVersion(found[name], name, wanted[name])
			if packageName == "" {
				continue
			}
			r.indexInstalled(files, name, filepath.Join(packageName, name))
		}
	}
}

// pickVersion is the version directory of the selected version, else the
// newest; an old-layout directory's version follows the name and a dash.
func pickVersion(directories []string, name, want string) string {
	version := func(d string) string {
		b := filepath.Base(d)
		if v, ok := strings.CutPrefix(b, name+"-"); ok {
			return v
		}
		return b
	}
	sort.Slice(directories, func(i, j int) bool { return compareVersions(version(directories[i]), version(directories[j])) > 0 })
	for _, d := range directories {
		if want != "" && version(d) == want {
			return d
		}
	}
	if len(directories) == 0 {
		return ""
	}
	return directories[0]
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
	aNumbers, bNumbers := strings.FieldsFunc(a, notDigit), strings.FieldsFunc(b, notDigit)
	for i := 0; i < len(aNumbers) && i < len(bNumbers); i++ {
		if len(aNumbers[i]) != len(bNumbers[i]) {
			if len(aNumbers[i]) < len(bNumbers[i]) {
				return -1
			}
			return 1
		}
		if aNumbers[i] != bNumbers[i] {
			if aNumbers[i] < bNumbers[i] {
				return -1
			}
			return 1
		}
	}
	return len(aNumbers) - len(bNumbers)
}

func notDigit(r rune) bool { return r < '0' || r > '9' }

// readRecipeDirectory reads the recipe in directory: dub.json, else dub.sdl.
func readRecipeDirectory(files lang.Root, directory string) *recipe {
	if data, ok := files.ReadBounded(filepath.Join(directory, "dub.json")); ok {
		return readJSONRecipe(data)
	}
	if data, ok := files.ReadBounded(filepath.Join(directory, "dub.sdl")); ok {
		return readSDLRecipe(data)
	}
	return nil
}

// indexInstalled records an installed package's recipe (with its sub-package
// directories' recipes as inline ones) and maps the modules under its import
// directories to it.
func (r *resolver) indexInstalled(files lang.Root, name, directory string) {
	found := readRecipeDirectory(files, directory)
	if found == nil {
		return
	}
	type part struct {
		directory string
		recipe    *recipe
	}
	parts := []part{{directory, found}}
	for _, s := range found.subs {
		if s.inline != nil {
			parts = append(parts, part{directory, s.inline})
			continue
		}
		subpackageDirectory := filepath.Join(directory, filepath.FromSlash(s.path))
		if subrecipe := readRecipeDirectory(files, subpackageDirectory); subrecipe != nil {
			parts = append(parts, part{subpackageDirectory, subrecipe})
			found.subs = append(found.subs, subPackage{inline: subrecipe})
		}
	}
	r.recipes[name] = found
	count := 0
	for _, part := range parts {
		var directories []string
		for _, set := range []struct {
			list []string
			ok   bool
		}{{part.recipe.importPaths, part.recipe.importsSet}, {part.recipe.sourcePaths, part.recipe.sourcesSet}} {
			if set.ok {
				for _, q := range set.list {
					directories = append(directories, filepath.Join(part.directory, filepath.FromSlash(q)))
				}
				continue
			}
			for _, defaultDirectory := range []string{"source", "src"} {
				if files.IsDirectory(filepath.Join(part.directory, defaultDirectory)) {
					directories = append(directories, filepath.Join(part.directory, defaultDirectory))
				}
			}
		}
		for _, d := range directories {
			files.WalkDir(d, func(p string, e fs.DirEntry, err error) error {
				if err != nil || count >= maxInstalledFiles {
					return filepath.SkipDir
				}
				if e.IsDir() {
					if p != d && strings.HasPrefix(e.Name(), ".") {
						return filepath.SkipDir
					}
					return nil
				}
				extension := filepath.Ext(p)
				if extension != ".d" && extension != ".di" {
					return nil
				}
				relative, err := filepath.Rel(d, p)
				if err != nil {
					return nil
				}
				count++
				module := strings.TrimSuffix(filepath.ToSlash(relative), extension)
				module = strings.ReplaceAll(strings.TrimSuffix(module, "/package"), "/", ".")
				if _, ok := r.installed[module]; !ok && module != "" && module != "package" {
					r.installed[module] = name
				}
				return nil
			})
		}
	}
}

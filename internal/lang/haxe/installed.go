package haxe

import (
	"cmp"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// lixScope is a directory whose haxe_libraries/ pins libraries for everything
// below it (lix and haxeshim read the nearest one).
type lixScope struct {
	directory string
	libraries map[string]*lixLibrary // lower-case name -> pin
	names     []string               // the keys, sorted
}

// lixLibrary is one haxe_libraries/<name>.hxml.
type lixLibrary struct {
	name, version    string
	pinned, floating bool
	origin           string
	local            string   // a class path in the repository: the library is the repository
	absolute         []string // class paths in lix's cache on this machine
	dependencies     []string // its -lib lines
}

func (l *lixLibrary) target() lang.Target {
	if l.local != "" {
		return lang.Target{Local: l.local}
	}
	return lang.Target{Ecosystem: ecosystemHaxelib, Package: l.name, Version: l.version, Pinned: l.pinned,
		Floating: l.floating, Origin: l.origin}
}

// lixScope is the scope governing a file: the nearest directory at or above it
// with a haxe_libraries/ directory.
func (r *resolver) lixScope(file string) *lixScope {
	if len(r.lix) == 0 {
		return nil
	}
	s, _ := lang.Nearest(r.lix, file)
	return s
}

// libraryCache is lix's download cache: HAXE_LIBCACHE, else haxe_libraries/ under
// HAXESHIM_ROOT or ~/haxe.
func libraryCache(getenv func(string) string) string {
	if c := getenv("HAXE_LIBCACHE"); c != "" {
		return c
	}
	if s := getenv("HAXESHIM_ROOT"); s != "" {
		return filepath.Join(s, "haxe_libraries")
	}
	if h := getenv("HOME"); h != "" {
		return filepath.Join(h, "haxe", "haxe_libraries")
	}
	return ""
}

// readLix reads every haxe_libraries/<name>.hxml: the version its
// `# @install: lix download` line pins (haxelib:/name#1.2.3, a git commit in
// gh://github.com/owner/repo#<commit>), its class paths and its -lib lines.
//
// Implements: REQ-HAXE-006
func (r *resolver) readLix(root string, all []*scan.File, getenv func(string) string) {
	cache := libraryCache(getenv)
	shim := getenv("HAXESHIM_ROOT")
	if shim == "" && getenv("HOME") != "" {
		shim = filepath.Join(getenv("HOME"), "haxe")
	}
	for _, f := range all {
		if class(f.Path) != classLix {
			continue
		}
		data, ok := lang.ReadScanned(f)
		if !ok {
			continue
		}
		scope := path.Dir(path.Dir(f.Path))
		name := strings.TrimSuffix(path.Base(f.Path), ".hxml")
		h := readHXML(data)
		l := &lixLibrary{name: name, dependencies: nil}
		for _, d := range h.libraries {
			l.dependencies = append(l.dependencies, d.name)
		}
		readInstall(l, h.install, h.defines[name])
		for _, classPath := range h.classPaths {
			switch {
			case strings.HasPrefix(classPath, "${SCOPE_DIR}"):
				if relative := path.Join(scope, strings.TrimPrefix(classPath, "${SCOPE_DIR}")); lang.Inside(relative) && r.directories[relative] && l.local == "" {
					l.local = relative
				}
			case strings.HasPrefix(classPath, "${HAXE_LIBCACHE}"), strings.HasPrefix(classPath, "${HAXESHIM_LIBCACHE}"):
				if _, rest, _ := strings.Cut(classPath, "}"); cache != "" {
					l.absolute = append(l.absolute, filepath.Join(cache, rest))
				}
			case strings.HasPrefix(classPath, "${HAXESHIM_ROOT}"):
				if shim != "" {
					l.absolute = append(l.absolute, filepath.Join(shim, strings.TrimPrefix(classPath, "${HAXESHIM_ROOT}")))
				}
			case strings.Contains(classPath, "$"):
			case filepath.IsAbs(classPath):
				l.absolute = append(l.absolute, classPath)
			default:
				if relative := path.Join(scope, classPath); lang.Inside(relative) && r.directories[relative] && l.local == "" {
					l.local = relative
				}
			}
		}
		s := r.lix[scope]
		if s == nil {
			s = &lixScope{directory: scope, libraries: map[string]*lixLibrary{}}
			r.lix[scope] = s
		}
		s.libraries[strings.ToLower(name)] = l
		r.lixFile[f.Path] = l
	}
	for _, s := range r.lix {
		s.names = lang.SortedKeys(s.libraries)
	}
}

// readInstall reads lix's install URL: haxelib:/name#1.2.3 pins that version,
// a git reference (gh://, gl://, git:) pins a commit, shows a tag and floats
// on a branch; no URL is a library in development (it floats), and a
// download URL shows the version the file defines.
func readInstall(l *lixLibrary, url, defined string) {
	switch {
	case url == "":
		l.version, l.floating = defined, true
	case strings.HasPrefix(url, "haxelib:"):
		_, v, _ := strings.Cut(strings.TrimLeft(strings.TrimPrefix(url, "haxelib:"), "/"), "#")
		v = cmp.Or(v, defined)
		l.version, l.pinned, l.floating = v, v != "", v == ""
	default:
		u, reference, _ := strings.Cut(url, "#")
		switch {
		case strings.HasPrefix(u, "gh://"):
			u = "https://" + strings.TrimPrefix(u, "gh://")
		case strings.HasPrefix(u, "gl://"):
			u = "https://" + strings.TrimPrefix(u, "gl://")
		case strings.HasPrefix(u, "github:"):
			u = "https://github.com/" + strings.TrimLeft(strings.TrimPrefix(u, "github:"), "/")
		case strings.HasPrefix(u, "gitlab:"):
			u = "https://gitlab.com/" + strings.TrimLeft(strings.TrimPrefix(u, "gitlab:"), "/")
		case strings.HasPrefix(u, "git:"):
			u = strings.TrimPrefix(u, "git:")
		}
		if !lang.PublicOrUnnamed(u) {
			l.origin = u
		}
		switch {
		case lang.Commit(reference):
			l.version, l.pinned = reference, true
		case reference != "" && tagLike(reference):
			l.version = reference
		case reference != "":
			l.version, l.floating = reference, true
		default:
			l.version = defined // an archive download
		}
	}
}

// installed is a library as haxelib installed it.
type installed struct {
	name, version string
	classPaths    []string // absolute class paths
	dependencies  []dependency
	local         bool // in the repository's own .haxelib/ repository
	// files reads the library's files: the repository's Root for a local
	// library, which a .dev path or class path cannot leave, else Machine.
	files lang.Root
}

// repositories are haxelib's repositories: a local one (.haxelib/) at the
// repository's root or beside a manifest, then the global one: HAXELIB_PATH,
// else the path in ~/.haxelib, else ~/haxelib.
func (r *resolver) repositories(root string, getenv func(string) string) (local []string, global string) {
	if root != "" {
		repository := lang.OpenRoot(root)
		directories := []string{"."}
		for _, m := range r.all {
			directories = append(directories, path.Dir(m.file))
		}
		seen := map[string]bool{}
		for _, d := range directories {
			absolute := filepath.Join(root, filepath.FromSlash(d), ".haxelib")
			if !seen[absolute] && repository.IsDirectory(absolute) {
				seen[absolute] = true
				local = append(local, absolute)
			}
		}
	}
	if p := getenv("HAXELIB_PATH"); p != "" {
		return local, p
	}
	if h := getenv("HOME"); h != "" {
		if data, err := os.ReadFile(filepath.Join(h, ".haxelib")); err == nil {
			if p := strings.TrimSpace(string(data)); p != "" && lang.IsDirectory(p) {
				return local, p
			}
		}
		if p := filepath.Join(h, "haxelib"); lang.IsDirectory(p) {
			return local, p
		}
	}
	return local, ""
}

// readInstalled finds the libraries installed for the repository: every
// library of its local repositories, and the declared ones in the global
// repository.
//
// Implements: REQ-HAXE-008
func (r *resolver) readInstalled(root string, getenv func(string) string) {
	local, global := r.repositories(root, getenv)
	wanted := map[string]string{} // lower name -> declared version
	var names []string
	for _, m := range r.all {
		for _, d := range m.libraries {
			k := strings.ToLower(d.name)
			if _, ok := wanted[k]; !ok {
				names = append(names, d.name)
				wanted[k] = d.version
			} else if wanted[k] == "" {
				wanted[k] = d.version
			}
		}
	}
	files := lang.OpenRoot(root)
	for _, repository := range local {
		entries, _ := files.ReadDir(repository)
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := strings.ReplaceAll(e.Name(), ",", ".")
			k := strings.ToLower(name)
			if r.installed[k] != nil {
				continue
			}
			if in := installedLibrary(files, filepath.Join(repository, e.Name()), name, wanted[k]); in != nil {
				in.local = true
				r.installed[k] = in
			}
		}
	}
	if global == "" {
		return
	}
	var listing []os.DirEntry
	listed := false
	for _, name := range names {
		k := strings.ToLower(name)
		if r.installed[k] != nil {
			continue
		}
		directory := filepath.Join(global, strings.ReplaceAll(name, ".", ","))
		if !lang.IsDirectory(directory) {
			// Names are case-insensitive in haxelib: look the directory up.
			if !listed {
				listing, _ = os.ReadDir(global)
				listed = true
			}
			directory = ""
			for _, e := range listing {
				if e.IsDir() && strings.EqualFold(strings.ReplaceAll(e.Name(), ",", "."), name) {
					directory = filepath.Join(global, e.Name())
					break
				}
			}
			if directory == "" {
				continue
			}
		}
		if in := installedLibrary(lang.Machine, directory, name, wanted[k]); in != nil {
			r.installed[k] = in
		}
	}
}

// installedLibrary reads a library's directory in a haxelib repository: the
// development path in .dev, else the declared version when it is installed,
// else the version .current names (1.2.3 in 1,2,3/, git in git/). files reads
// them: the repository's Root for its local repository, whose .dev may not
// point out of it, else Machine.
func installedLibrary(files lang.Root, libraryDirectory, name, want string) *installed {
	directory, version := "", ""
	if dev := readTrim(files, filepath.Join(libraryDirectory, ".dev")); dev != "" && files.IsDirectory(dev) {
		directory, version = dev, "dev"
	} else {
		if want != "" && lang.Pinned(want) && files.IsDirectory(filepath.Join(libraryDirectory, strings.ReplaceAll(want, ".", ","))) {
			version = want
		} else {
			version = readTrim(files, filepath.Join(libraryDirectory, ".current"))
		}
		if version == "" || strings.ContainsAny(version, `/\`) || strings.Contains(version, "..") {
			return nil
		}
		directory = filepath.Join(libraryDirectory, strings.ReplaceAll(version, ".", ","))
	}
	if !files.IsDirectory(directory) {
		return nil
	}
	in := &installed{name: name, version: version, classPaths: []string{directory}, files: files}
	if data, ok := files.ReadBounded(filepath.Join(directory, "haxelib.json")); ok {
		if h, dependencies, ok := readHaxelib(data); ok {
			if h.Name != "" {
				in.name = h.Name
			}
			in.classPaths = []string{filepath.Join(directory, filepath.FromSlash(h.ClassPath))}
			in.dependencies = dependencies
		}
	}
	sort.SliceStable(in.dependencies, func(i, j int) bool { return in.dependencies[i].line < in.dependencies[j].line })
	return in
}

func readTrim(files lang.Root, p string) string {
	data, ok := files.ReadLimited(p, 4096)
	if !ok {
		return ""
	}
	return strings.TrimSpace(string(data))
}

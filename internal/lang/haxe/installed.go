package haxe

import (
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
	dir   string
	libs  map[string]*lixLib // lower-case name -> pin
	names []string           // the keys, sorted
}

// lixLib is one haxe_libraries/<name>.hxml.
type lixLib struct {
	name, version    string
	pinned, floating bool
	origin           string
	local            string   // a class path in the repository: the library is the repository
	abs              []string // class paths in lix's cache on this machine
	deps             []string // its -lib lines
}

func (l *lixLib) target() lang.Target {
	if l.local != "" {
		return lang.Target{Local: l.local}
	}
	return lang.Target{Ecosystem: ecoHaxelib, Package: l.name, Version: l.version, Pinned: l.pinned,
		Floating: l.floating, Origin: l.origin}
}

// lixScope is the scope governing a file: the nearest directory at or above it
// with a haxe_libraries/ directory.
func (r *resolver) lixScope(file string) *lixScope {
	if len(r.lix) == 0 {
		return nil
	}
	for d := path.Dir(file); ; d = path.Dir(d) {
		if s := r.lix[d]; s != nil {
			return s
		}
		if d == "." || d == "/" {
			return nil
		}
	}
}

// libCache is lix's download cache: HAXE_LIBCACHE, else haxe_libraries/ under
// HAXESHIM_ROOT or ~/haxe.
func libCache(getenv func(string) string) string {
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
	cache := libCache(getenv)
	shim := getenv("HAXESHIM_ROOT")
	if shim == "" && getenv("HOME") != "" {
		shim = filepath.Join(getenv("HOME"), "haxe")
	}
	for _, f := range all {
		if class(f.Path) != classLix || f.Binary || f.TooLarge || f.Size > lang.MaxParseSize {
			continue
		}
		data, err := os.ReadFile(f.Abs)
		if err != nil {
			continue
		}
		scope := path.Dir(path.Dir(f.Path))
		name := strings.TrimSuffix(path.Base(f.Path), ".hxml")
		h := readHXML(data)
		l := &lixLib{name: name, deps: nil}
		for _, d := range h.libs {
			l.deps = append(l.deps, d.name)
		}
		readInstall(l, h.install, h.defs[name])
		for _, cp := range h.cps {
			switch {
			case strings.HasPrefix(cp, "${SCOPE_DIR}"):
				if rel := path.Join(scope, strings.TrimPrefix(cp, "${SCOPE_DIR}")); inside(rel) && r.dirs[rel] && l.local == "" {
					l.local = rel
				}
			case strings.HasPrefix(cp, "${HAXE_LIBCACHE}"), strings.HasPrefix(cp, "${HAXESHIM_LIBCACHE}"):
				if _, rest, _ := strings.Cut(cp, "}"); cache != "" {
					l.abs = append(l.abs, filepath.Join(cache, rest))
				}
			case strings.HasPrefix(cp, "${HAXESHIM_ROOT}"):
				if shim != "" {
					l.abs = append(l.abs, filepath.Join(shim, strings.TrimPrefix(cp, "${HAXESHIM_ROOT}")))
				}
			case strings.Contains(cp, "$"):
			case filepath.IsAbs(cp):
				l.abs = append(l.abs, cp)
			default:
				if rel := path.Join(scope, cp); inside(rel) && r.dirs[rel] && l.local == "" {
					l.local = rel
				}
			}
		}
		s := r.lix[scope]
		if s == nil {
			s = &lixScope{dir: scope, libs: map[string]*lixLib{}}
			r.lix[scope] = s
		}
		s.libs[strings.ToLower(name)] = l
		r.lixFile[f.Path] = l
	}
	for _, s := range r.lix {
		s.names = sortedKeys(s.libs)
	}
}

// readInstall reads lix's install URL: haxelib:/name#1.2.3 pins that version,
// a git reference (gh://, gl://, git:) pins a commit, shows a tag and floats
// on a branch; no URL is a library in development (it floats), and a
// download URL shows the version the file defines.
func readInstall(l *lixLib, url, defined string) {
	switch {
	case url == "":
		l.version, l.floating = defined, true
	case strings.HasPrefix(url, "haxelib:"):
		_, v, _ := strings.Cut(strings.TrimLeft(strings.TrimPrefix(url, "haxelib:"), "/"), "#")
		if v == "" {
			v = defined
		}
		l.version, l.pinned, l.floating = v, v != "", v == ""
	default:
		u, ref, _ := strings.Cut(url, "#")
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
		if !public(u) {
			l.origin = u
		}
		switch {
		case lang.Commit(ref):
			l.version, l.pinned = ref, true
		case ref != "" && tagLike(ref):
			l.version = ref
		case ref != "":
			l.version, l.floating = ref, true
		default:
			l.version = defined // an archive download
		}
	}
}

// installed is a library as haxelib installed it.
type installed struct {
	name, version string
	cps           []string // absolute class paths
	deps          []dep
	local         bool // in the repository's own .haxelib/ repository
}

// repositories are haxelib's repositories: a local one (.haxelib/) at the
// repository's root or beside a manifest, then the global one: HAXELIB_PATH,
// else the path in ~/.haxelib, else ~/haxelib.
func (r *resolver) repositories(root string, getenv func(string) string) (local []string, global string) {
	if root != "" {
		dirs := []string{"."}
		for _, m := range r.all {
			dirs = append(dirs, path.Dir(m.file))
		}
		seen := map[string]bool{}
		for _, d := range dirs {
			abs := filepath.Join(root, filepath.FromSlash(d), ".haxelib")
			if !seen[abs] && isDir(abs) {
				seen[abs] = true
				local = append(local, abs)
			}
		}
	}
	if p := getenv("HAXELIB_PATH"); p != "" {
		return local, p
	}
	if h := getenv("HOME"); h != "" {
		if data, err := os.ReadFile(filepath.Join(h, ".haxelib")); err == nil {
			if p := strings.TrimSpace(string(data)); p != "" && isDir(p) {
				return local, p
			}
		}
		if p := filepath.Join(h, "haxelib"); isDir(p) {
			return local, p
		}
	}
	return local, ""
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
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
		for _, d := range m.libs {
			k := strings.ToLower(d.name)
			if _, ok := wanted[k]; !ok {
				names = append(names, d.name)
				wanted[k] = d.version
			} else if wanted[k] == "" {
				wanted[k] = d.version
			}
		}
	}
	for _, repo := range local {
		entries, _ := os.ReadDir(repo)
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := strings.ReplaceAll(e.Name(), ",", ".")
			k := strings.ToLower(name)
			if r.installed[k] != nil {
				continue
			}
			if in := installedLib(filepath.Join(repo, e.Name()), name, wanted[k]); in != nil {
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
		dir := filepath.Join(global, strings.ReplaceAll(name, ".", ","))
		if !isDir(dir) {
			// Names are case-insensitive in haxelib: look the directory up.
			if !listed {
				listing, _ = os.ReadDir(global)
				listed = true
			}
			dir = ""
			for _, e := range listing {
				if e.IsDir() && strings.EqualFold(strings.ReplaceAll(e.Name(), ",", "."), name) {
					dir = filepath.Join(global, e.Name())
					break
				}
			}
			if dir == "" {
				continue
			}
		}
		if in := installedLib(dir, name, wanted[k]); in != nil {
			r.installed[k] = in
		}
	}
}

// installedLib reads a library's directory in a haxelib repository: the
// development path in .dev, else the declared version when it is installed,
// else the version .current names (1.2.3 in 1,2,3/, git in git/).
func installedLib(libDir, name, want string) *installed {
	dir, version := "", ""
	if dev := readTrim(filepath.Join(libDir, ".dev")); dev != "" && isDir(dev) {
		dir, version = dev, "dev"
	} else {
		if want != "" && lang.Pinned(want) && isDir(filepath.Join(libDir, strings.ReplaceAll(want, ".", ","))) {
			version = want
		} else {
			version = readTrim(filepath.Join(libDir, ".current"))
		}
		if version == "" || strings.ContainsAny(version, `/\`) || strings.Contains(version, "..") {
			return nil
		}
		dir = filepath.Join(libDir, strings.ReplaceAll(version, ".", ","))
	}
	if !isDir(dir) {
		return nil
	}
	in := &installed{name: name, version: version, cps: []string{dir}}
	if data, err := os.ReadFile(filepath.Join(dir, "haxelib.json")); err == nil {
		if h, deps, ok := readHaxelib(data); ok {
			if h.Name != "" {
				in.name = h.Name
			}
			in.cps = []string{filepath.Join(dir, filepath.FromSlash(h.ClassPath))}
			in.deps = deps
		}
	}
	sort.SliceStable(in.deps, func(i, j int) bool { return in.deps[i].line < in.deps[j].line })
	return in
}

func readTrim(p string) string {
	data, err := os.ReadFile(p)
	if err != nil || len(data) > 4096 {
		return ""
	}
	return strings.TrimSpace(string(data))
}

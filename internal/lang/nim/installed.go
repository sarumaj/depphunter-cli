package nim

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// installed is a package nimble or Atlas installed: where its modules are and
// what its .nimble file requires.
type installed struct {
	name    string
	version string
	url     string // an Atlas checkout's origin
	root    string // absolute directory its modules are found under
	deps    []dep
	modules map[string]bool // module paths under root: "chronos", "chronos/asyncloop"
}

// maxModules bounds the files indexed per installed package.
const maxModules = 5000

// pkgDir parses the name of a directory of nimble's pkgs2/ (name-version-
// checksum) or pkgs/ (name-version) directory.
func pkgDir(base string, pkgs2 bool) (name, version string, ok bool) {
	s := base
	if pkgs2 {
		i := strings.LastIndexByte(s, '-')
		if i <= 0 {
			return "", "", false
		}
		s = s[:i]
	}
	i := strings.LastIndexByte(s, '-')
	if i <= 0 || i == len(s)-1 {
		return "", "", false
	}
	return s[:i], s[i+1:], true
}

// readInstalled reads the package installed at dir: its .nimble file (name,
// srcDir, requirements) and the modules under its source directory. local is
// true for a checkout or a nimbledeps/ install, which keep srcDir; a global
// install has srcDir's contents at its root.
//
// Implements: REQ-NIM-007
func readInstalled(dir, name, version string, local bool) *installed {
	p := &installed{name: name, version: version, root: dir, modules: map[string]bool{}}
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if n := e.Name(); strings.HasSuffix(n, ".nimble") && !e.IsDir() {
				if data, err := os.ReadFile(filepath.Join(dir, n)); err == nil && len(data) <= 1<<20 {
					nf := readNimble(data, strings.TrimSuffix(n, ".nimble"))
					if p.name == "" {
						p.name = nf.name
					}
					if p.version == "" {
						p.version = nf.version
					}
					p.deps = nf.deps
					if local && nf.srcDir != "" {
						if st, err := os.Stat(filepath.Join(dir, filepath.FromSlash(nf.srcDir))); err == nil && st.IsDir() {
							p.root = filepath.Join(dir, filepath.FromSlash(nf.srcDir))
						}
					}
				}
				break
			}
		}
	}
	if p.name == "" {
		return nil
	}
	p.index()
	return p
}

// index lists the modules under the package's root.
func (p *installed) index() {
	count := 0
	filepath.WalkDir(p.root, func(f string, d os.DirEntry, err error) error {
		if err != nil || count >= maxModules {
			if d != nil && d.IsDir() && f != p.root {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if n := d.Name(); f != p.root && (strings.HasPrefix(n, ".") || n == "nimcache" || n == "nimbledeps") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(f, ".nim") {
			if rel, err := filepath.Rel(p.root, f); err == nil {
				p.modules[strings.TrimSuffix(filepath.ToSlash(rel), ".nim")] = true
				count++
			}
		}
		return nil
	})
}

// readPkgs reads the packages installed in a nimble directory's pkgs2/ and
// pkgs/ (nimbledeps/, or ~/.nimble). want, when not nil, limits them to the
// names it holds (folded); of several versions of one package the newest is
// read, unless prefer names one.
func readPkgs(nimbleDir string, local bool, want map[string]bool, prefer map[string]string) []*installed {
	type candidate struct {
		dir, name, version string
	}
	best := map[string]candidate{}
	var order []string
	for _, sub := range []string{"pkgs2", "pkgs"} {
		entries, err := os.ReadDir(filepath.Join(nimbleDir, sub))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name, version, ok := pkgDir(e.Name(), sub == "pkgs2")
			if !ok || want != nil && !want[fold(name)] {
				continue
			}
			c := candidate{filepath.Join(nimbleDir, sub, e.Name()), name, version}
			key := fold(name)
			old, seen := best[key]
			switch {
			case !seen:
				order = append(order, key)
				best[key] = c
			case prefer[key] != "" && c.version == prefer[key] && old.version != prefer[key]:
				best[key] = c
			case (prefer[key] == "" || old.version != prefer[key]) && newer(c.version, old.version):
				best[key] = c
			}
		}
	}
	sort.Strings(order)
	var out []*installed
	for _, k := range order {
		c := best[k]
		if p := readInstalled(c.dir, c.name, c.version, local); p != nil {
			out = append(out, p)
		}
	}
	return out
}

// newer compares two versions numerically, dot by dot; #head and other special
// versions are older than any number.
func newer(a, b string) bool {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var x, y int
		okx, oky := false, false
		if i < len(as) {
			v, err := strconv.Atoi(as[i])
			x, okx = v, err == nil
		}
		if i < len(bs) {
			v, err := strconv.Atoi(bs[i])
			y, oky = v, err == nil
		}
		if okx != oky {
			return okx
		}
		if x != y {
			return x > y
		}
	}
	return false
}

var originURL = regexp.MustCompile(`(?m)^\s*url\s*=\s*(\S+)\s*$`)

// readAtlas reads the checkouts of an Atlas deps directory: each directory with
// a .nimble file is a package, its origin from .git/config.
func readAtlas(deps string) []*installed {
	entries, err := os.ReadDir(deps)
	if err != nil {
		return nil
	}
	var out []*installed
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") || e.Name() == "_nimbles" || e.Name() == "_packages" {
			continue
		}
		dir := filepath.Join(deps, e.Name())
		p := readInstalled(dir, "", "", true)
		if p == nil {
			continue
		}
		if cfg, err := os.ReadFile(filepath.Join(dir, ".git", "config")); err == nil {
			if m := originURL.FindSubmatch(cfg); m != nil {
				p.url = string(m[1])
			}
		}
		out = append(out, p)
		if len(out) >= 512 {
			break
		}
	}
	return out
}

// atlasDepsDir is the deps directory of an Atlas project in dir: atlas.config's
// "deps" (in dir, else deps/atlas.config), else deps/ when it exists.
func atlasDepsDir(dir string) string {
	for _, cfg := range []string{filepath.Join(dir, "atlas.config"), filepath.Join(dir, "deps", "atlas.config")} {
		data, err := os.ReadFile(cfg)
		if err != nil {
			continue
		}
		if m := regexp.MustCompile(`"deps"\s*:\s*"([^"]*)"`).FindSubmatch(data); m != nil && len(m[1]) > 0 {
			d := filepath.FromSlash(string(m[1]))
			if !filepath.IsAbs(d) {
				d = filepath.Join(dir, d)
			}
			return d
		}
		return filepath.Join(dir, "deps")
	}
	if st, err := os.Stat(filepath.Join(dir, "deps")); err == nil && st.IsDir() {
		return filepath.Join(dir, "deps")
	}
	return ""
}

// fromPaths reads the packages the absolute search paths of a nimble.paths
// file name: a path under a pkgs2/ or pkgs/ directory is that package's
// source directory.
func fromPaths(paths []pathSwitch) []*installed {
	var out []*installed
	for _, p := range paths {
		v := filepath.Clean(filepath.FromSlash(strings.Trim(p.value, "\"")))
		if !filepath.IsAbs(v) {
			continue
		}
		segments := strings.Split(filepath.ToSlash(v), "/")
		for i := len(segments) - 2; i >= 0; i-- {
			if segments[i] != "pkgs2" && segments[i] != "pkgs" {
				continue
			}
			name, version, ok := pkgDir(segments[i+1], segments[i] == "pkgs2")
			if !ok {
				break
			}
			ip := readInstalled(filepath.FromSlash(strings.Join(segments[:i+2], "/")), name, version, false)
			if ip != nil {
				ip.root = v
				ip.modules = map[string]bool{}
				ip.index()
				out = append(out, ip)
			}
			break
		}
	}
	return out
}

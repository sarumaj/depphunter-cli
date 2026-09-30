package nim

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// installed is a package nimble or Atlas installed: where its modules are and
// what its .nimble file requires.
type installed struct {
	name         string
	version      string
	url          string // an Atlas checkout's origin
	root         string // absolute directory its modules are found under
	dependencies []dependency
	modules      map[string]bool // module paths under root: "chronos", "chronos/asyncloop"
}

// maxModules bounds the files indexed per installed package.
const maxModules = 5000

// packageDirectory parses the name of a directory of nimble's pkgs2/ (name-version-
// checksum) or pkgs/ (name-version) directory.
func packageDirectory(base string, pkgs2 bool) (name, version string, ok bool) {
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

// readInstalled reads the package installed at directory: its .nimble file (name,
// srcDir, requirements) and the modules under its source directory. local is
// true for a checkout or a nimbledeps/ install, which keep srcDir; a global
// install has srcDir's contents at its root.
//
// Implements: REQ-NIM-007
func readInstalled(directory, name, version string, local bool) *installed {
	p := &installed{name: name, version: version, root: directory, modules: map[string]bool{}}
	if entries, err := os.ReadDir(directory); err == nil {
		for _, e := range entries {
			if n := e.Name(); strings.HasSuffix(n, ".nimble") && !e.IsDir() {
				if data, ok := lang.ReadCapped(filepath.Join(directory, n)); ok {
					nf := readNimble(data, strings.TrimSuffix(n, ".nimble"))
					if p.name == "" {
						p.name = nf.name
					}
					if p.version == "" {
						p.version = nf.version
					}
					p.dependencies = nf.dependencies
					if local && nf.sourceDirectory != "" {
						if fileInfo, err := os.Stat(filepath.Join(directory, filepath.FromSlash(nf.sourceDirectory))); err == nil && fileInfo.IsDir() {
							p.root = filepath.Join(directory, filepath.FromSlash(nf.sourceDirectory))
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
			if relative, err := filepath.Rel(p.root, f); err == nil {
				p.modules[strings.TrimSuffix(filepath.ToSlash(relative), ".nim")] = true
				count++
			}
		}
		return nil
	})
}

// readPackages reads the packages installed in a nimble directory's pkgs2/ and
// pkgs/ (nimbledeps/, or ~/.nimble). want, when not nil, limits them to the
// names it holds (folded); of several versions of one package the newest is
// read, unless prefer names one.
func readPackages(nimbleDirectory string, local bool, want map[string]bool, prefer map[string]string) []*installed {
	type candidate struct {
		directory, name, version string
	}
	best := map[string]candidate{}
	var order []string
	for _, packagesDirectory := range []string{"pkgs2", "pkgs"} {
		entries, err := os.ReadDir(filepath.Join(nimbleDirectory, packagesDirectory))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name, version, ok := packageDirectory(e.Name(), packagesDirectory == "pkgs2")
			if !ok || want != nil && !want[fold(name)] {
				continue
			}
			c := candidate{filepath.Join(nimbleDirectory, packagesDirectory, e.Name()), name, version}
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
		if p := readInstalled(c.directory, c.name, c.version, local); p != nil {
			out = append(out, p)
		}
	}
	return out
}

// newer compares two versions numerically, dot by dot; #head and other special
// versions are older than any number.
func newer(a, b string) bool {
	aParts, bParts := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(aParts) || i < len(bParts); i++ {
		var x, y int
		okx, oky := false, false
		if i < len(aParts) {
			v, err := strconv.Atoi(aParts[i])
			x, okx = v, err == nil
		}
		if i < len(bParts) {
			v, err := strconv.Atoi(bParts[i])
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

// readAtlas reads the checkouts of an Atlas dependencies directory: each directory with
// a .nimble file is a package, its origin from .git/config.
func readAtlas(dependencies string) []*installed {
	entries, err := os.ReadDir(dependencies)
	if err != nil {
		return nil
	}
	var out []*installed
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") || e.Name() == "_nimbles" || e.Name() == "_packages" {
			continue
		}
		directory := filepath.Join(dependencies, e.Name())
		p := readInstalled(directory, "", "", true)
		if p == nil {
			continue
		}
		if config, err := os.ReadFile(filepath.Join(directory, ".git", "config")); err == nil {
			if m := originURL.FindSubmatch(config); m != nil {
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

// atlasDependenciesDirectory is the dependencies directory of an Atlas project in directory: atlas.config's
// "deps" (in directory, else deps/atlas.config), else deps/ when it exists.
func atlasDependenciesDirectory(directory string) string {
	for _, config := range []string{filepath.Join(directory, "atlas.config"), filepath.Join(directory, "deps", "atlas.config")} {
		data, err := os.ReadFile(config)
		if err != nil {
			continue
		}
		if m := regexp.MustCompile(`"deps"\s*:\s*"([^"]*)"`).FindSubmatch(data); m != nil && len(m[1]) > 0 {
			d := filepath.FromSlash(string(m[1]))
			if !filepath.IsAbs(d) {
				d = filepath.Join(directory, d)
			}
			return d
		}
		return filepath.Join(directory, "deps")
	}
	if fileInfo, err := os.Stat(filepath.Join(directory, "deps")); err == nil && fileInfo.IsDir() {
		return filepath.Join(directory, "deps")
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
			name, version, ok := packageDirectory(segments[i+1], segments[i] == "pkgs2")
			if !ok {
				break
			}
			installedPackage := readInstalled(filepath.FromSlash(strings.Join(segments[:i+2], "/")), name, version, false)
			if installedPackage != nil {
				installedPackage.root = v
				installedPackage.modules = map[string]bool{}
				installedPackage.index()
				out = append(out, installedPackage)
			}
			break
		}
	}
	return out
}

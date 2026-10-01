// Package locate finds the directory a dependency is installed in on this machine:
// a node_modules or vendor directory of the project's, a virtual environment's
// site-packages, or a package manager's shared cache.
//
// It looks where each package manager puts what it installs and takes the first
// directory that exists. Nothing is run and nothing is fetched, so a package that
// was never installed here - or was installed somewhere unusual - has no folder.
package locate

import (
	"bufio"
	"cmp"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/mod/module"

	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// Package is what Folder needs to know of a package.
type Package struct {
	Ecosystem, Name, Version string
	// Origin is where it was installed from (graph.Node.Origin).
	Origin string
	// Near are the directories, relative to the root, of the files that use it:
	// the project directories whose own installs are looked in first.
	Near []string
}

// Folder is the directory p is installed in, or "" when none is found.
//
// Implements: REQ-SRV-018
func Folder(root string, machine userconf.Machine, p Package) string {
	if folder := local(root, p.Origin, machine.GOOS); folder != "" {
		return folder
	}
	if !confined(p.Name) {
		return ""
	}
	projects := above(root, p.Near)
	name := filepath.FromSlash(p.Name)
	switch p.Ecosystem {
	case "npm":
		return inProjects(projects, "node_modules", name)
	case "composer":
		return inProjects(projects, "vendor", name)
	case "hex":
		return inProjects(projects, "deps", name)
	case "go":
		return cmp.Or(inProjects(projects, "vendor", name), goModule(machine, p.Name, p.Version))
	case "crates":
		return cmp.Or(inProjects(projects, "vendor", name), crate(machine, p.Name, p.Version))
	case "pypi":
		return python(machine, projects, p.Name)
	case "maven":
		return maven(machine, p.Name, p.Version)
	case "nuget":
		return nuget(machine, p.Name, p.Version)
	case "rubygems":
		return gem(machine, projects, p.Name, p.Version)
	case "pub":
		return pub(machine, p.Name, p.Version)
	}
	return ""
}

// confined reports whether a name, joined to a directory, stays inside it.
func confined(name string) bool {
	segments := strings.FieldsFunc(name, func(r rune) bool { return r == '/' || r == '\\' })
	return name != "" && !filepath.IsAbs(name) && !strings.HasPrefix(name, "/") && !slices.Contains(segments, "..")
}

// local is the directory a package installed from a directory was installed from:
// file:///src/lib, path:../lib, ./lib.
func local(root, origin, goos string) string {
	origin = strings.TrimPrefix(origin, "path:")
	if origin == "" {
		return ""
	}
	if u, err := url.Parse(origin); err == nil && u.Scheme == "file" {
		origin = u.Path
		if goos == "windows" {
			origin = strings.TrimPrefix(origin, "/")
		}
	} else if strings.Contains(origin, "://") || strings.Contains(origin, "@") {
		return ""
	}
	origin = filepath.FromSlash(origin)
	if !filepath.IsAbs(origin) {
		origin = filepath.Join(root, origin)
	}
	return directory(origin)
}

// above lists the directories from each of near up to the root, nearest first and
// each once; the root is always among them.
func above(root string, near []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, relative := range append(near, ".") {
		for d := filepath.Join(root, filepath.FromSlash(relative)); ; d = filepath.Dir(d) {
			if within, err := filepath.Rel(root, d); err != nil || strings.HasPrefix(within, "..") {
				break
			}
			if !seen[d] {
				seen[d] = true
				out = append(out, d)
			}
			if d == root {
				break
			}
		}
	}
	return out
}

func inProjects(projects []string, elements ...string) string {
	for _, p := range projects {
		if d := directory(filepath.Join(append([]string{p}, elements...)...)); d != "" {
			return d
		}
	}
	return ""
}

// directory is path when it is a directory, else "".
func directory(path string) string {
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return path
	}
	return ""
}

// goModule is a module's directory in the module cache: GOMODCACHE, else the first
// GOPATH's pkg/mod, as the go command decides it.
func goModule(machine userconf.Machine, path, version string) string {
	escapedPath, err := module.EscapePath(path)
	if err != nil || version == "" {
		return ""
	}
	escapedVersion, err := module.EscapeVersion(version)
	if err != nil {
		return ""
	}
	cache := machine.GoEnvironment("GOMODCACHE")
	if cache == "" {
		gopath := filepath.SplitList(machine.GoEnvironment("GOPATH"))
		if len(gopath) == 0 || gopath[0] == "" {
			gopath = []string{filepath.Join(machine.Home, "go")}
		}
		cache = filepath.Join(gopath[0], "pkg", "mod")
	}
	return directory(filepath.Join(cache, filepath.FromSlash(escapedPath)+"@"+escapedVersion))
}

// crate is a crate's sources as Cargo unpacked them, under any registry's directory.
func crate(machine userconf.Machine, name, version string) string {
	if version == "" {
		return ""
	}
	return first(filepath.Join(machine.CargoHome(), "registry", "src", "*", name+"-"+version))
}

// first is the first directory matching a glob pattern, or "".
func first(pattern string) string {
	matches, _ := filepath.Glob(pattern)
	for _, m := range matches {
		if d := directory(m); d != "" {
			return d
		}
	}
	return ""
}

// environments are the virtual environments a project keeps beside its sources.
var environments = []string{".venv", "venv", "env"}

// sitePackages are where an environment installs, POSIX's and Windows'.
var sitePackages = []string{filepath.Join("lib", "python*", "site-packages"), filepath.Join("Lib", "site-packages")}

// python is the directory a distribution installed into a virtual environment - the
// activated one, else the project's own - put its code in: the top-level package
// its top_level.txt names, else its .dist-info directory.
func python(machine userconf.Machine, projects []string, name string) string {
	var prefixes []string
	if active := machine.Environment("VIRTUAL_ENV"); active != "" {
		prefixes = append(prefixes, active)
	}
	for _, p := range projects {
		for _, e := range environments {
			prefixes = append(prefixes, filepath.Join(p, e))
		}
	}
	normalized := normalize(name)
	for _, prefix := range prefixes {
		for _, site := range sitePackages {
			sites, _ := filepath.Glob(filepath.Join(prefix, site))
			for _, s := range sites {
				infos, _ := filepath.Glob(filepath.Join(s, "*.dist-info"))
				for _, info := range infos {
					distribution, _, _ := strings.Cut(strings.TrimSuffix(filepath.Base(info), ".dist-info"), "-")
					if normalize(distribution) != normalized {
						continue
					}
					if top := topLevel(info); top != "" {
						if d := directory(filepath.Join(s, top)); d != "" {
							return d
						}
					}
					return info
				}
			}
		}
	}
	return ""
}

var separators = regexp.MustCompile(`[-_.]+`)

// normalize is a distribution name as PEP 503 compares them.
func normalize(name string) string { return strings.ToLower(separators.ReplaceAllString(name, "-")) }

// topLevel is the first package a .dist-info's top_level.txt names.
func topLevel(info string) string {
	f, err := os.Open(filepath.Join(info, "top_level.txt"))
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" && !strings.ContainsAny(line, `/\.`) {
			return line
		}
	}
	return ""
}

// maven is an artifact's version directory in the local Maven repository, else in
// Gradle's cache.
func maven(machine userconf.Machine, name, version string) string {
	group, artifact, ok := strings.Cut(name, ":")
	if !ok || version == "" || strings.ContainsAny(name, `/\`) {
		return ""
	}
	return cmp.Or(
		directory(filepath.Join(machine.Home, ".m2", "repository", filepath.Join(strings.Split(group, ".")...), artifact, version)),
		directory(filepath.Join(machine.GradleUserHome(), "caches", "modules-2", "files-2.1", group, artifact, version)))
}

// nuget is a package's directory in the global packages folder, where NuGet keeps
// both its id and its version in lower case.
func nuget(machine userconf.Machine, name, version string) string {
	if version == "" {
		return ""
	}
	packages := cmp.Or(machine.Environment("NUGET_PACKAGES"), filepath.Join(machine.Home, ".nuget", "packages"))
	return directory(filepath.Join(packages, strings.ToLower(name), strings.ToLower(version)))
}

// gem is a gem's directory where Bundler installed it into the project
// (vendor/bundle), else under GEM_HOME.
func gem(machine userconf.Machine, projects []string, name, version string) string {
	if version == "" {
		return ""
	}
	for _, p := range projects {
		if d := first(filepath.Join(p, "vendor", "bundle", "ruby", "*", "gems", name+"-"+version)); d != "" {
			return d
		}
	}
	if home := machine.Environment("GEM_HOME"); home != "" {
		return directory(filepath.Join(home, "gems", name+"-"+version))
	}
	return ""
}

// pub is a package's directory in the pub cache.
func pub(machine userconf.Machine, name, version string) string {
	if version == "" {
		return ""
	}
	cache := machine.Environment("PUB_CACHE")
	if cache == "" && machine.GOOS == "windows" {
		cache = filepath.Join(machine.Environment("LOCALAPPDATA"), "Pub", "Cache")
	} else if cache == "" {
		cache = filepath.Join(machine.Home, ".pub-cache")
	}
	return directory(filepath.Join(cache, "hosted", "pub.dev", name+"-"+version))
}

package php

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// A Composer project is a composer.json: what it requires, and how its own classes
// are found (autoload, autoload-dev). composer.lock beside it records every package
// installed - the direct ones and theirs - with its exact version, what it requires,
// and its own autoload rules, which is what turns a namespace into a package: the
// lock says symfony/http-foundation autoloads Symfony\Component\HttpFoundation\. The
// vendor directory is not scanned (internal/scan ignores it), so without a lock the
// same records are read from vendor/composer/installed.json on disk.

// mapping is one autoload rule: classes whose name starts with prefix live under
// dirs (project-relative). psr0 marks PSR-0, where the whole class name is the path.
type mapping struct {
	prefix      string
	directories []string
	psr0        bool
}

// composerPackage is one package composer.lock or installed.json records.
type composerPackage struct {
	name, version string
	require       map[string]string
	prefixes      []string // the namespaces (PSR-4, PSR-0) it autoloads
	installed     bool     // known from vendor/composer/installed.json, not a lock
	git           string   // "<repository URL>#<commit>" of a branch checkout (dev-main)
}

type project struct {
	directory string
	require   map[string]string // require and require-dev, platform packages left out
	locked    map[string]*composerPackage
	byPrefix  []prefixed // every locked package's namespaces, longest first
}

type prefixed struct {
	prefix          string
	composerPackage *composerPackage
}

// autoload is composer.json's autoload section, and the same section of a package
// in a lock. A PSR-4 or PSR-0 value is a directory or a list of them.
type autoload struct {
	PSR4 map[string]json.RawMessage `json:"psr-4"`
	PSR0 map[string]json.RawMessage `json:"psr-0"`
}

// lockedPackage is a package entry of composer.lock or installed.json.
type lockedPackage struct {
	Name     string                     `json:"name"`
	Version  string                     `json:"version"`
	Require  map[string]json.RawMessage `json:"require"`
	Autoload autoload                   `json:"autoload"`
	Dist     struct {
		Type string `json:"type"`
		URL  string `json:"url"`
	} `json:"dist"`
	Source struct {
		Type      string `json:"type"`
		URL       string `json:"url"`
		Reference string `json:"reference"`
	} `json:"source"`
}

// stringList reads a JSON string or list of strings.
func stringList(raw json.RawMessage) []string {
	var one string
	if json.Unmarshal(raw, &one) == nil {
		return []string{one}
	}
	var many []string
	_ = json.Unmarshal(raw, &many)
	return many
}

// requirements reads a require section leniently: a value that is not a string (a
// broken manifest) is left out rather than failing the whole file.
func requirements(m map[string]json.RawMessage) map[string]string {
	out := map[string]string{}
	for name, raw := range m {
		var v string
		if json.Unmarshal(raw, &v) == nil && !platform(name) {
			out[strings.ToLower(name)] = strings.TrimSpace(v)
		}
	}
	return out
}

// platform reports whether a requirement is the platform rather than a package: php,
// ext-*, lib-*, composer-plugin-api and the like. Every Composer package is named
// vendor/name.
//
// Implements: REQ-PHP-007
func platform(name string) bool { return !strings.Contains(name, "/") }

// readProject reads one composer.json and what was installed for it. It returns the
// project and its local autoload rules; rules of a path-repository package inside the
// project (a monorepo's packages/*) count as local too.
//
// Implements: REQ-PHP-005, REQ-PHP-007, REQ-PHP-008
func readProject(root, relative, absolute string) (*project, []mapping) {
	data, err := os.ReadFile(absolute)
	if err != nil {
		return nil, nil
	}
	var doc struct {
		Require     map[string]json.RawMessage `json:"require"`
		RequireDev  map[string]json.RawMessage `json:"require-dev"`
		Autoload    autoload                   `json:"autoload"`
		AutoloadDev autoload                   `json:"autoload-dev"`
	}
	if json.Unmarshal(data, &doc) != nil {
		return nil, nil
	}
	directory := path.Dir(relative)
	p := &project{directory: directory, require: requirements(doc.Require), locked: map[string]*composerPackage{}}
	for name, v := range requirements(doc.RequireDev) {
		if _, ok := p.require[name]; !ok {
			p.require[name] = v
		}
	}
	local := append(mappings(directory, doc.Autoload), mappings(directory, doc.AutoloadDev)...)

	packages, installed := readLock(lang.OpenRoot(root), filepath.Join(filepath.Dir(absolute), "composer.lock"))
	if packages == nil {
		packages, installed = readInstalled(root, directory), true
	}
	for _, lockedPackage := range packages {
		name := strings.ToLower(lockedPackage.Name)
		if name == "" {
			continue
		}
		if lockedPackage.Dist.Type == "path" && lockedPackage.Dist.URL != "" && !path.IsAbs(lockedPackage.Dist.URL) {
			// A path repository: the package's code is in this repository.
			if packageDirectory := path.Join(directory, filepath.ToSlash(lockedPackage.Dist.URL)); packageDirectory != ".." && !strings.HasPrefix(packageDirectory, "../") {
				local = append(local, mappings(packageDirectory, lockedPackage.Autoload)...)
				continue
			}
		}
		k := &composerPackage{name: name, version: lockedPackage.Version, require: requirements(lockedPackage.Require), installed: installed}
		// A branch's version names no release, so the commit it was locked at is the
		// only thing the vulnerability database could be asked about.
		// Implements: REQ-FND-026
		if v := strings.ToLower(lockedPackage.Version); (strings.HasPrefix(v, "dev-") || strings.HasSuffix(v, "-dev")) &&
			lockedPackage.Source.Type == "git" && lockedPackage.Source.URL != "" && lang.Commit(lockedPackage.Source.Reference) {
			k.git = lockedPackage.Source.URL + "#" + lockedPackage.Source.Reference
		}
		for prefix := range lockedPackage.Autoload.PSR4 {
			k.prefixes = append(k.prefixes, prefix)
		}
		for prefix := range lockedPackage.Autoload.PSR0 {
			k.prefixes = append(k.prefixes, prefix)
		}
		p.locked[name] = k
		for _, prefix := range k.prefixes {
			if prefix != "" {
				p.byPrefix = append(p.byPrefix, prefixed{prefix, k})
			}
		}
	}
	sort.Slice(p.byPrefix, func(i, j int) bool {
		a, b := p.byPrefix[i], p.byPrefix[j]
		if len(a.prefix) != len(b.prefix) {
			return len(a.prefix) > len(b.prefix)
		}
		return a.prefix+a.composerPackage.name < b.prefix+b.composerPackage.name
	})
	return p, local
}

// mappings reads an autoload section's PSR-4 and PSR-0 rules, directories made
// project-relative. classmap and files are not rules a name can be matched to; the
// classes they hold are found by what the files declare (resolve.go).
func mappings(directory string, a autoload) []mapping {
	var out []mapping
	add := func(rules map[string]json.RawMessage, psr0 bool) {
		for prefix, raw := range rules {
			m := mapping{prefix: prefix, psr0: psr0}
			for _, d := range stringList(raw) {
				m.directories = append(m.directories, path.Join(directory, filepath.ToSlash(d)))
			}
			if len(m.directories) > 0 {
				out = append(out, m)
			}
		}
	}
	add(a.PSR4, false)
	add(a.PSR0, true)
	return out
}

// readLock reads composer.lock's packages and packages-dev; nil when there is none.
//
// Implements: REQ-PHP-008
func readLock(repository lang.Root, absolute string) ([]lockedPackage, bool) {
	data, ok := repository.ReadBounded(absolute)
	if !ok {
		return nil, false
	}
	var lock struct {
		Packages    []lockedPackage `json:"packages"`
		PackagesDev []lockedPackage `json:"packages-dev"`
	}
	if json.Unmarshal(data, &lock) != nil {
		return nil, false
	}
	return append(append([]lockedPackage{}, lock.Packages...), lock.PackagesDev...), false
}

// readInstalled reads vendor/composer/installed.json beside a composer.json, in
// Composer 2's form ({"packages": [...]}) or Composer 1's (a list). The directory is
// read from disk, since the scan skips vendor/, and only inside the project.
//
// Implements: REQ-PHP-008
func readInstalled(root, directory string) []lockedPackage {
	if root == "" || directory == ".." || strings.HasPrefix(directory, "../") {
		return nil
	}
	data, ok := lang.OpenRoot(root).ReadBounded(filepath.Join(root, filepath.FromSlash(directory), "vendor", "composer", "installed.json"))
	if !ok {
		return nil
	}
	var v2 struct {
		Packages []lockedPackage `json:"packages"`
	}
	if json.Unmarshal(data, &v2) == nil && v2.Packages != nil {
		return v2.Packages
	}
	var v1 []lockedPackage
	_ = json.Unmarshal(data, &v1)
	return v1
}

// matchPrefix reports whether a class name falls under an autoload prefix. A PSR-4
// prefix ends in `\`; a PSR-0 one may end in `\` or `_` (Twig_), or name a namespace
// or class without a separator. Case is ignored, as PHP ignores it in class names.
func matchPrefix(qualifiedName, prefix string) bool {
	if prefix == "" || len(qualifiedName) < len(prefix) || !strings.EqualFold(qualifiedName[:len(prefix)], prefix) {
		return false
	}
	if strings.HasSuffix(prefix, `\`) || strings.HasSuffix(prefix, "_") || len(qualifiedName) == len(prefix) {
		return true
	}
	return qualifiedName[len(prefix)] == '\\' || qualifiedName[len(prefix)] == '_'
}

// target is a package as a project resolves it: the lock's version pins it, and a
// manifest constraint without a lock pins it only when it names one version.
//
// Implements: REQ-PHP-009
func (p *project) target(name string) lang.Target {
	t := lang.Target{Ecosystem: ecosystemComposer, Package: name}
	constraint := p.require[name]
	if k := p.locked[name]; k != nil && k.version != "" {
		t.Version, t.Pinned, t.Git = k.version, true, k.git
		if constraint != "" && constraint != k.version {
			t.Requested = constraint
		}
		return t
	}
	t.Version, t.Pinned = constraint, pinned(constraint)
	return t
}

var stability = regexp.MustCompile(`@(?i:dev|alpha|beta|rc|stable)$`)

// pinned is Composer's reading of a constraint: a bare version is exact in Composer
// ("1.2.3", "v1.2.3", even "1.2", which means 1.2.0), as is "=1.2.3"; a branch pins
// only with a commit ("dev-main#<sha>"); ranges, wildcards (`1.2.*`, `*`), `^`, `~`,
// `||` and branches float. An inline alias ("dev-main as 1.0.x") is read for what it
// stands for, a stability flag ("@beta") is ignored.
//
// Implements: REQ-PHP-009
func pinned(constraint string) bool {
	c := strings.TrimSpace(constraint)
	if before, _, ok := strings.Cut(c, " as "); ok {
		c = strings.TrimSpace(before)
	}
	c = stability.ReplaceAllString(c, "")
	if branch, reference, ok := strings.Cut(c, "#"); ok {
		return strings.HasPrefix(strings.ToLower(branch), "dev-") && lang.Commit(reference) || lang.Pinned(branch)
	}
	c = strings.TrimPrefix(strings.TrimPrefix(c, "=="), "=")
	return lang.Pinned(c)
}

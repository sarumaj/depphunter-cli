package javascript

import (
	"cmp"
	"encoding/json"
	"maps"
	"net/url"
	"path"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// A lock file records not only which version of a package is installed but what that
// package itself pulls in, which is the whole transitive graph - already on disk, with
// no registry to ask. tree collects it: package name -> the names it depends on.
//
// Versions are not part of the key. The map draws one building per package, so two
// versions of the same package are one node, and an edge between names is what the
// graph can hold anyway.
type tree struct {
	dependencies map[string]map[string]bool
	locked       map[string]string // package -> a version some lock file pinned it to
	// "parent/dep" -> the version of dep installed under parent rather than
	// hoisted ("" when that copy pins nothing), where a lock file says so.
	nested map[string]string
	// "name@version" -> dependency -> the version of the copy that package loads
	// ("" when the lock holds none), where a lock file records it: the node_modules
	// paths of package-lock.json, yarn.lock's descriptors and pnpm's exact versions
	// each name the copy, where the name alone stands for only one of them.
	exact map[string]map[string]string
	// package -> the names it depends on that a lock file resolves to a directory
	// of the project (a workspace, a link, a portal) rather than to a package.
	local map[string]map[string]bool
	// package -> the platforms it installs on, "os=linux & cpu=x64", where a lock
	// file says it installs on some only: the binaries a package such as esbuild
	// ships per platform, each an optional dependency every install lists.
	platform map[string]string
	// "name@version" -> the repository a git dependency was installed from, where
	// the version is the commit a lock file checked out ("name@" where it names
	// none).
	git map[string]string
}

func newTree() *tree {
	return &tree{
		dependencies: map[string]map[string]bool{}, locked: map[string]string{}, nested: map[string]string{},
		exact: map[string]map[string]string{}, local: map[string]map[string]bool{}, platform: map[string]string{},
		git: map[string]string{},
	}
}

func (t *tree) add(packageName string, dependencies ...string) {
	if packageName == "" {
		return
	}
	m := t.dependencies[packageName]
	if m == nil {
		m = map[string]bool{}
		t.dependencies[packageName] = m
	}
	for _, d := range dependencies {
		if d != "" && d != packageName {
			m[d] = true
		}
	}
}

// addLocal records that each of names depends on the project's own dependency.
func (t *tree) addLocal(names []string, dependency string) {
	for _, n := range names {
		if n == "" || dependency == "" {
			continue
		}
		if t.local[n] == nil {
			t.local[n] = map[string]bool{}
		}
		t.local[n][dependency] = true
	}
}

// addPlatform records the platforms each of names installs on; the first lock
// file to say keeps it.
func (t *tree) addPlatform(names []string, condition string) {
	for _, n := range names {
		if _, ok := t.platform[n]; !ok && n != "" && condition != "" {
			t.platform[n] = condition
		}
	}
}

// platformList is a manifest's os, cpu or libc field as a lock file copies it:
// a list, or a single value written without one (pnpm writes `libc: glibc`,
// Bun `"os": "darwin"`). Any other shape says nothing about the platform, and
// must not cost the lock file the rest of what it says.
type platformList []string

func (l *platformList) UnmarshalJSON(data []byte) error {
	var one string
	if json.Unmarshal(data, &one) == nil {
		*l = platformList{one}
	} else if json.Unmarshal(data, (*[]string)(l)) != nil {
		*l = nil
	}
	return nil
}

func (l *platformList) UnmarshalYAML(node *yaml.Node) error {
	var one string
	if node.Kind == yaml.ScalarNode && node.Decode(&one) == nil {
		*l = platformList{one}
	} else if node.Decode((*[]string)(l)) != nil {
		*l = nil
	}
	return nil
}

// platformCondition writes the os, cpu and libc lists of a package's manifest
// (as package-lock.json, pnpm-lock.yaml and bun.lock copy them) as Yarn Berry
// writes a yarn.lock entry's conditions: "os=linux & cpu=x64", a list of several
// values "(os=darwin | os=linux)", a negated value "!os=win32".
//
// Implements: REQ-JS-018
func platformCondition(operatingSystems, processors, cLibraries []string) string {
	var fields []string
	for _, field := range []struct {
		name   string
		values []string
	}{{"os", operatingSystems}, {"cpu", processors}, {"libc", cLibraries}} {
		var tokens []string
		for _, raw := range field.values {
			value := strings.TrimLeft(raw, "!")
			if value == "" {
				continue
			}
			prefix := ""
			if (len(raw)-len(value))%2 == 1 {
				prefix = "!"
			}
			tokens = append(tokens, prefix+field.name+"="+value)
		}
		switch len(tokens) {
		case 0:
		case 1:
			fields = append(fields, tokens[0])
		default:
			fields = append(fields, "("+strings.Join(tokens, " | ")+")")
		}
	}
	return strings.Join(fields, " & ")
}

// addExact records what one copy of a package loads, under each name it is
// installed as (the real one and an alias). The first lock file, and within one
// the first copy, to name a version keeps it.
func (t *tree) addExact(names []string, version string, dependencies map[string]string) {
	if version == "" {
		return
	}
	for _, n := range names {
		if _, ok := t.exact[n+"@"+version]; !ok && n != "" {
			t.exact[n+"@"+version] = dependencies
		}
	}
}

// Dependencies implements lang.Transitive. The edges are the name's, from every
// copy installed (one node stands for them all); the version of each is the copy
// that t's own copy loads where a lock file says which that is, else the one
// installed under t's name (bun.lock), else the one the name is locked to.
//
// Implements: REQ-SUP-009
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != lang.EcosystemNPM {
		return nil
	}
	var exact map[string]string
	if t.Version != "" {
		exact = r.tree.exact[t.Package+"@"+t.Version]
	}
	var out []lang.Target
	for dependency := range r.tree.dependencies[t.Package] {
		version := exact[dependency] // "" when the lock holds no copy it loads
		ok := version != ""
		if !ok {
			version, ok = r.tree.nested[t.Package+"/"+dependency]
		}
		if !ok {
			version = r.tree.locked[dependency]
		}
		// A name no lock file installs from a registry, which a workspace package
		// of the project has: npm, pnpm, Yarn and Bun all link that one.
		if directory, ok := r.byName[dependency]; ok && version == "" {
			out = append(out, lang.Target{Local: directory})
			continue
		}
		out = append(out, lang.Target{
			Ecosystem: lang.EcosystemNPM, Package: dependency, Version: version,
			// It is in a lock file, which is what pins an npm package.
			Pinned:   version != "",
			Platform: r.tree.platform[dependency],
			Origin:   r.tree.origin(dependency, version), // Implements: REQ-JS-019
		})
	}
	// What the lock resolves to a directory of the project is that workspace
	// package's, where the project has one of the name; a link out of the
	// project, or to a directory the scan left out, is nothing on the map.
	//
	// Implements: REQ-JS-004
	for dependency := range r.tree.local[t.Package] {
		if directory, ok := r.byName[dependency]; ok && !r.tree.dependencies[t.Package][dependency] {
			out = append(out, lang.Target{Local: directory})
		}
	}
	return out
}

// addPackageLockTree reads the dependency edges of package-lock.json (or
// npm-shrinkwrap.json), v1 through v3. v2 and v3 key every copy by its install
// path; v1 nests the same tree, which is flattened into those paths.
//
// Implements: REQ-SUP-009
func (t *tree) addPackageLockTree(lock *packageLock) {
	paths := lock.Packages
	if len(paths) == 0 {
		paths = map[string]lockPath{}
		flattenV1(paths, "", lock.Dependencies)
	}
	keys := make([]string, 0, len(paths))
	for k := range paths {
		keys = append(keys, k)
	}
	// Shallowest first: the hoisted copy is the one a name stands for, and the
	// first copy of a version is the one that version's node follows.
	sort.Slice(keys, func(i, j int) bool {
		depthI, depthJ := strings.Count(keys[i], "node_modules/"), strings.Count(keys[j], "node_modules/")
		if depthI != depthJ {
			return depthI < depthJ
		}
		return keys[i] < keys[j]
	})
	seen := map[string]bool{}
	for _, key := range keys {
		p := paths[key]
		alias := lockName(key)
		if alias == "" || p.Link {
			continue // the project, a workspace, or a link to one: local, not npm's
		}
		name := cmp.Or(p.Name, alias)
		names := []string{name, alias} // a package imported as "c2" is that node
		if p.Version != "" && !seen[name] {
			seen[name] = true
			t.locked[name] = p.Version
		}
		t.addPlatform(names, platformCondition(p.OS, p.CPU, p.Libc))
		dependencies := map[string]string{}
		for dependency, optional := range p.requirements() {
			found, ok := installed(paths, key, dependency)
			switch {
			case ok && found.Link: // a workspace (or a file: directory) linked in
				t.addLocal(names, dependency)
			case ok:
				dependencies[cmp.Or(found.Name, dependency)] = found.Version
			case !optional: // missing from the lock: the name is all there is
				if _, have := dependencies[dependency]; !have {
					dependencies[dependency] = ""
				}
			} // an optional dependency missing is one this install left out
		}
		for _, n := range names {
			for dependency := range dependencies {
				t.add(n, dependency)
			}
		}
		t.addExact(names, p.Version, dependencies)
	}
}

// requirements are the names the package at p needs, each true when an install
// may leave it out: its dependencies and optional dependencies, and the peer
// dependencies npm 7 onwards installs beside it, optional where
// peerDependenciesMeta says so. A name listed twice is required if either says.
//
// Implements: REQ-JS-007
func (p lockPath) requirements() map[string]bool {
	out := map[string]bool{}
	add := func(m map[string]string, optional func(string) bool) {
		for dependency := range m {
			if have, ok := out[dependency]; !ok || have {
				out[dependency] = optional(dependency)
			}
		}
	}
	add(p.Dependencies, func(string) bool { return false })
	add(p.OptionalDependencies, func(string) bool { return true })
	add(p.PeerDependencies, func(dependency string) bool { return p.PeerDependenciesMeta[dependency].Optional })
	return out
}

// installed finds the copy of dependency that the package installed at from loads:
// Node looks in from's own node_modules, then in each directory's above it.
func installed(paths map[string]lockPath, from, dependency string) (lockPath, bool) {
	for directory := from; ; directory = path.Dir(directory) {
		if path.Base(directory) == "node_modules" {
			continue
		}
		key := "node_modules/" + dependency
		if directory != "." {
			key = directory + "/" + key
		}
		if p, ok := paths[key]; ok {
			return p, true
		}
		if directory == "." {
			return lockPath{}, false
		}
	}
}

// flattenV1 turns v1's nested "dependencies" into the install paths v2 keys
// them by. An alias is written "npm:real@1.2.3", a local package "file:dir".
func flattenV1(out map[string]lockPath, parent string, dependencies map[string]*lockV1) {
	for name, d := range dependencies {
		if d == nil {
			continue
		}
		key := path.Join(parent, "node_modules", name)
		p := lockPath{Version: d.Version, Dependencies: d.Requires}
		switch {
		case strings.HasPrefix(d.Version, "npm:"):
			p.Name, p.Version = splitIdentifier(strings.TrimPrefix(d.Version, "npm:"))
		case strings.HasPrefix(d.Version, "file:") || strings.HasPrefix(d.Version, "link:"):
			p.Link, p.Version = true, ""
		}
		out[key] = p
		flattenV1(out, key, d.Dependencies)
	}
}

// splitIdentifier splits "name@version" (or "@scope/name@range") at the "@" after
// the name.
func splitIdentifier(s string) (name, version string) {
	if i := strings.Index(s[min(1, len(s)):], "@"); i >= 0 {
		return s[:i+1], s[i+2:]
	}
	return s, ""
}

// lockName reads the package name from a package-lock path key: "node_modules/x" and
// the nested "node_modules/a/node_modules/b" both name their last segment.
func lockName(key string) string {
	i := strings.LastIndex(key, "node_modules/")
	if i < 0 {
		return ""
	}
	return key[i+len("node_modules/"):]
}

// addPnpmTree reads the dependency edges of pnpm-lock.yaml. Each entry names the
// exact version of every dependency, peer context aside.
//
// Implements: REQ-SUP-009
func (t *tree) addPnpmTree(doc *pnpmLock) {
	aliases := map[[2]string]string{} // {alias, real name} -> version
	add := func(name, version string, lists ...map[string]string) {
		dependencies := map[string]string{}
		for _, m := range lists {
			for alias, v := range m {
				if strings.HasPrefix(v, "link:") || strings.HasPrefix(v, "file:") {
					t.addLocal([]string{name}, alias) // a workspace package, or a directory
				} else if dependency, version, ok := doc.reference(alias, v); ok {
					dependencies[dependency] = version
					if dependency != alias {
						aliases[[2]string{alias, dependency}] = version
					}
				}
			}
		}
		for dependency := range dependencies {
			t.add(name, dependency)
		}
		t.addExact([]string{name}, version, dependencies)
	}
	for _, key := range slices.Sorted(maps.Keys(doc.Packages)) {
		p := doc.Packages[key]
		name, version, repository := doc.keyed(key)
		if version != "" {
			t.locked[name] = version
		}
		t.addGit([]string{name}, version, repository) // Implements: REQ-JS-019
		// Implements: REQ-JS-018
		t.addPlatform([]string{name}, platformCondition(p.OS, p.CPU, p.Libc))
		// v9 keeps the edges in "snapshots": "packages" says nothing about them.
		if len(doc.Snapshots) == 0 {
			add(name, version, p.Dependencies, p.OptionalDependencies)
		}
	}
	for _, key := range slices.Sorted(maps.Keys(doc.Snapshots)) {
		s := doc.Snapshots[key]
		name, version, _ := doc.keyed(key)
		add(name, version, s.Dependencies, s.OptionalDependencies)
	}
	// A package the project itself imports under an alias ("c2": "npm:c@^2") is
	// a node of that name: it needs what the real package does.
	for _, section := range append([]pnpmDependencies{doc.pnpmDependencies}, slices.Collect(maps.Values(doc.Importers))...) {
		for _, m := range []map[string]any{section.Dependencies, section.DevDependencies, section.OptionalDependencies} {
			for alias, v := range m {
				if dependency, version, ok := doc.reference(alias, pnpmReference(v)); ok && dependency != alias {
					aliases[[2]string{alias, dependency}] = version
				}
			}
		}
	}
	for pair, version := range aliases {
		for dependency := range t.dependencies[pair[1]] {
			t.add(pair[0], dependency)
		}
		if dependencies, ok := t.exact[pair[1]+"@"+version]; ok {
			t.addExact(pair[:1], version, dependencies)
		}
	}
}

// reference reads a dependency reference of pnpm-lock.yaml as the package and version
// it installs; ok is false for a workspace or local directory. A dependency on
// something outside the registry is written as that entry's key, which names
// the package only in the entry's own "name".
func (lock *pnpmLock) reference(alias, v string) (name, version string, ok bool) {
	v, _, _ = strings.Cut(v, "(")
	switch {
	case v == "" || strings.HasPrefix(v, "link:") || strings.HasPrefix(v, "file:"):
		return "", "", false
	case startsWithDigit(v):
		v, _, _ = strings.Cut(v, "_") // v5's peer suffix
		return alias, v, true
	}
	if name, commit, _, git := lock.gitReference(alias, v); git {
		return name, commit, true
	}
	for _, key := range []string{v, "/" + v} {
		if p, ok := lock.Packages[key]; ok && p.Name != "" {
			return p.Name, p.Version, true
		}
	}
	if name, version = pnpmKey(v); version == "" { // an alias: "real@1.0.0", "/real@1.0.0", "/real/1.0.0"
		return alias, "", true
	}
	return name, version, true
}

// pnpmKey splits a package key into name and version. Version 6 onwards writes
// "/lodash@4.17.21" and "react-dom@18.3.1(react@18.3.1)", where the parenthesized
// part is peer-dependency context; version 5 wrote "/lodash/4.17.21",
// "/@scope/pkg/1.2.3" and "/react-dom/18.2.0_react@18.2.0", where the segment
// after the name is the version and "_" starts the peer context.
func pnpmKey(key string) (name, version string) {
	key = strings.TrimPrefix(key, "/")
	key, _, _ = strings.Cut(key, "(")
	n := 0
	if strings.HasPrefix(key, "@") {
		i := strings.Index(key, "/")
		if i < 0 {
			return key, ""
		}
		n = i + 1
	}
	j := strings.IndexAny(key[n:], "@/")
	if j < 0 {
		return key, ""
	}
	name, version = key[:n+j], key[n+j+1:]
	if key[n+j] == '/' {
		if !startsWithDigit(version) {
			return key, "" // a path that names no version
		}
		version, _, _ = strings.Cut(version, "_")
	}
	return name, version
}

func startsWithDigit(s string) bool { return s != "" && s[0] >= '0' && s[0] <= '9' }

// yarnEntry is one entry of a yarn.lock, classic or Berry: the descriptors
// ("name@range") it answers for, what it resolved to, and the descriptors it
// in turn requires.
type yarnEntry struct {
	descriptors         []string
	version, resolution string
	dependencies        [][2]string // name, range
	// Berry's platform conditions, "os=linux & cpu=x64": the entry is installed
	// only where they hold.
	conditions string
	// Classic's record of what it downloaded: a registry tarball, or a git
	// repository at a commit.
	resolved string
	// The repository a git dependency was fetched from; its version is then the
	// commit, "" where the entry names none.
	repository string
}

// readYarnEntries reads a yarn.lock's entries. Classic writes `version "1.2.3"`
// and `    dep "^1"`, Berry `version: 1.2.3` and `    dep: "npm:^1"`; a
// dependency block is indented one level deeper than the entry's own keys, which
// is the only thing telling a dependency "version-guard" from "version".
func readYarnEntries(data []byte) []*yarnEntry {
	var entries []*yarnEntry
	var e *yarnEntry
	section := ""
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "" || strings.HasPrefix(trimmed, "#"):
		case !strings.HasPrefix(line, " "): // the descriptors of a new entry
			e, section = &yarnEntry{}, ""
			entries = append(entries, e)
			for _, k := range strings.Split(strings.TrimSuffix(trimmed, ":"), ",") {
				if k = strings.Trim(strings.TrimSpace(k), `"`); k != "" {
					e.descriptors = append(e.descriptors, k)
				}
			}
		case e == nil:
		case strings.HasPrefix(line, "    "):
			if (section == "dependencies" || section == "optionalDependencies") && !strings.HasPrefix(line, "     ") {
				if name, versionRange := yarnDependency(trimmed); name != "" {
					e.dependencies = append(e.dependencies, [2]string{name, versionRange})
				}
			}
		default:
			section = strings.TrimSuffix(strings.Fields(trimmed)[0], ":")
			switch k, v := yarnField(line); k {
			case "version":
				e.version = v
			case "resolution":
				e.resolution = v
			case "resolved":
				e.resolved = v
			case "conditions": // "conditions: os=linux & cpu=x64", spaces and all
				_, condition, _ := strings.Cut(trimmed, ":")
				e.conditions = strings.Trim(strings.TrimSpace(condition), `"`)
			}
		}
	}
	for _, e := range entries {
		e.pinGit()
	}
	return entries
}

// pinGit reads the entry as a git dependency where it is one: Berry's resolution
// ("x@https://github.com/o/r.git#commit=<sha>") or classic's resolved URL names
// the repository and the commit, which stands for the version (the one of the
// repository's package.json).
//
// Implements: REQ-JS-019
func (e *yarnEntry) pinGit() {
	_, source := splitIdentifier(e.resolution)
	if repository, commit, ok := gitSource(cmp.Or(source, e.resolved)); ok {
		e.repository, e.version = repository, commit
	}
}

// yarnVersions maps each descriptor of a yarn.lock's entries ("name@range") to
// the version it resolved to.
//
// Implements: REQ-JS-008
func yarnVersions(entries []*yarnEntry) map[string]string {
	out := map[string]string{}
	for _, e := range entries {
		for _, d := range e.descriptors {
			if e.version != "" {
				out[d] = e.version
			}
		}
	}
	return out
}

// unpinnedGit names the git dependencies of a yarn.lock whose entries name no
// commit.
func unpinnedGit(entries []*yarnEntry) []string {
	var out []string
	for _, e := range entries {
		if name, _ := e.identifier(); e.repository != "" && e.version == "" && !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	return out
}

// yarnDependency reads one line of a dependency block: `dep "^1"`, `dep: ^1`,
// `"@scope/dep": "npm:^1"`.
func yarnDependency(s string) (name, versionRange string) {
	if strings.HasPrefix(s, `"`) {
		end := strings.Index(s[1:], `"`)
		if end < 0 {
			return "", ""
		}
		name, s = s[1:1+end], s[2+end:]
	} else {
		i := strings.IndexAny(s, " :")
		if i < 0 {
			return "", ""
		}
		name, s = s[:i], s[i:]
	}
	s = strings.TrimPrefix(strings.TrimSpace(s), ":")
	return name, strings.Trim(strings.TrimSpace(s), `"`)
}

// yarnLocalProtocols are the ranges and resolutions that name a directory of the
// project (or beside it), not a package any registry has.
var yarnLocalProtocols = []string{"workspace:", "portal:", "link:", "file:"}

func yarnLocal(versionRange string) bool {
	for _, p := range yarnLocalProtocols {
		if strings.HasPrefix(versionRange, p) {
			return true
		}
	}
	return false
}

// identifier is the package an entry installs and whether it is a local one. Berry
// writes it as the resolution ("real@npm:1.2.3", "ws@workspace:packages/ws");
// classic only has the descriptors, where an alias reads "alias@npm:real@^1".
func (e *yarnEntry) identifier() (name string, local bool) {
	if e.resolution != "" {
		name, rest := splitIdentifier(e.resolution)
		return name, yarnLocal(rest)
	}
	if len(e.descriptors) == 0 || e.descriptors[0] == "__metadata" {
		return "", true
	}
	name, versionRange := splitIdentifier(e.descriptors[0])
	if real, ok := strings.CutPrefix(versionRange, "npm:"); ok {
		if n, v := splitIdentifier(real); v != "" {
			name = n
		}
	}
	return name, yarnLocal(versionRange)
}

// names are the names the entry is installed as: its package's, and any alias.
func (e *yarnEntry) names(identifier string) []string {
	out := []string{identifier}
	for _, d := range e.descriptors {
		if n, _ := splitIdentifier(d); n != identifier && !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	return out
}

// yarnCandidates are the descriptors a dependency's range may be keyed under:
// as written; with "npm:" where Berry adds the default protocol; and for a
// "patch:" range, the range it patches.
func yarnCandidates(name, versionRange string) []string {
	out := []string{name + "@" + versionRange}
	if base, ok := strings.CutPrefix(versionRange, "patch:"); ok {
		base, _, _ = strings.Cut(base, "#")
		if u, err := url.PathUnescape(base); err == nil {
			base = u
		}
		return append(out, yarnCandidates(splitIdentifier(base))...)
	}
	if !yarnProtocol(versionRange) {
		out = append(out, name+"@npm:"+versionRange)
	}
	return out
}

// yarnProtocol tells "npm:^1", "workspace:*" and "https://..." from a bare range.
func yarnProtocol(versionRange string) bool {
	i := strings.Index(versionRange, ":")
	if i <= 0 {
		return false
	}
	for _, c := range versionRange[:i] {
		if (c < 'a' || c > 'z') && c != '+' && c != '-' {
			return false
		}
	}
	return true
}

// addYarnTree reads the dependency edges of a yarn.lock, classic or Berry: each
// dependency's descriptor is looked up among the entries' own, which is the copy
// the package gets.
//
// Implements: REQ-SUP-009, REQ-JS-008
func (t *tree) addYarnTree(entries []*yarnEntry) {
	byDescriptor := map[string]*yarnEntry{}
	for _, e := range entries {
		for _, d := range e.descriptors {
			byDescriptor[d] = e
		}
	}
	find := func(name, versionRange string) *yarnEntry {
		for _, d := range yarnCandidates(name, versionRange) {
			if e := byDescriptor[d]; e != nil {
				return e
			}
		}
		return nil
	}
	for _, e := range entries {
		name, local := e.identifier()
		if name == "" || local {
			continue
		}
		names := e.names(name)
		if e.version != "" {
			for _, n := range names {
				t.locked[n] = e.version
			}
		}
		// Implements: REQ-JS-018
		t.addPlatform(names, e.conditions)
		t.addGit(names, e.version, e.repository) // Implements: REQ-JS-019
		dependencies := map[string]string{}
		for _, d := range e.dependencies {
			switch dependency := find(d[0], d[1]); {
			case dependency != nil:
				if n, local := dependency.identifier(); !local {
					dependencies[n] = dependency.version
				} else {
					t.addLocal(names, n)
				}
			case yarnLocal(d[1]):
				t.addLocal(names, d[0])
			default:
				if _, have := dependencies[d[0]]; !have {
					dependencies[d[0]] = ""
				}
			}
		}
		for _, n := range names {
			for dependency := range dependencies {
				t.add(n, dependency)
			}
		}
		t.addExact(names, e.version, dependencies)
	}
}

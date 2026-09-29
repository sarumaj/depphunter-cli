package javascript

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/tidwall/jsonc"
	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
	"github.com/sarumaj/depphunter-cli/internal/trace"
)

// Implements: REQ-JS-005
var builtins = map[string]bool{}

func init() {
	// cSpell: disable
	for _, m := range strings.Fields(`assert async_hooks buffer child_process cluster console constants crypto
		dgram diagnostics_channel dns domain events fs http http2 https inspector module net os path
		perf_hooks process punycode querystring readline repl stream string_decoder sys timers tls
		trace_events tty url util v8 vm wasi worker_threads zlib`) {
		builtins[m] = true
	}
	// cSpell: enable
}

// Extensions tried, in order, for extension-less specifiers (TypeScript's order
// first). ".vue" comes last: Vue CLI and webpack setups resolve "./Button" to
// Button.vue, and nothing else is found under that name when it applies.
var probeExtensions = []string{".ts", ".tsx", ".d.ts", ".js", ".jsx", ".mjs", ".cjs", ".mts", ".cts", ".json", ".vue"}

type resolver struct {
	files        map[string]bool
	directories  map[string]bool
	dependencies map[string]map[string]string // package.json dir -> dependency -> version range
	locks        map[string]map[string]string // lock file (or importer) dir -> package -> exact version
	byName       map[string]string            // workspace package name -> its directory
	configs      map[string]*tsconfig         // directory -> effective tsconfig/jsconfig
	kits         map[string]bool              // directories holding a SvelteKit svelte.config
	tree         *tree                        // what the lock files say the packages need
	lang.NoteList
}

// The notes a resolver keeps reach --explain only through lang.Noter.
var _ lang.Noter = (*resolver)(nil)

type tsconfig struct {
	baseURL string // project-relative; "" when unset
	paths   []pathRule
}

type pathRule struct {
	pattern string   // may contain one "*"
	targets []string // project-relative, may contain one "*"
}

func newResolver(all []*scan.File) *resolver {
	r := &resolver{
		files: map[string]bool{}, directories: map[string]bool{}, dependencies: map[string]map[string]string{},
		locks: map[string]map[string]string{}, byName: map[string]string{}, configs: map[string]*tsconfig{},
		kits: map[string]bool{}, tree: newTree(),
	}
	byPath := map[string]*scan.File{}
	for _, f := range all {
		r.files[f.Path] = true
		byPath[f.Path] = f
		for d := path.Dir(f.Path); d != "." && !r.directories[d]; d = path.Dir(d) {
			r.directories[d] = true
		}
	}
	yarn := map[string]yarnDescriptors{} // yarn.lock dir -> its descriptors
	var bunDirectories []string          // bun.lock directories, in path order
	buns := map[string]*bunLock{}
	for _, f := range all {
		directory := path.Dir(f.Path)
		switch path.Base(f.Path) {
		case "svelte.config.js", "svelte.config.mjs", "svelte.config.cjs", "svelte.config.ts":
			r.kits[directory] = true
		case "package.json":
			var pj struct {
				Name                                                                  string
				Dependencies, DevDependencies, PeerDependencies, OptionalDependencies map[string]string
			}
			if readJSON(f.AbsolutePath, &pj) != nil {
				continue
			}
			dependencies := map[string]string{}
			for _, m := range []map[string]string{pj.OptionalDependencies, pj.PeerDependencies, pj.DevDependencies, pj.Dependencies} {
				for k, v := range m {
					dependencies[k] = v
				}
			}
			r.dependencies[directory] = dependencies
			if pj.Name != "" {
				r.byName[pj.Name] = directory
			}
		// One decoding of each lock file serves both the versions the project's
		// imports pin and the tree the transitive walk follows: a monorepo's
		// pnpm-lock.yaml runs to megabytes of YAML.
		case "package-lock.json", "npm-shrinkwrap.json":
			// npm-shrinkwrap.json is package-lock.json's format under the name a
			// published package's lock takes; beside one, npm reads only it.
			// Implements: REQ-JS-007
			if path.Base(f.Path) == "package-lock.json" && r.files[path.Join(directory, "npm-shrinkwrap.json")] {
				continue
			}
			var lock packageLock
			if readJSON(f.AbsolutePath, &lock) == nil {
				r.noteUnpinned(f.Path, lock.pinGit(r.tree))
				r.addLock(directory, lock.versions())
				r.tree.addPackageLockTree(&lock)
			}
		case "yarn.lock":
			if data, err := os.ReadFile(f.AbsolutePath); err == nil {
				entries := readYarnEntries(data)
				r.noteUnpinned(f.Path, unpinnedGit(entries))
				yarn[directory] = newYarnDescriptors(yarnVersions(entries))
				r.tree.addYarnTree(entries)
			}
		case "pnpm-lock.yaml":
			data, err := os.ReadFile(f.AbsolutePath)
			if err != nil {
				continue
			}
			lock, err := readPnpmLock(data)
			if err != nil {
				continue
			}
			for importer, versions := range lock.versions() {
				r.addLock(path.Join(directory, importer), versions)
			}
			r.tree.addPnpmTree(lock)
		case "bun.lockb":
			// Implements: REQ-JS-017, REQ-TRC-017
			if !r.files[path.Join(directory, "bun.lock")] && !r.files[path.Join(directory, "yarn.lock")] {
				r.Note(f.Path, trace.NoteUnread, "Bun's binary lock file is not read: nothing here pins "+
					"what it installed or records its edges; `bun install --save-text-lockfile` writes bun.lock, "+
					"which is read")
			}
		case "bun.lock":
			data, err := os.ReadFile(f.AbsolutePath)
			if err != nil {
				continue
			}
			if lock, err := readBunLock(data); err == nil {
				bunDirectories = append(bunDirectories, directory)
				buns[directory] = lock
			}
		}
	}
	// yarn.lock keys are "name@range": pin each declared range of the packages below.
	// Implements: REQ-JS-008
	for lockDirectory, descriptors := range yarn {
		for packageDirectory, dependencies := range r.dependencies {
			if lockDirectory != "." && packageDirectory != lockDirectory && !strings.HasPrefix(packageDirectory, lockDirectory+"/") {
				continue
			}
			pinned := map[string]string{}
			for name, versionRange := range dependencies {
				if v := descriptors.version(name, versionRange); v != "" {
					pinned[name] = v
				}
			}
			r.addLock(packageDirectory, pinned)
		}
	}
	// bun.lock comes last: beside another lock file it only answers for what that
	// one does not, so adding Bun's lock never changes a version another gave.
	// Implements: REQ-JS-016
	for _, directory := range bunDirectories {
		lock := buns[directory]
		entries := lock.entries()
		for importer, versions := range lock.versions(entries) {
			r.addLock(path.Join(directory, importer), versions)
		}
		r.tree.addBunTree(entries)
	}
	// tsconfig.json wins over jsconfig.json in the same directory.
	for _, name := range []string{"jsconfig.json", "tsconfig.json"} {
		for _, f := range all {
			if path.Base(f.Path) == name {
				if c := loadTSConfig(f.Path, byPath, map[string]bool{}); c != nil {
					r.configs[path.Dir(f.Path)] = c
				}
			}
		}
	}
	return r
}

func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	return r.resolve(rawImport.Module, file)
}

// Implements: REQ-JS-002, REQ-JS-004, REQ-JS-005, REQ-JS-006
func (r *resolver) resolve(spec, from string) lang.Target {
	directory := path.Dir(from)
	switch {
	case spec == "" || strings.Contains(spec, "://") || strings.HasPrefix(spec, "data:") || strings.HasPrefix(spec, "/"):
		return lang.Target{}
	case spec == "." || spec == ".." || strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../"):
		t, _ := r.probe(path.Join(directory, spec))
		return t // unresolvable relative imports point outside the project: drop them
	}
	if c := r.nearestConfig(directory); c != nil {
		if t, ok := r.viaConfig(c, spec); ok {
			return t
		}
	}
	if name, ok := strings.CutPrefix(spec, "node:"); ok {
		packageName, _ := splitPackage(name)
		return lang.Target{Ecosystem: ecosystemNode, Package: packageName}
	}
	packageName, subpath := splitPackage(spec)
	if builtins[packageName] {
		return lang.Target{Ecosystem: ecosystemNode, Package: packageName}
	}
	if packageDirectory, ok := r.byName[packageName]; ok {
		if subpath != "" {
			if t, ok := r.probe(path.Join(packageDirectory, subpath)); ok {
				return t
			}
		}
		return lang.Target{Local: packageDirectory}
	}
	if t, ok := r.svelteKit(spec, directory); ok {
		return t
	}
	// Bundler aliases and package.json "imports" we cannot see: not an npm package.
	// Neither is anything with a "$" or ":", which an npm name cannot hold: the
	// modules a framework generates, as SvelteKit's $app and $env, Astro's
	// astro:content and Vite's virtual: plugins.
	if strings.ContainsAny(spec, "$:") || strings.HasPrefix(spec, "~") || strings.HasPrefix(spec, "#") || strings.HasPrefix(spec, "@/") {
		return lang.Target{}
	}
	t, declared := r.declared(packageName, directory)
	t.Ecosystem, t.Package, t.Unresolved = ecosystemNPM, packageName, !declared
	t.Platform = r.tree.platform[packageName] // Implements: REQ-JS-018
	t.Origin = r.gitOrigin(t)
	return t
}

// gitOrigin is the repository a package the project declares was installed from,
// where it is a git dependency: the one the lock file that pins it records, or,
// with no lock file pinning it, the one the declared range names
// ("github:owner/repo#main").
//
// Implements: REQ-JS-019
func (r *resolver) gitOrigin(t lang.Target) string {
	if t.Pinned {
		return r.tree.origin(t.Package, t.Version)
	}
	repository, _, _ := gitSource(t.Version)
	return repository
}

// noteUnpinned reports the git dependencies a lock file names no commit for.
//
// Implements: REQ-JS-019, REQ-TRC-017
func (r *resolver) noteUnpinned(file string, names []string) {
	if len(names) > 0 {
		r.Note(file, trace.NoteGitUnpinned, unpinnedNote(names))
	}
}

// svelteKit resolves SvelteKit's $lib alias to src/lib beside the nearest
// svelte.config, the directory the framework points it at unless configured
// otherwise.
//
// Implements: REQ-JS-015
func (r *resolver) svelteKit(spec, directory string) (lang.Target, bool) {
	rest, ok := strings.CutPrefix(spec, "$lib")
	if !ok || rest != "" && rest[0] != '/' {
		return lang.Target{}, false
	}
	for d := directory; ; d = path.Dir(d) {
		if r.kits[d] {
			return r.probe(path.Join(d, "src/lib", rest))
		}
		if d == "." {
			return lang.Target{}, false
		}
	}
}

// splitPackage splits "@scope/name/sub/path" into "@scope/name" and "sub/path".
func splitPackage(spec string) (string, string) {
	parts := strings.SplitN(spec, "/", 3)
	if strings.HasPrefix(spec, "@") && len(parts) >= 2 {
		packageName := parts[0] + "/" + parts[1]
		if len(parts) == 3 {
			return packageName, parts[2]
		}
		return packageName, ""
	}
	packageName, subpath, _ := strings.Cut(spec, "/")
	return packageName, subpath
}

// probe finds the file or directory a module path refers to.
//
// Implements: REQ-JS-002
func (r *resolver) probe(p string) (lang.Target, bool) {
	p = path.Clean(p)
	if strings.HasPrefix(p, "../") || p == ".." {
		return lang.Target{}, false
	}
	if r.files[p] {
		return lang.Target{Local: p}, true
	}
	bases := []string{p}
	// TypeScript ESM style: "./util.js" in source refers to util.ts.
	switch extension := path.Ext(p); extension {
	case ".js", ".jsx", ".mjs", ".cjs":
		bases = append(bases, strings.TrimSuffix(p, extension))
	}
	for _, b := range bases {
		for _, extension := range probeExtensions {
			if r.files[b+extension] {
				return lang.Target{Local: b + extension}, true
			}
		}
	}
	for _, extension := range probeExtensions {
		if index := path.Join(p, "index"+extension); r.files[index] {
			return lang.Target{Local: index}, true
		}
	}
	if r.directories[p] {
		return lang.Target{Local: p}, true
	}
	return lang.Target{}, false
}

func (r *resolver) nearestConfig(directory string) *tsconfig {
	for d := directory; ; d = path.Dir(d) {
		if c := r.configs[d]; c != nil {
			return c
		}
		if d == "." {
			return nil
		}
	}
}

// Implements: REQ-JS-003
func (r *resolver) viaConfig(c *tsconfig, spec string) (lang.Target, bool) {
	best, bestLength := (*pathRule)(nil), -1
	var star string
	for i := range c.paths {
		rule := &c.paths[i]
		prefix, suffix, wild := strings.Cut(rule.pattern, "*")
		switch {
		case !wild && spec == rule.pattern && len(prefix) > bestLength:
			best, bestLength, star = rule, len(prefix), ""
		case wild && strings.HasPrefix(spec, prefix) && strings.HasSuffix(spec, suffix) &&
			len(spec) >= len(prefix)+len(suffix) && len(prefix) > bestLength:
			best, bestLength, star = rule, len(prefix), spec[len(prefix):len(spec)-len(suffix)]
		}
	}
	if best != nil {
		for _, t := range best.targets {
			if probed, ok := r.probe(strings.Replace(t, "*", star, 1)); ok {
				return probed, true
			}
		}
	}
	if c.baseURL != "" {
		return r.probe(path.Join(c.baseURL, spec))
	}
	return lang.Target{}, false
}

// declared looks up packageName in the nearest package.json files that declare it, preferring
// the exact version from the nearest lock file: the range in package.json is what
// was asked for, the lock is what is installed. Where several lock files sit in one
// directory, the first to answer wins, in the order npm-shrinkwrap.json (or else
// package-lock.json), pnpm-lock.yaml, yarn.lock, bun.lock.
//
// Implements: REQ-JS-006, REQ-JS-007, REQ-JS-010
func (r *resolver) declared(packageName, directory string) (lang.Target, bool) {
	for d := directory; ; d = path.Dir(d) {
		if v, ok := r.dependencies[d][packageName]; ok {
			for l := d; ; l = path.Dir(l) {
				if exact := r.locks[l][packageName]; exact != "" {
					return lang.Target{Version: exact, Requested: v, Pinned: true}, true
				}
				if l == "." {
					break
				}
			}
			// Without a lock only a complete version pins: npm reads "1.2" as 1.2.x.
			return lang.Target{Version: v, Pinned: lang.PinnedSemver(v)}, true
		}
		if d == "." {
			return lang.Target{}, false
		}
	}
}

// Implements: REQ-JS-003
func loadTSConfig(relative string, files map[string]*scan.File, seen map[string]bool) *tsconfig {
	f := files[relative]
	if f == nil || seen[relative] {
		return nil
	}
	seen[relative] = true
	data, err := os.ReadFile(f.AbsolutePath)
	if err != nil {
		return nil
	}
	var raw struct {
		Extends         json.RawMessage
		CompilerOptions struct {
			BaseURL *string             `json:"baseUrl"`
			Paths   map[string][]string `json:"paths"`
		} `json:"compilerOptions"`
	}
	// Implements: REQ-DIST-016
	if json.Unmarshal(jsonc.ToJSON(data), &raw) != nil { // tsconfig allows comments and trailing commas
		return nil
	}
	directory := path.Dir(relative)

	// Start from the (relative) configs this one extends; later entries win.
	c := &tsconfig{}
	var parents []string
	if json.Unmarshal(raw.Extends, &parents) != nil {
		var one string
		if json.Unmarshal(raw.Extends, &one) == nil && one != "" {
			parents = []string{one}
		}
	}
	for _, p := range parents {
		if !strings.HasPrefix(p, ".") {
			continue // package-provided base configs do not define project paths
		}
		jsonPath := path.Join(directory, p)
		if !strings.HasSuffix(jsonPath, ".json") {
			jsonPath += ".json"
		}
		if pc := loadTSConfig(jsonPath, files, seen); pc != nil {
			*c = *pc
		}
	}
	if b := raw.CompilerOptions.BaseURL; b != nil {
		c.baseURL = path.Join(directory, *b)
	}
	if raw.CompilerOptions.Paths != nil {
		base := directory
		if c.baseURL != "" {
			base = c.baseURL
		}
		c.paths = nil
		for pattern, targets := range raw.CompilerOptions.Paths {
			rule := pathRule{pattern: pattern}
			for _, t := range targets {
				rule.targets = append(rule.targets, path.Join(base, t))
			}
			c.paths = append(c.paths, rule)
		}
	}
	return c
}

func readJSON(absolute string, v any) error {
	data, err := os.ReadFile(absolute)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// Implements: REQ-SUP-003
func (r *resolver) addLock(directory string, versions map[string]string) {
	if len(versions) == 0 {
		return
	}
	if r.locks[directory] == nil {
		r.locks[directory] = map[string]string{}
	}
	for k, v := range versions {
		if _, ok := r.locks[directory][k]; !ok {
			r.locks[directory][k] = v
		}
	}
}

// yarnField reads one "key value" line of a yarn.lock. The classic format writes
// `version "1.2.3"`, Berry writes `version: 1.2.3`; both may quote the value.
func yarnField(line string) (key, value string) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", ""
	}
	return strings.TrimSuffix(fields[0], ":"), strings.Trim(fields[1], `"`)
}

// yarnDescriptors is a yarn.lock's descriptors, and each package's version when the
// lock holds only one, which answers for a range written differently than the lock
// wrote it.
type yarnDescriptors struct {
	exact  map[string]string // "name@range" -> version
	unique map[string]string // name -> its version, "" when the lock holds several
}

func newYarnDescriptors(exact map[string]string) yarnDescriptors {
	d := yarnDescriptors{exact: exact, unique: map[string]string{}}
	for key, v := range exact {
		i := strings.LastIndex(key, "@")
		if i <= 0 {
			continue
		}
		name := key[:i]
		if have, ok := d.unique[name]; ok && have != v {
			v = ""
		}
		d.unique[name] = v
	}
	return d
}

// version finds the locked version of name for the declared range.
//
// Implements: REQ-JS-008
func (d yarnDescriptors) version(name, versionRange string) string {
	for _, key := range []string{name + "@" + versionRange, name + "@npm:" + versionRange} {
		if v, ok := d.exact[key]; ok {
			return v
		}
	}
	// The range is written differently: accept a version only when it is unique.
	return d.unique[name]
}

// pnpmDependencies maps names to "1.2.3" (lockfile v5) or {specifier, version} (v6+).
type pnpmDependencies struct {
	Dependencies         map[string]any `yaml:"dependencies"`
	DevDependencies      map[string]any `yaml:"devDependencies"`
	OptionalDependencies map[string]any `yaml:"optionalDependencies"`
}

type pnpmLock struct {
	pnpmDependencies `yaml:",inline"`            // v5 lists the root's dependencies at the top level
	Importers        map[string]pnpmDependencies `yaml:"importers"`
	// The dependency edges. v5 to v8 keep them under "packages", v9 moved them to
	// "snapshots"; both key entries by name and version.
	Packages  map[string]pnpmPackage `yaml:"packages"`
	Snapshots map[string]struct {
		Dependencies         map[string]string `yaml:"dependencies"`
		OptionalDependencies map[string]string `yaml:"optionalDependencies"`
	} `yaml:"snapshots"`
}

// pnpmPackage is one entry of pnpm-lock.yaml's "packages".
type pnpmPackage struct {
	// Set where the key does not say: v5 and v6 for a package from outside the
	// registry, whose key is where it came from.
	Name, Version        string
	Dependencies         map[string]string `yaml:"dependencies"`
	OptionalDependencies map[string]string `yaml:"optionalDependencies"`
	// The platforms the package installs on, from its manifest.
	OS   platformList `yaml:"os"`
	CPU  platformList `yaml:"cpu"`
	Libc platformList `yaml:"libc"`
	// Where it was fetched from: {commit, repo, type: git} for a git repository,
	// {tarball} for an archive, which GitHub's are of a commit.
	Resolution struct{ Commit, Repo, Type, Tarball string } `yaml:"resolution"`
}

// git reads the entry as a git dependency: the repository (without
// credentials) and the commit it was fetched at.
//
// Implements: REQ-JS-019
func (p pnpmPackage) git() (repository, commit string, ok bool) {
	if p.Resolution.Type == "git" && p.Resolution.Repo != "" {
		repository, _, ok = gitSource("git+" + strings.TrimPrefix(p.Resolution.Repo, "git+"))
		if strings.HasPrefix(p.Resolution.Repo, "git@") { // scp form: no scheme to add
			repository, ok = p.Resolution.Repo, true
		}
		if lang.Commit(p.Resolution.Commit) {
			commit = strings.ToLower(p.Resolution.Commit)
		}
		return repository, commit, ok
	}
	return gitSource(p.Resolution.Tarball)
}

// keyed is the package a "packages" or "snapshots" key names, at the version the
// lock installs, and for a git dependency the repository it came from, the
// version being the commit.
func (lock *pnpmLock) keyed(key string) (name, version, repository string) {
	name, version = pnpmKey(key)
	p, found := lock.Packages[key]
	if found && p.Name != "" { // v5 spells them out instead of packing them into the key
		name, version = p.Name, p.Version
	}
	if repository, commit, ok := p.git(); found && ok {
		return name, commit, repository
	}
	return name, version, ""
}

// gitReference reads a dependency reference as a git dependency: the package
// and commit it installs, and the repository. A reference names the entry of
// "packages" it installs: its key (v5, v6), or the key without the name (v9).
func (lock *pnpmLock) gitReference(alias, reference string) (name, commit, repository string, ok bool) {
	reference, _, _ = strings.Cut(reference, "(")
	for _, key := range []string{reference, "/" + reference, alias + "@" + reference} {
		if _, found := lock.Packages[key]; found {
			name, commit, repository = lock.keyed(key)
			return name, commit, repository, repository != ""
		}
	}
	repository, commit, ok = gitSource(reference)
	return alias, commit, repository, ok
}

// readPnpmLock decodes a pnpm-lock.yaml. Recent pnpm (vite locks with pnpm 12)
// may write a document of its own before the project's: the lock of the package
// manager itself and of its config dependencies, whose packages are none of the
// project's. The project's lock is the last document.
//
// Implements: REQ-JS-009
func readPnpmLock(data []byte) (*pnpmLock, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	lock := &pnpmLock{}
	for {
		var document pnpmLock
		switch err := decoder.Decode(&document); {
		case errors.Is(err, io.EOF):
			return lock, nil
		case err != nil:
			return nil, err
		}
		lock = &document
	}
}

// versions returns versions per importer (a directory relative to the lockfile,
// "." for the root) from pnpm-lock.yaml v5–v9.
//
// Implements: REQ-JS-009
func (lock *pnpmLock) versions() map[string]map[string]string {
	out := map[string]map[string]string{}
	collect := func(section pnpmDependencies) map[string]string {
		m := map[string]string{}
		for _, d := range []map[string]any{section.OptionalDependencies, section.DevDependencies, section.Dependencies} {
			for name, v := range d {
				// Implements: REQ-JS-019
				if _, commit, _, git := lock.gitReference(name, pnpmReference(v)); git {
					if commit != "" {
						m[name] = commit
					}
				} else if version := pnpmVersion(pnpmReference(v)); version != "" {
					m[name] = version
				}
			}
		}
		return m
	}
	if len(lock.Importers) == 0 {
		out["."] = collect(lock.pnpmDependencies)
	}
	for importer, section := range lock.Importers {
		out[path.Clean(importer)] = collect(section)
	}
	return out
}

// pnpmReference reads a dependency's reference: v5 writes it as the value, v6 on as
// the "version" of a {specifier, version} map.
func pnpmReference(v any) string {
	switch v := v.(type) {
	case string:
		return v
	case map[string]any:
		s, _ := v["version"].(string)
		return s
	}
	return ""
}

// pnpmVersion is the version a dependency reference installs: "1.2.3(react@18.2.0)"
// and v5's "1.2.3_react@18.2.0" carry peer-dependency context, an alias names the
// real package ("c@2.0.0", "/c@2.0.0", v5 "/c/2.0.0"), and "link:../x" is local.
func pnpmVersion(reference string) string {
	reference, _, _ = strings.Cut(reference, "(")
	switch {
	case reference == "" || strings.HasPrefix(reference, "link:"):
		return ""
	case startsWithDigit(reference):
		reference, _, _ = strings.Cut(reference, "_")
	default:
		if _, v := pnpmKey(reference); startsWithDigit(v) {
			return v
		}
	}
	return reference
}

// packageLock is package-lock.json (or npm-shrinkwrap.json), v1 through v3.
type packageLock struct {
	// v2 and v3 list every installed path under "packages".
	Packages map[string]lockPath
	// v1 nests them under "dependencies", with "requires" for the edges.
	Dependencies map[string]*lockV1
}

// lockPath is one install path of package-lock.json v2/v3: "node_modules/a",
// "node_modules/a/node_modules/b", or a workspace directory.
type lockPath struct {
	Name                 string // the real package's, where the path is an alias
	Version              string
	Resolved             string // where it was fetched from: a tarball, or a git repository at a commit
	Link                 bool   // a symlink to a workspace (or file:) directory
	Dependencies         map[string]string
	OptionalDependencies map[string]string
	// npm 7 onwards installs peer dependencies, and records them here; an optional
	// one is installed only when something else needs it.
	PeerDependencies     map[string]string
	PeerDependenciesMeta map[string]struct{ Optional bool }
	// The platforms the package installs on, copied from its manifest: set on
	// the binaries a package ships per platform, which the lock lists for every
	// platform whichever one it was written on.
	OS, CPU, Libc platformList
}

// lockV1 is one package-lock.json v1 dependency, and those installed inside it.
type lockV1 struct {
	Version      string // a git dependency's is its repository at a commit, as Resolved
	Resolved     string
	Optional     bool
	Requires     map[string]string
	Dependencies map[string]*lockV1
}

// pinGit turns every git dependency of the lock into its commit: the version of
// its path (v2 and v3 write the repository's package.json version there, and the
// repository and commit as "resolved"; v1 writes them as the version), and the
// repository it came from, recorded in t. It returns the names of those the lock
// names no commit for, which are not pinned.
//
// Implements: REQ-JS-019
func (lock *packageLock) pinGit(t *tree) (unpinned []string) {
	note := func(name string) {
		if !slices.Contains(unpinned, name) {
			unpinned = append(unpinned, name)
		}
	}
	for key, p := range lock.Packages {
		alias := lockName(key)
		if alias == "" || p.Link {
			continue
		}
		repository, commit, ok := gitSource(cmp.Or(p.Resolved, p.Version))
		if !ok {
			continue
		}
		p.Version = commit
		lock.Packages[key] = p
		t.addGit([]string{cmp.Or(p.Name, alias), alias}, commit, repository)
		if commit == "" {
			note(cmp.Or(p.Name, alias))
		}
	}
	var walk func(map[string]*lockV1)
	walk = func(dependencies map[string]*lockV1) {
		for name, d := range dependencies {
			if d == nil {
				continue
			}
			if repository, commit, ok := gitSource(cmp.Or(d.Version, d.Resolved)); ok {
				d.Version = commit
				t.addGit([]string{name}, commit, repository)
				if commit == "" {
					note(name)
				}
			}
			walk(d.Dependencies)
		}
	}
	walk(lock.Dependencies)
	slices.Sort(unpinned)
	return unpinned
}

// versions returns the top-level package versions.
//
// Implements: REQ-JS-007
func (lock *packageLock) versions() map[string]string {
	out := map[string]string{}
	for k, v := range lock.Dependencies {
		if v == nil {
			continue
		}
		out[k] = v.Version
		if real, ok := strings.CutPrefix(v.Version, "npm:"); ok { // an alias, "npm:real@1.2.3"
			if _, version := splitIdentifier(real); version != "" {
				out[k] = version
			}
		}
	}
	for k, v := range lock.Packages {
		name, ok := strings.CutPrefix(k, "node_modules/")
		if ok && !strings.Contains(name, "/node_modules/") {
			out[name] = v.Version
		}
	}
	return out
}

// Packages is what a project's package.json and lock files say about npm
// packages, for plugins of languages whose sources import them too: a Hardhat
// project's Solidity reads @openzeppelin/contracts from node_modules as Node.js
// would, and its imports belong on the npm package JavaScript imports.
//
// Implements: REQ-SOLIDITY-008
type Packages struct{ r *resolver }

// ReadPackages reads the package.json and lock files (npm-shrinkwrap.json,
// package-lock.json, pnpm-lock.yaml, yarn.lock, bun.lock) of the project.
func ReadPackages(all []*scan.File) *Packages { return &Packages{r: newResolver(all)} }

// Package resolves a bare specifier ("@scope/name/sub/path") that file
// imports: to the file or directory of a workspace package the project builds,
// or to the npm package a package.json above the file declares, pinned by the
// nearest lock file as JavaScript's imports are. ok is false when nothing
// declares the package.
func (p *Packages) Package(spec, file string) (lang.Target, bool) {
	packageName, subpath := splitPackage(spec)
	if packageName == "" {
		return lang.Target{}, false
	}
	if directory, ok := p.r.byName[packageName]; ok {
		if subpath != "" {
			if t, ok := p.r.probe(path.Join(directory, subpath)); ok {
				return t, true
			}
		}
		return lang.Target{Local: directory}, true
	}
	t, declared := p.r.declared(packageName, path.Dir(file))
	if !declared {
		return lang.Target{}, false
	}
	t.Ecosystem, t.Package = ecosystemNPM, packageName
	t.Origin = p.r.gitOrigin(t)
	return t, true
}

// Dependencies is the lock files' answer for an npm package, as the JavaScript
// resolver gives it.
func (p *Packages) Dependencies(t lang.Target) []lang.Target { return p.r.Dependencies(t) }

// PackageName splits an npm specifier into its package name: "@scope/name"
// or "name".
func PackageName(spec string) string {
	packageName, _ := splitPackage(spec)
	return packageName
}

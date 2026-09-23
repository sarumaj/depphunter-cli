package javascript

import (
	"encoding/json"
	"os"
	"path"
	"strings"

	"github.com/tidwall/jsonc"
	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

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

// Extensions tried, in order, for extension-less specifiers (TypeScript's order first).
var probeExts = []string{".ts", ".tsx", ".d.ts", ".js", ".jsx", ".mjs", ".cjs", ".mts", ".cts", ".json"}

type resolver struct {
	files   map[string]bool
	dirs    map[string]bool
	deps    map[string]map[string]string // package.json dir -> dependency -> version range
	locks   map[string]map[string]string // package-lock.json dir -> package -> exact version
	byName  map[string]string            // workspace package name -> its directory
	configs map[string]*tsconfig         // directory -> effective tsconfig/jsconfig
	tree    *tree                        // what the lock files say the packages need
}

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
		files: map[string]bool{}, dirs: map[string]bool{}, deps: map[string]map[string]string{},
		locks: map[string]map[string]string{}, byName: map[string]string{}, configs: map[string]*tsconfig{},
		tree: newTree(),
	}
	byPath := map[string]*scan.File{}
	for _, f := range all {
		r.files[f.Path] = true
		byPath[f.Path] = f
		for d := path.Dir(f.Path); d != "." && !r.dirs[d]; d = path.Dir(d) {
			r.dirs[d] = true
		}
	}
	yarn := map[string]yarnDescriptors{} // yarn.lock dir -> its descriptors
	for _, f := range all {
		dir := path.Dir(f.Path)
		switch path.Base(f.Path) {
		case "package.json":
			var pj struct {
				Name                                                                  string
				Dependencies, DevDependencies, PeerDependencies, OptionalDependencies map[string]string
			}
			if readJSON(f.Abs, &pj) != nil {
				continue
			}
			deps := map[string]string{}
			for _, m := range []map[string]string{pj.OptionalDependencies, pj.PeerDependencies, pj.DevDependencies, pj.Dependencies} {
				for k, v := range m {
					deps[k] = v
				}
			}
			r.deps[dir] = deps
			if pj.Name != "" {
				r.byName[pj.Name] = dir
			}
		// One decoding of each lock file serves both the versions the project's
		// imports pin and the tree the transitive walk follows: a monorepo's
		// pnpm-lock.yaml runs to megabytes of YAML.
		case "package-lock.json":
			var lock packageLock
			if readJSON(f.Abs, &lock) == nil {
				r.addLock(dir, lock.versions())
				r.tree.addPackageLockTree(&lock)
			}
		case "yarn.lock":
			if data, err := os.ReadFile(f.Abs); err == nil {
				yarn[dir] = newYarnDescriptors(readYarnLock(data))
				r.tree.addYarnTree(data)
			}
		case "pnpm-lock.yaml":
			data, err := os.ReadFile(f.Abs)
			var lock pnpmLock
			if err != nil || yaml.Unmarshal(data, &lock) != nil {
				continue
			}
			for importer, versions := range lock.versions() {
				r.addLock(path.Join(dir, importer), versions)
			}
			r.tree.addPnpmTree(&lock)
		}
	}
	// yarn.lock keys are "name@range": pin each declared range of the packages below.
	for lockDir, descriptors := range yarn {
		for pkgDir, deps := range r.deps {
			if lockDir != "." && pkgDir != lockDir && !strings.HasPrefix(pkgDir, lockDir+"/") {
				continue
			}
			pinned := map[string]string{}
			for name, rng := range deps {
				if v := descriptors.version(name, rng); v != "" {
					pinned[name] = v
				}
			}
			r.addLock(pkgDir, pinned)
		}
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

func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	return r.resolve(imp.Module, file)
}

func (r *resolver) resolve(spec, from string) lang.Target {
	dir := path.Dir(from)
	switch {
	case spec == "" || strings.Contains(spec, "://") || strings.HasPrefix(spec, "data:") || strings.HasPrefix(spec, "/"):
		return lang.Target{}
	case spec == "." || spec == ".." || strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../"):
		t, _ := r.probe(path.Join(dir, spec))
		return t // unresolvable relative imports point outside the project: drop them
	}
	if c := r.nearestConfig(dir); c != nil {
		if t, ok := r.viaConfig(c, spec); ok {
			return t
		}
	}
	if name, ok := strings.CutPrefix(spec, "node:"); ok {
		pkg, _ := splitPackage(name)
		return lang.Target{Ecosystem: ecoNode, Package: pkg}
	}
	pkg, sub := splitPackage(spec)
	if builtins[pkg] {
		return lang.Target{Ecosystem: ecoNode, Package: pkg}
	}
	if packageDir, ok := r.byName[pkg]; ok {
		if sub != "" {
			if t, ok := r.probe(path.Join(packageDir, sub)); ok {
				return t
			}
		}
		return lang.Target{Local: packageDir}
	}
	// Bundler aliases and package.json "imports" we cannot see: not an npm package.
	if strings.HasPrefix(spec, "~") || strings.HasPrefix(spec, "#") || strings.HasPrefix(spec, "@/") {
		return lang.Target{}
	}
	t, declared := r.declared(pkg, dir)
	t.Ecosystem, t.Package, t.Unresolved = ecoNPM, pkg, !declared
	return t
}

// splitPackage splits "@scope/name/sub/path" into "@scope/name" and "sub/path".
func splitPackage(spec string) (string, string) {
	parts := strings.SplitN(spec, "/", 3)
	if strings.HasPrefix(spec, "@") && len(parts) >= 2 {
		pkg := parts[0] + "/" + parts[1]
		if len(parts) == 3 {
			return pkg, parts[2]
		}
		return pkg, ""
	}
	pkg, sub, _ := strings.Cut(spec, "/")
	return pkg, sub
}

// probe finds the file or directory a module path refers to.
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
	switch ext := path.Ext(p); ext {
	case ".js", ".jsx", ".mjs", ".cjs":
		bases = append(bases, strings.TrimSuffix(p, ext))
	}
	for _, b := range bases {
		for _, ext := range probeExts {
			if r.files[b+ext] {
				return lang.Target{Local: b + ext}, true
			}
		}
	}
	for _, ext := range probeExts {
		if idx := path.Join(p, "index"+ext); r.files[idx] {
			return lang.Target{Local: idx}, true
		}
	}
	if r.dirs[p] {
		return lang.Target{Local: p}, true
	}
	return lang.Target{}, false
}

func (r *resolver) nearestConfig(dir string) *tsconfig {
	for d := dir; ; d = path.Dir(d) {
		if c := r.configs[d]; c != nil {
			return c
		}
		if d == "." {
			return nil
		}
	}
}

func (r *resolver) viaConfig(c *tsconfig, spec string) (lang.Target, bool) {
	best, bestLen := (*pathRule)(nil), -1
	var star string
	for i := range c.paths {
		rule := &c.paths[i]
		prefix, suffix, wild := strings.Cut(rule.pattern, "*")
		switch {
		case !wild && spec == rule.pattern && len(prefix) > bestLen:
			best, bestLen, star = rule, len(prefix), ""
		case wild && strings.HasPrefix(spec, prefix) && strings.HasSuffix(spec, suffix) &&
			len(spec) >= len(prefix)+len(suffix) && len(prefix) > bestLen:
			best, bestLen, star = rule, len(prefix), spec[len(prefix):len(spec)-len(suffix)]
		}
	}
	if best != nil {
		for _, t := range best.targets {
			if res, ok := r.probe(strings.Replace(t, "*", star, 1)); ok {
				return res, true
			}
		}
	}
	if c.baseURL != "" {
		return r.probe(path.Join(c.baseURL, spec))
	}
	return lang.Target{}, false
}

// declared looks up pkg in the nearest package.json files that declare it, preferring
// the exact version from the nearest package-lock.json: the range in package.json is
// what was asked for, the lock is what is installed.
func (r *resolver) declared(pkg, dir string) (lang.Target, bool) {
	for d := dir; ; d = path.Dir(d) {
		if v, ok := r.deps[d][pkg]; ok {
			for l := d; ; l = path.Dir(l) {
				if exact := r.locks[l][pkg]; exact != "" {
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

func loadTSConfig(rel string, files map[string]*scan.File, seen map[string]bool) *tsconfig {
	f := files[rel]
	if f == nil || seen[rel] {
		return nil
	}
	seen[rel] = true
	data, err := os.ReadFile(f.Abs)
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
	if json.Unmarshal(jsonc.ToJSON(data), &raw) != nil { // tsconfig allows comments and trailing commas
		return nil
	}
	dir := path.Dir(rel)

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
		pp := path.Join(dir, p)
		if !strings.HasSuffix(pp, ".json") {
			pp += ".json"
		}
		if pc := loadTSConfig(pp, files, seen); pc != nil {
			*c = *pc
		}
	}
	if b := raw.CompilerOptions.BaseURL; b != nil {
		c.baseURL = path.Join(dir, *b)
	}
	if raw.CompilerOptions.Paths != nil {
		base := dir
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

func readJSON(abs string, v any) error {
	data, err := os.ReadFile(abs)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func (r *resolver) addLock(dir string, versions map[string]string) {
	if len(versions) == 0 {
		return
	}
	if r.locks[dir] == nil {
		r.locks[dir] = map[string]string{}
	}
	for k, v := range versions {
		if _, ok := r.locks[dir][k]; !ok {
			r.locks[dir][k] = v
		}
	}
}

// readYarnLock maps "name@range" descriptors to versions. Both the classic format
// (`"a@^1", a@^1.2:` / `  version "1.2.3"`) and Berry's YAML (`"a@npm:^1":` /
// `  version: 1.2.3`) are line-oriented enough to read the same way.
func readYarnLock(data []byte) map[string]string {
	out := map[string]string{}
	var keys []string
	for _, line := range strings.Split(string(data), "\n") {
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case !strings.HasPrefix(line, " ") && strings.HasSuffix(line, ":"):
			keys = keys[:0]
			for _, k := range strings.Split(strings.TrimSuffix(line, ":"), ",") {
				keys = append(keys, strings.Trim(strings.TrimSpace(k), `"`))
			}
		// The key has to be "version" itself: a dependency named version-guard, one
		// level deeper inside the entry, starts with the same letters.
		case !strings.HasPrefix(line, "    "):
			if k, v := yarnField(line); k == "version" && v != "" {
				for _, key := range keys {
					out[key] = v
				}
				keys = keys[:0]
			}
		}
	}
	return out
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
func (d yarnDescriptors) version(name, rng string) string {
	for _, key := range []string{name + "@" + rng, name + "@npm:" + rng} {
		if v, ok := d.exact[key]; ok {
			return v
		}
	}
	// The range is written differently: accept a version only when it is unique.
	return d.unique[name]
}

// pnpmDeps maps names to "1.2.3" (lockfile v5) or {specifier, version} (v6+).
type pnpmDeps struct {
	Dependencies         map[string]any `yaml:"dependencies"`
	DevDependencies      map[string]any `yaml:"devDependencies"`
	OptionalDependencies map[string]any `yaml:"optionalDependencies"`
}

type pnpmLock struct {
	pnpmDeps  `yaml:",inline"`    // v5 lists the root's dependencies at the top level
	Importers map[string]pnpmDeps `yaml:"importers"`
	// The dependency edges. v5 to v8 keep them under "packages", v9 moved them to
	// "snapshots"; both key entries by name and version.
	Packages map[string]struct {
		Name, Version string
		Dependencies  map[string]string `yaml:"dependencies"`
	} `yaml:"packages"`
	Snapshots map[string]struct {
		Dependencies map[string]string `yaml:"dependencies"`
	} `yaml:"snapshots"`
}

// versions returns versions per importer (a directory relative to the lockfile,
// "." for the root) from pnpm-lock.yaml v5–v9.
func (lock *pnpmLock) versions() map[string]map[string]string {
	out := map[string]map[string]string{}
	collect := func(sec pnpmDeps) map[string]string {
		m := map[string]string{}
		for _, d := range []map[string]any{sec.OptionalDependencies, sec.DevDependencies, sec.Dependencies} {
			for name, v := range d {
				var version string
				switch v := v.(type) {
				case string:
					version = v
				case map[string]any:
					version, _ = v["version"].(string)
				}
				// "1.2.3(react@18.2.0)" carries peer-dependency context; "link:../x" is local.
				version, _, _ = strings.Cut(version, "(")
				if version != "" && !strings.HasPrefix(version, "link:") {
					m[name] = version
				}
			}
		}
		return m
	}
	if len(lock.Importers) == 0 {
		out["."] = collect(lock.pnpmDeps)
	}
	for importer, sec := range lock.Importers {
		out[path.Clean(importer)] = collect(sec)
	}
	return out
}

// packageLock is package-lock.json, v1 through v3.
type packageLock struct {
	// v2 and v3 list every installed path under "packages".
	Packages map[string]struct {
		Version              string
		Dependencies         map[string]string
		OptionalDependencies map[string]string
	}
	// v1 nests them under "dependencies", with "requires" for the edges.
	Dependencies map[string]struct {
		Version  string
		Requires map[string]string
	}
}

// versions returns the top-level package versions.
func (lock *packageLock) versions() map[string]string {
	out := map[string]string{}
	for k, v := range lock.Dependencies {
		out[k] = v.Version
	}
	for k, v := range lock.Packages {
		name, ok := strings.CutPrefix(k, "node_modules/")
		if ok && !strings.Contains(name, "/node_modules/") {
			out[name] = v.Version
		}
	}
	return out
}

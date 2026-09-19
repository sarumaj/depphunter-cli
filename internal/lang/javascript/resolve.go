package javascript

import (
	"encoding/json"
	"os"
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

var builtins = map[string]bool{}

func init() {
	for _, m := range strings.Fields(`assert async_hooks buffer child_process cluster console constants crypto
		dgram diagnostics_channel dns domain events fs http http2 https inspector module net os path
		perf_hooks process punycode querystring readline repl stream string_decoder sys timers tls
		trace_events tty url util v8 vm wasi worker_threads zlib`) {
		builtins[m] = true
	}
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
	}
	byPath := map[string]*scan.File{}
	for _, f := range all {
		r.files[f.Path] = true
		byPath[f.Path] = f
		for d := path.Dir(f.Path); d != "." && !r.dirs[d]; d = path.Dir(d) {
			r.dirs[d] = true
		}
	}
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
		case "package-lock.json":
			r.locks[dir] = readLock(f.Abs)
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
	if pdir, ok := r.byName[pkg]; ok {
		if sub != "" {
			if t, ok := r.probe(path.Join(pdir, sub)); ok {
				return t
			}
		}
		return lang.Target{Local: pdir}
	}
	// Bundler aliases and package.json "imports" we cannot see: not an npm package.
	if strings.HasPrefix(spec, "~") || strings.HasPrefix(spec, "#") || strings.HasPrefix(spec, "@/") {
		return lang.Target{}
	}
	version, declared := r.declared(pkg, dir)
	return lang.Target{Ecosystem: ecoNPM, Package: pkg, Version: version, Unresolved: !declared}
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
// the exact version from the nearest package-lock.json.
func (r *resolver) declared(pkg, dir string) (string, bool) {
	for d := dir; ; d = path.Dir(d) {
		if v, ok := r.deps[d][pkg]; ok {
			for l := d; ; l = path.Dir(l) {
				if exact := r.locks[l][pkg]; exact != "" {
					return exact, true
				}
				if l == "." {
					break
				}
			}
			return v, true
		}
		if d == "." {
			return "", false
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
	if json.Unmarshal(stripJSONC(data), &raw) != nil {
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

// readLock returns top-level package versions from package-lock.json (v1–v3).
func readLock(abs string) map[string]string {
	var lock struct {
		Packages     map[string]struct{ Version string }
		Dependencies map[string]struct{ Version string }
	}
	out := map[string]string{}
	if readJSON(abs, &lock) != nil {
		return out
	}
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

// stripJSONC removes comments and trailing commas, which tsconfig files allow.
func stripJSONC(in []byte) []byte {
	out := make([]byte, 0, len(in))
	inString := false
	for i := 0; i < len(in); i++ {
		c := in[i]
		switch {
		case inString:
			out = append(out, c)
			if c == '\\' && i+1 < len(in) {
				i++
				out = append(out, in[i])
			} else if c == '"' {
				inString = false
			}
		case c == '"':
			inString = true
			out = append(out, c)
		case c == '/' && i+1 < len(in) && in[i+1] == '/':
			for i < len(in) && in[i] != '\n' {
				i++
			}
			out = append(out, '\n')
		case c == '/' && i+1 < len(in) && in[i+1] == '*':
			i += 2
			for i+1 < len(in) && !(in[i] == '*' && in[i+1] == '/') {
				i++
			}
			i++
		case c == ',':
			j := i + 1
			for j < len(in) && (in[j] == ' ' || in[j] == '\t' || in[j] == '\n' || in[j] == '\r') {
				j++
			}
			if j < len(in) && (in[j] == '}' || in[j] == ']') {
				continue
			}
			out = append(out, c)
		default:
			out = append(out, c)
		}
	}
	return out
}

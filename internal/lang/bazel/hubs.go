package bazel

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
	"golang.org/x/mod/modfile"
	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/starlark"
)

// ---------------------------------------------------------------- Maven

// normMaven is how rules_jvm_external names an artifact's target in its hub:
// com.google.guava:guava -> com_google_guava_guava.
func normMaven(s string) string {
	b := []byte(strings.ToLower(s))
	for i, c := range b {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
			b[i] = '_'
		}
	}
	return string(b)
}

// mavenTarget is the Maven package of a coordinate, group:artifact (the Clojure
// plugin's naming, which the index and OSV address), with its version:
// group:artifact[:packaging[:classifier]]:version.
//
// Implements: REQ-BAZEL-009
func mavenTarget(coord string) (lang.Target, bool) {
	parts := strings.Split(strings.TrimSpace(coord), ":")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return lang.Target{}, false
	}
	t := lang.Target{Ecosystem: ecoMaven, Package: parts[0] + ":" + parts[1]}
	if len(parts) >= 3 {
		t.Version = parts[len(parts)-1]
		if len(parts) == 3 && strings.Contains(t.Version, "@") { // g:a:v@packaging
			t.Version = t.Version[:strings.Index(t.Version, "@")]
		}
	}
	t.Pinned = lang.PinnedMaven(t.Version)
	t.Floating = t.Version != "" && !t.Pinned
	return t, true
}

func (h *hub) addMaven(coord string) {
	if t, ok := mavenTarget(coord); ok {
		h.add(normMaven(t.Package), t, false)
	}
}

// mavenInstall reads maven_install's or maven.install's artifacts and the
// pinned lock file (maven_install.json) its lockAttr names.
func (r *resolver) mavenInstall(w *workspace, h *hub, n *starlark.Node, lockAttr string) {
	for _, it := range listItems(n.Kw("artifacts")) {
		if s, ok := it.Str(); ok {
			h.addMaven(s)
		} else if it.Kind == starlark.Call {
			if c := coordinate(it); c != "" {
				h.addMaven(c)
			}
		}
	}
	if p := w.labelPath(n.KwStr(lockAttr)); p != "" {
		if src, ok := r.read(p); ok {
			readMavenLock(h, src)
		}
	}
}

// readMavenLock reads rules_jvm_external's lock file: version 2 ("artifacts"
// keyed group:artifact[:packaging:classifier] with a version, "dependencies" as
// lists of keys) or the older dependency_tree of full coordinates.
func readMavenLock(h *hub, src []byte) {
	var doc struct {
		Artifacts map[string]struct {
			Version string `json:"version"`
		} `json:"artifacts"`
		Dependencies   map[string][]string `json:"dependencies"`
		DependencyTree struct {
			Dependencies []struct {
				Coord        string   `json:"coord"`
				Dependencies []string `json:"dependencies"`
			} `json:"dependencies"`
		} `json:"dependency_tree"`
	}
	if json.Unmarshal(src, &doc) != nil {
		return
	}
	ga := func(key string) string {
		parts := strings.Split(key, ":")
		if len(parts) < 2 {
			return ""
		}
		return parts[0] + ":" + parts[1]
	}
	for _, key := range sortedKeys(doc.Artifacts) {
		v := doc.Artifacts[key].Version
		if name := ga(key); name != "" && v != "" {
			h.add(normMaven(name), lang.Target{Ecosystem: ecoMaven, Package: name, Version: v, Pinned: true}, true)
		}
	}
	for _, key := range sortedKeys(doc.Dependencies) {
		from := ga(key)
		for _, d := range doc.Dependencies[key] {
			if to := ga(d); from != "" && to != "" && to != from {
				h.deps[from] = appendNew(h.deps[from], to)
			}
		}
	}
	for _, d := range doc.DependencyTree.Dependencies {
		t, ok := mavenTarget(d.Coord)
		if !ok || t.Version == "" {
			continue
		}
		t.Pinned, t.Floating = true, false
		h.add(normMaven(t.Package), t, true)
		for _, dep := range d.Dependencies {
			if dt, ok := mavenTarget(dep); ok && dt.Package != t.Package {
				h.deps[t.Package] = appendNew(h.deps[t.Package], dt.Package)
			}
		}
	}
}

func appendNew(list []string, s string) []string {
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}

// ---------------------------------------------------------------- PyPI

// normPy is a distribution name as rules_python's hub labels spell it: PEP 503
// normalized with underscores (PyYAML -> pyyaml, typing-extensions ->
// typing_extensions).
func normPy(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.NewReplacer("-", "_", ".", "_").Replace(s)
}

// pipParse reads the requirements lock files pip.parse or pip_parse names.
func (r *resolver) pipParse(w *workspace, h *hub, n *starlark.Node) {
	var labels []string
	for _, attr := range []string{"requirements_lock", "requirements", "requirements_linux", "requirements_darwin", "requirements_windows"} {
		if s := n.KwStr(attr); s != "" {
			labels = append(labels, s)
		}
	}
	if d := n.Kw("requirements_by_platform"); d != nil && d.Kind == starlark.Dict {
		for i := 0; i < len(d.Items); i += 2 {
			if s, ok := d.Items[i].Str(); ok {
				labels = append(labels, s)
			}
		}
	}
	for _, l := range labels {
		if p := w.labelPath(l); p != "" {
			if src, ok := r.read(p); ok {
				readRequirements(h, src)
			}
		}
	}
}

var requirement = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9._-]*)\s*(?:\[[^\]]*\])?\s*(===?|>=|~=|<=|!=|<|>)?\s*([^\s;\\#]*)`)

// readRequirements reads a requirements file: name==version lines (pip-compile's
// locks), with hashes, markers and options skipped.
func readRequirements(h *hub, src []byte) {
	for _, line := range strings.Split(string(src), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
			continue
		}
		m := requirement.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		t := lang.Target{Ecosystem: ecoPyPI, Package: m[1]}
		if m[2] == "==" || m[2] == "===" {
			t.Version, t.Pinned = m[3], lang.Pinned(m[3])
		} else if m[2] != "" {
			t.Version = m[2] + m[3]
		}
		h.add(normPy(m[1]), t, true)
	}
}

// ---------------------------------------------------------------- Go

// readGoMod reads a go.mod's requirements (go_deps.from_file). Go's minimal
// version selection makes the listed version the one built, so it pins.
func readGoMod(src []byte) []lang.Target {
	f, err := modfile.ParseLax("go.mod", src, nil)
	if err != nil {
		return nil
	}
	var out []lang.Target
	for _, req := range f.Require {
		out = append(out, lang.Target{Ecosystem: ecoGo, Package: req.Mod.Path, Version: req.Mod.Version, Pinned: lang.Pinned(req.Mod.Version)})
	}
	return out
}

// ---------------------------------------------------------------- npm

// pnpm reads the versions a pnpm-lock.yaml resolved for every importer's direct
// dependencies (rules_js' npm_translate_lock).
func (r *resolver) pnpm(w *workspace, h *hub, lockLabel string) {
	p := w.labelPath(lockLabel)
	if p == "" {
		return
	}
	src, ok := r.read(p)
	if !ok {
		return
	}
	type deps = map[string]any
	var doc struct {
		Dependencies    deps `yaml:"dependencies"`
		DevDependencies deps `yaml:"devDependencies"`
		Importers       map[string]struct {
			Dependencies         deps `yaml:"dependencies"`
			DevDependencies      deps `yaml:"devDependencies"`
			OptionalDependencies deps `yaml:"optionalDependencies"`
		} `yaml:"importers"`
	}
	if yaml.Unmarshal(src, &doc) != nil {
		return
	}
	all := []deps{doc.Dependencies, doc.DevDependencies}
	for _, k := range sortedKeys(doc.Importers) {
		i := doc.Importers[k]
		all = append(all, i.Dependencies, i.DevDependencies, i.OptionalDependencies)
	}
	for _, d := range all {
		for _, name := range sortedKeys(d) {
			v := ""
			switch val := d[name].(type) {
			case string:
				v = val
			case map[string]any:
				v, _ = val["version"].(string)
			}
			// 1.2.3(react@18.2.0) in lock v6+, 1.2.3_react@18.2.0 before; link:
			// and workspace: versions are the project's own packages.
			if i := strings.IndexAny(v, "(_"); i > 0 {
				v = v[:i]
			}
			if strings.Contains(v, ":") || v == "" {
				continue
			}
			h.add(name, lang.Target{Ecosystem: ecoNPM, Package: name, Version: v, Pinned: lang.PinnedSemver(v)}, true)
		}
	}
}

// ---------------------------------------------------------------- crates.io

func normCrate(s string) string { return strings.ReplaceAll(strings.ToLower(s), "-", "_") }

// exactCargo reports whether a Cargo requirement names one version: "=1.2.3".
// A bare "1.2.3" is a caret range in Cargo.
func exactCargo(v string) bool {
	x, ok := strings.CutPrefix(strings.TrimSpace(v), "=")
	return ok && lang.PinnedSemver(strings.TrimSpace(x))
}

// cargo reads the Cargo.lock crate_universe names (cargo_lockfile), and the
// packages a crates_repository declares in its packages dict.
func (r *resolver) cargo(w *workspace, h *hub, n *starlark.Node) {
	if d := n.Kw("packages"); d != nil && d.Kind == starlark.Dict {
		for i := 0; i+1 < len(d.Items); i += 2 {
			name, ok := d.Items[i].Str()
			if !ok {
				continue
			}
			v := d.Items[i+1].KwStr("version")
			h.add(normCrate(name), lang.Target{Ecosystem: ecoCrates, Package: name, Version: v, Pinned: exactCargo(v)}, false)
		}
	}
	p := w.labelPath(n.KwStr("cargo_lockfile"))
	if p == "" {
		return
	}
	src, ok := r.read(p)
	if !ok {
		return
	}
	var lock struct {
		Package []struct {
			Name, Version, Source string
		} `toml:"package"`
	}
	if _, err := toml.Decode(string(src), &lock); err != nil {
		return
	}
	versions := map[string][]string{}
	var names []string
	for _, pkg := range lock.Package {
		if pkg.Source == "" {
			continue // a member of the Cargo workspace
		}
		if versions[pkg.Name] == nil {
			names = append(names, pkg.Name)
		}
		versions[pkg.Name] = append(versions[pkg.Name], pkg.Version)
	}
	for _, name := range names {
		t := lang.Target{Ecosystem: ecoCrates, Package: name}
		if vs := versions[name]; len(vs) == 1 {
			t.Version, t.Pinned = vs[0], true
		}
		h.add(normCrate(name), t, true)
	}
}

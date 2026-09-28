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

// normalizeMaven is how rules_jvm_external names an artifact's target in its hub:
// com.google.guava:guava -> com_google_guava_guava.
func normalizeMaven(s string) string {
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
func mavenTarget(coordinate string) (lang.Target, bool) {
	parts := strings.Split(strings.TrimSpace(coordinate), ":")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return lang.Target{}, false
	}
	t := lang.Target{Ecosystem: ecosystemMaven, Package: parts[0] + ":" + parts[1]}
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

func (h *hub) addMaven(coordinate string) {
	if t, ok := mavenTarget(coordinate); ok {
		h.add(normalizeMaven(t.Package), t, false)
	}
}

// mavenInstall reads maven_install's or maven.install's artifacts and the
// pinned lock file (maven_install.json) its lockAttribute names.
func (r *resolver) mavenInstall(w *workspace, h *hub, n *starlark.Node, lockAttribute string) {
	for _, it := range listItems(n.Keyword("artifacts")) {
		if s, ok := it.StringValue(); ok {
			h.addMaven(s)
		} else if it.Kind == starlark.Call {
			if c := coordinate(it); c != "" {
				h.addMaven(c)
			}
		}
	}
	if p := w.labelPath(n.KeywordString(lockAttribute)); p != "" {
		if source, ok := r.read(p); ok {
			readMavenLock(h, source)
		}
	}
}

// readMavenLock reads rules_jvm_external's lock file: version 2 ("artifacts"
// keyed group:artifact[:packaging:classifier] with a version, "dependencies" as
// lists of keys) or the older dependency_tree of full coordinates.
func readMavenLock(h *hub, source []byte) {
	var doc struct {
		Artifacts map[string]struct {
			Version string `json:"version"`
		} `json:"artifacts"`
		Dependencies   map[string][]string `json:"dependencies"`
		DependencyTree struct {
			Dependencies []struct {
				Coordinate   string   `json:"coord"`
				Dependencies []string `json:"dependencies"`
			} `json:"dependencies"`
		} `json:"dependency_tree"`
	}
	if json.Unmarshal(source, &doc) != nil {
		return
	}
	groupArtifactOf := func(key string) string {
		parts := strings.Split(key, ":")
		if len(parts) < 2 {
			return ""
		}
		return parts[0] + ":" + parts[1]
	}
	for _, key := range sortedKeys(doc.Artifacts) {
		v := doc.Artifacts[key].Version
		if name := groupArtifactOf(key); name != "" && v != "" {
			h.add(normalizeMaven(name), lang.Target{Ecosystem: ecosystemMaven, Package: name, Version: v, Pinned: true}, true)
		}
	}
	for _, key := range sortedKeys(doc.Dependencies) {
		from := groupArtifactOf(key)
		for _, d := range doc.Dependencies[key] {
			if to := groupArtifactOf(d); from != "" && to != "" && to != from {
				h.dependencies[from] = appendNew(h.dependencies[from], to)
			}
		}
	}
	for _, d := range doc.DependencyTree.Dependencies {
		t, ok := mavenTarget(d.Coordinate)
		if !ok || t.Version == "" {
			continue
		}
		t.Pinned, t.Floating = true, false
		h.add(normalizeMaven(t.Package), t, true)
		for _, dependency := range d.Dependencies {
			if dependencyTarget, ok := mavenTarget(dependency); ok && dependencyTarget.Package != t.Package {
				h.dependencies[t.Package] = appendNew(h.dependencies[t.Package], dependencyTarget.Package)
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

// normalizePy is a distribution name as rules_python's hub labels spell it: PEP 503
// normalized with underscores (PyYAML -> pyyaml, typing-extensions ->
// typing_extensions).
func normalizePy(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.NewReplacer("-", "_", ".", "_").Replace(s)
}

// pipParse reads the requirements lock files pip.parse or pip_parse names.
func (r *resolver) pipParse(w *workspace, h *hub, n *starlark.Node) {
	var labels []string
	for _, attribute := range []string{"requirements_lock", "requirements", "requirements_linux", "requirements_darwin", "requirements_windows"} {
		if s := n.KeywordString(attribute); s != "" {
			labels = append(labels, s)
		}
	}
	if d := n.Keyword("requirements_by_platform"); d != nil && d.Kind == starlark.Dictionary {
		for i := 0; i < len(d.Items); i += 2 {
			if s, ok := d.Items[i].StringValue(); ok {
				labels = append(labels, s)
			}
		}
	}
	for _, l := range labels {
		if p := w.labelPath(l); p != "" {
			if source, ok := r.read(p); ok {
				readRequirements(h, source)
			}
		}
	}
}

var requirement = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9._-]*)\s*(?:\[[^\]]*\])?\s*(===?|>=|~=|<=|!=|<|>)?\s*([^\s;\\#]*)`)

// readRequirements reads a requirements file: name==version lines (pip-compile's
// locks), with hashes, markers and options skipped.
func readRequirements(h *hub, source []byte) {
	for _, line := range strings.Split(string(source), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
			continue
		}
		m := requirement.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		t := lang.Target{Ecosystem: ecosystemPyPI, Package: m[1]}
		if m[2] == "==" || m[2] == "===" {
			t.Version, t.Pinned = m[3], lang.Pinned(m[3])
		} else if m[2] != "" {
			t.Version = m[2] + m[3]
		}
		h.add(normalizePy(m[1]), t, true)
	}
}

// ---------------------------------------------------------------- Go

// readGoMod reads a go.mod's requirements (go_deps.from_file). Go's minimal
// version selection makes the listed version the one built, so it pins.
func readGoMod(source []byte) []lang.Target {
	f, err := modfile.ParseLax("go.mod", source, nil)
	if err != nil {
		return nil
	}
	var out []lang.Target
	for _, require := range f.Require {
		out = append(out, lang.Target{Ecosystem: ecosystemGo, Package: require.Mod.Path, Version: require.Mod.Version, Pinned: lang.Pinned(require.Mod.Version)})
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
	source, ok := r.read(p)
	if !ok {
		return
	}
	type dependencyTable = map[string]any
	var doc struct {
		Dependencies    dependencyTable `yaml:"dependencies"`
		DevDependencies dependencyTable `yaml:"devDependencies"`
		Importers       map[string]struct {
			Dependencies         dependencyTable `yaml:"dependencies"`
			DevDependencies      dependencyTable `yaml:"devDependencies"`
			OptionalDependencies dependencyTable `yaml:"optionalDependencies"`
		} `yaml:"importers"`
	}
	if yaml.Unmarshal(source, &doc) != nil {
		return
	}
	all := []dependencyTable{doc.Dependencies, doc.DevDependencies}
	for _, k := range sortedKeys(doc.Importers) {
		i := doc.Importers[k]
		all = append(all, i.Dependencies, i.DevDependencies, i.OptionalDependencies)
	}
	for _, d := range all {
		for _, name := range sortedKeys(d) {
			v := ""
			switch value := d[name].(type) {
			case string:
				v = value
			case map[string]any:
				v, _ = value["version"].(string)
			}
			// 1.2.3(react@18.2.0) in lock v6+, 1.2.3_react@18.2.0 before; link:
			// and workspace: versions are the project's own packages.
			if i := strings.IndexAny(v, "(_"); i > 0 {
				v = v[:i]
			}
			if strings.Contains(v, ":") || v == "" {
				continue
			}
			h.add(name, lang.Target{Ecosystem: ecosystemNPM, Package: name, Version: v, Pinned: lang.PinnedSemver(v)}, true)
		}
	}
}

// ---------------------------------------------------------------- crates.io

func normalizeCrate(s string) string { return strings.ReplaceAll(strings.ToLower(s), "-", "_") }

// exactCargo reports whether a Cargo requirement names one version: "=1.2.3".
// A bare "1.2.3" is a caret range in Cargo.
func exactCargo(v string) bool {
	x, ok := strings.CutPrefix(strings.TrimSpace(v), "=")
	return ok && lang.PinnedSemver(strings.TrimSpace(x))
}

// cargo reads the Cargo.lock crate_universe names (cargo_lockfile), and the
// packages a crates_repository declares in its packages dict.
func (r *resolver) cargo(w *workspace, h *hub, n *starlark.Node) {
	if d := n.Keyword("packages"); d != nil && d.Kind == starlark.Dictionary {
		for i := 0; i+1 < len(d.Items); i += 2 {
			name, ok := d.Items[i].StringValue()
			if !ok {
				continue
			}
			v := d.Items[i+1].KeywordString("version")
			h.add(normalizeCrate(name), lang.Target{Ecosystem: ecosystemCrates, Package: name, Version: v, Pinned: exactCargo(v)}, false)
		}
	}
	p := w.labelPath(n.KeywordString("cargo_lockfile"))
	if p == "" {
		return
	}
	source, ok := r.read(p)
	if !ok {
		return
	}
	var lock struct {
		Package []struct {
			Name, Version, Source string
		} `toml:"package"`
	}
	if _, err := toml.Decode(string(source), &lock); err != nil {
		return
	}
	versions := map[string][]string{}
	var names []string
	for _, packageName := range lock.Package {
		if packageName.Source == "" {
			continue // a member of the Cargo workspace
		}
		if versions[packageName.Name] == nil {
			names = append(names, packageName.Name)
		}
		versions[packageName.Name] = append(versions[packageName.Name], packageName.Version)
	}
	for _, name := range names {
		t := lang.Target{Ecosystem: ecosystemCrates, Package: name}
		if candidates := versions[name]; len(candidates) == 1 {
			t.Version, t.Pinned = candidates[0], true
		}
		h.add(normalizeCrate(name), t, true)
	}
}

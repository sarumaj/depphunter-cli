package beam

import (
	"slices"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// dependency is one entry of a mix.exs dependencies list or a rebar.config dependencies list.
type dependency struct {
	app           string // the OTP application, which is what code refers to
	packageName   string // the Hex package, when it differs (hex: :x, {pkg, x})
	requirement   string // version requirement
	git           string
	reference     string // a git reference, tag or branch
	referenceKind string // reference, tag or branch
	path          string
	umbrella      bool // in_umbrella: true
	// repository is the Hex repository the package is published to, when not hex.pm's
	// public one: "hexpm:<organization>" for a private organization's.
	repository string
	line       int
	spec       string
}

func (d *dependency) hexName() string {
	if d.packageName != "" {
		return d.packageName
	}
	return d.app
}

// ---------------------------------------------------------------- mix.exs

// mixDependencies reads the dependencies of a mix.exs: the list its dependencies function returns (def or
// defp deps, with a do block or do:), or a literal `deps: [...]` in project/0.
//
// Implements: REQ-BEAM-009
func mixDependencies(tokens []token) []*dependency {
	p := &termParser{tokens: tokens}
	var out []*dependency
	seen := map[string]bool{}
	read := func(list term) {
		for _, it := range list.items {
			if d := mixDependency(it); d != nil && !seen[d.app] {
				seen[d.app] = true
				out = append(out, d)
			}
		}
	}
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		switch {
		case t.kind == tKey && t.value == "deps" && i+1 < len(tokens) && tokens[i+1].kind == tPunctuation && tokens[i+1].value == "[":
			p.i = i + 1
			read(p.value())
		case t.kind == tIdentifier && (t.value == "def" || t.value == "defp") && i+1 < len(tokens) && tokens[i+1].kind == tIdentifier && tokens[i+1].value == "deps":
			// The first list of the body; `[...] ++ other()` keeps the literal part.
			for j := i + 2; j < len(tokens) && j < i+8; j++ {
				if tokens[j].kind == tPunctuation && tokens[j].value == "[" {
					p.i = j
					v := p.value()
					if v.kind != 'l' {
						p.i = j + 1
						v = term{kind: 'l', items: p.sequence("]")}
					}
					read(v)
					break
				}
			}
		}
	}
	return out
}

// mixDependency reads {:name, requirement, opts}, {:name, opts} or {:name, requirement}.
func mixDependency(t term) *dependency {
	if t.kind != 't' || t.at(0).kind != 'a' {
		return nil
	}
	d := &dependency{app: t.at(0).s, line: t.line, spec: t.render(false)}
	// {:name, opt: value} is {:name, [opt: value]} written without brackets.
	items := []term{}
	var keyword term
	for _, it := range t.items[1:] {
		if it.kind == 'k' {
			keyword.kind = 'l'
			keyword.items = append(keyword.items, it)
			continue
		}
		items = append(items, it)
	}
	if keyword.kind == 'l' {
		items = append(items, keyword)
	}
	for _, it := range items {
		switch it.kind {
		case 's':
			d.requirement = it.s
		case 'l':
			if v, ok := it.option("hex"); ok {
				d.packageName = v.text()
			}
			if v, ok := it.option("git"); ok {
				d.git = v.text()
			}
			if v, ok := it.option("github"); ok && v.text() != "" {
				d.git = "https://github.com/" + v.text() + ".git"
			}
			if v, ok := it.option("path"); ok {
				d.path = v.text()
			}
			if v, ok := it.option("in_umbrella"); ok && v.isAtom("true") {
				d.umbrella = true
			}
			if v, ok := it.option("repo"); ok {
				d.repository = hexRepository(v.text())
			}
			if v, ok := it.option("organization"); ok && v.text() != "" {
				d.repository = "hexpm:" + v.text()
			}
			for _, k := range []string{"ref", "tag", "branch"} {
				if v, ok := it.option(k); ok && v.text() != "" {
					d.reference, d.referenceKind = v.text(), k
				}
			}
		}
	}
	return d
}

func mixDependencyImports(tokens []token) []lang.RawImport {
	var out []lang.RawImport
	for _, d := range mixDependencies(tokens) {
		out = append(out, lang.RawImport{Spec: d.spec, Module: d.app, Name: kindDependency, Line: d.line})
	}
	return out
}

// mixProjectInfo reads project/0's app: and apps_path: from a mix.exs.
func mixProjectInfo(tokens []token) (app, appsPath string) {
	for i := 0; i+1 < len(tokens); i++ {
		if tokens[i].kind != tKey {
			continue
		}
		switch v := tokens[i+1]; tokens[i].value {
		case "app":
			if v.kind == tAtom && app == "" {
				app = v.value
			}
		case "apps_path":
			if v.kind == tString {
				appsPath = v.value
			}
		}
	}
	return app, appsPath
}

// ---------------------------------------------------------------- locks

// locked is one package of mix.lock or rebar.lock.
type locked struct {
	app          string
	packageName  string
	version      string
	git          string
	reference    string
	repository   string             // mix.lock only: the Hex repository, "" for hex.pm's
	dependencies []lockedDependency // mix.lock only
	level        int                // rebar.lock only: 0 for a direct dependency
}

type lockedDependency struct {
	app, packageName, requirement, repository string
	optional                                  bool
}

// hexRepository is a Hex repository name as a lang.Target.Registry: "" for hex.pm's
// public repository, which is every package's unless one is named.
//
// Implements: REQ-BEAM-013
func hexRepository(name string) string {
	if name = strings.TrimSpace(name); name == "hexpm" {
		return ""
	}
	return name
}

// readMixLock reads mix.lock: "app": {:hex, :pkg, "1.2.3", hash, managers, deps,
// "hexpm", hash} or {:git, url, sha, opts}. The repository ("hexpm:acme" for a
// private organization's package) is kept, for the package and for each of its
// requirements (their repository: option).
//
// Implements: REQ-BEAM-010, REQ-BEAM-013
func readMixLock(source []byte) map[string]*locked {
	p := &termParser{tokens: lexElixir(source)}
	m := p.value()
	out := map[string]*locked{}
	for _, e := range m.items {
		if e.kind != 'k' {
			continue
		}
		v := e.at(0)
		l := &locked{app: e.s}
		switch {
		case v.at(0).isAtom("hex"):
			l.packageName, l.version, l.repository = v.at(1).text(), v.at(2).text(), hexRepository(v.at(6).text())
			for _, d := range v.at(5).items {
				options := d.at(2)
				ld := lockedDependency{app: d.at(0).text(), requirement: d.at(1).text()}
				if h, ok := options.option("hex"); ok {
					ld.packageName = h.text()
				}
				if r, ok := options.option("repo"); ok {
					ld.repository = hexRepository(r.text())
				}
				if o, ok := options.option("optional"); ok && o.isAtom("true") {
					ld.optional = true
				}
				if ld.app != "" {
					l.dependencies = append(l.dependencies, ld)
				}
			}
		case v.at(0).isAtom("git"):
			l.git, l.reference = v.at(1).text(), v.at(2).text()
		default:
			continue
		}
		out[l.app] = l
	}
	return out
}

// readRebarLock reads rebar.lock, version 1 ({"1.2.0", [...]}) or the older bare
// list: {<<"app">>, {pkg, <<"pkg">>, <<"1.0.0">>}, Level} or
// {<<"app">>, {git, Url, {ref, Sha}}, Level}.
//
// Implements: REQ-BEAM-010
func readRebarLock(source []byte) map[string]*locked {
	forms := erlForms(source)
	if len(forms) == 0 {
		return nil
	}
	list := forms[0]
	if list.kind == 't' {
		list = list.at(1)
	}
	out := map[string]*locked{}
	for _, e := range list.items {
		if e.kind != 't' {
			continue
		}
		l := &locked{app: e.at(0).text()}
		if n := e.at(2); n.kind == 'n' {
			l.level = atoi(n.s)
		}
		source := e.at(1)
		switch {
		case source.at(0).isAtom("pkg"):
			l.packageName, l.version = source.at(1).text(), source.at(2).text()
		case source.at(0).isAtom("git") || source.at(0).isAtom("git_subdir"):
			l.git = source.at(1).text()
			l.reference = source.at(2).at(1).text()
		default:
			continue
		}
		if l.app != "" {
			out[l.app] = l
		}
	}
	return out
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return n
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// ---------------------------------------------------------------- rebar.config

// rebarDependencies reads the dependencies of a rebar.config, its profiles' included: name,
// {name, "1.0.0"}, {name, {pkg, hexname}}, {name, "1.0.0", {pkg, hexname}},
// {name, {git, Url, {tag|branch|ref, X}}}, {name, ".*", {git, ...}} (rebar2) and
// {name, {git_subdir, Url, Ref, Dir}}.
//
// Implements: REQ-BEAM-009
func rebarDependencies(forms []term) []*dependency {
	var out []*dependency
	seen := map[string]bool{}
	read := func(list term) {
		for _, it := range list.items {
			if d := rebarDependency(it); d != nil && !seen[d.app] {
				seen[d.app] = true
				out = append(out, d)
			}
		}
	}
	for _, f := range forms {
		if f.kind != 't' {
			continue
		}
		switch {
		case f.at(0).isAtom("deps"):
			read(f.at(1))
		case f.at(0).isAtom("profiles"):
			for _, profile := range f.at(1).items {
				if dependencies, ok := profile.at(1).option("deps"); ok {
					read(dependencies)
				}
			}
		}
	}
	return out
}

func rebarDependency(t term) *dependency {
	if t.kind == 'a' {
		return &dependency{app: t.s, line: t.line, spec: t.s}
	}
	if t.kind != 't' || t.at(0).kind != 'a' {
		return nil
	}
	d := &dependency{app: t.at(0).s, line: t.line, spec: t.render(true)}
	for _, it := range t.items[1:] {
		switch {
		case it.kind == 's':
			d.requirement = it.s
		case it.kind == 't' && it.at(0).isAtom("pkg"):
			d.packageName = it.at(1).text()
			if v := it.at(2).text(); v != "" {
				d.requirement = v
			}
		case it.kind == 't' && (it.at(0).isAtom("git") || it.at(0).isAtom("git_subdir") || it.at(0).isAtom("hg")):
			d.git = it.at(1).text()
			reference := it.at(2)
			switch reference.kind {
			case 't':
				d.referenceKind, d.reference = reference.at(0).text(), reference.at(1).text()
			case 's':
				d.referenceKind, d.reference = "branch", reference.s
			}
			if d.requirement == ".*" || d.requirement == "" {
				d.requirement = ""
			}
		}
	}
	if d.git != "" {
		d.requirement = "" // rebar2's version regex beside a git source says nothing
	}
	return d
}

// rebarRegistry is the lang.Target.Registry of a rebar3 project's Hex packages: the
// Hex repositories rebar3 asks, in its order, as internal/index reads it. rebar3
// records no repository for a package, in rebar.config's dependencies or in rebar.lock
// ({pkg, Name, Vsn} and a hash); it asks the repositories its configuration names
// - {hex, [{repos, [#{name => <<"hexpm:acme">>}]}]}, the project's and then the
// global rebar.config's - and then hex.pm's public one ("hexpm"), unless the first
// repos entry is {repos, replace, [...]}. "*" stands for what the machine's
// configuration adds; "" is hex.pm's alone; "-" none at all.
//
// Implements: REQ-BEAM-013
func rebarRegistry(forms []term) string {
	var repositories []string
	replace, first := false, true
	for _, f := range forms {
		if f.kind != 't' || !f.at(0).isAtom("hex") {
			continue
		}
		for _, option := range f.at(1).items {
			if option.kind != 't' || !option.at(0).isAtom("repos") {
				continue
			}
			if first {
				replace = len(option.items) == 3 && option.at(1).isAtom("replace")
				first = false
			}
			for _, r := range option.at(len(option.items) - 1).items {
				if n, ok := r.option("name"); ok && n.text() != "" && !strings.Contains(n.text(), ",") {
					repositories = append(repositories, strings.TrimSpace(n.text()))
				}
			}
		}
	}
	switch {
	case !replace:
		repositories = append(repositories, "*")
	case len(repositories) == 0:
		return "-" // replaced by none: no repository, so nothing is asked
	}
	if !slices.ContainsFunc(repositories, func(r string) bool { return r != "hexpm" }) {
		return ""
	}
	return strings.Join(repositories, ",")
}

// rebarConfigImports makes a rebar.config's dependencies imports of what they declare.
func rebarConfigImports(source []byte) []lang.RawImport {
	var out []lang.RawImport
	for _, d := range rebarDependencies(erlForms(source)) {
		out = append(out, lang.RawImport{Spec: d.spec, Module: d.app, Name: kindDependency, Line: d.line})
	}
	return out
}

// ---------------------------------------------------------------- .app.src

// appSource reads {application, name, [{applications, [...]}, {included_applications,
// [...]}, ...]}: the application and the ones it depends on.
func appSource(source []byte) (name string, apps []term, line int) {
	for _, f := range erlForms(source) {
		if f.kind == 't' && f.at(0).isAtom("application") {
			name, line = f.at(1).text(), f.line
			for _, key := range []string{"applications", "included_applications"} {
				if v, ok := f.at(2).option(key); ok {
					apps = append(apps, v.items...)
				}
			}
			return
		}
	}
	return "", nil, 0
}

// extractAppSource makes an application resource file's applications imports.
//
// Implements: REQ-BEAM-009
func extractAppSource(source []byte) *lang.Extraction {
	name, apps, line := appSource(source)
	extraction := &lang.Extraction{}
	var symbols lang.SymbolSet
	if name != "" {
		symbols.Add(name, "application", line)
	}
	seen := map[string]bool{}
	for _, a := range apps {
		if a.kind != 'a' || seen[a.s] {
			continue
		}
		seen[a.s] = true
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: "application " + a.s, Module: a.s, Name: kindApp, Line: a.line})
	}
	extraction.Symbols = symbols.List()
	return extraction
}

// hexPinned reads a Hex requirement: "== 1.2.3" and a bare "1.2.3" name one version
// (returned bare, as OSV and the index name it); "~>", ">=", "or" and "and" float.
//
// Implements: REQ-BEAM-011
func hexPinned(requirement string) (string, bool) {
	s := strings.TrimSpace(requirement)
	if rest, ok := strings.CutPrefix(s, "=="); ok {
		s = strings.TrimSpace(rest)
	}
	if strings.Contains(s, " ") || !lang.Pinned(s) || lang.Commit(s) {
		return requirement, false
	}
	return s, true
}

// HexPinned is the Hex pinning rule for a requirement written without a lock:
// "== 1.2.3" and a bare "1.2.3" pin that version, anything else is kept as
// written and not pinned. The Gleam plugin shares it.
//
// Implements: REQ-BEAM-011
func HexPinned(requirement string) (string, bool) { return hexPinned(requirement) }

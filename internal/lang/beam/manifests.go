package beam

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// dependency is one entry of a mix.exs deps list or a rebar.config deps list.
type dependency struct {
	app      string // the OTP application, which is what code refers to
	pkg      string // the Hex package, when it differs (hex: :x, {pkg, x})
	req      string // version requirement
	git      string
	ref      string // a git ref, tag or branch
	refKind  string // ref, tag or branch
	path     string
	umbrella bool // in_umbrella: true
	// repo is the Hex repository the package is published to, when not hex.pm's
	// public one: "hexpm:<organization>" for a private organization's.
	repo string
	line int
	spec string
}

func (d *dependency) hexName() string {
	if d.pkg != "" {
		return d.pkg
	}
	return d.app
}

// ---------------------------------------------------------------- mix.exs

// mixDeps reads the deps of a mix.exs: the list its deps function returns (def or
// defp deps, with a do block or do:), or a literal `deps: [...]` in project/0.
//
// Implements: REQ-BEAM-009
func mixDeps(tokens []token) []*dependency {
	p := &termParser{tokens: tokens}
	var out []*dependency
	seen := map[string]bool{}
	read := func(list term) {
		for _, it := range list.items {
			if d := mixDep(it); d != nil && !seen[d.app] {
				seen[d.app] = true
				out = append(out, d)
			}
		}
	}
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		switch {
		case t.kind == tKey && t.val == "deps" && i+1 < len(tokens) && tokens[i+1].kind == tPunct && tokens[i+1].val == "[":
			p.i = i + 1
			read(p.value())
		case t.kind == tIdent && (t.val == "def" || t.val == "defp") && i+1 < len(tokens) && tokens[i+1].kind == tIdent && tokens[i+1].val == "deps":
			// The first list of the body; `[...] ++ other()` keeps the literal part.
			for j := i + 2; j < len(tokens) && j < i+8; j++ {
				if tokens[j].kind == tPunct && tokens[j].val == "[" {
					p.i = j
					v := p.value()
					if v.kind != 'l' {
						p.i = j + 1
						v = term{kind: 'l', items: p.seq("]")}
					}
					read(v)
					break
				}
			}
		}
	}
	return out
}

// mixDep reads {:name, requirement, opts}, {:name, opts} or {:name, requirement}.
func mixDep(t term) *dependency {
	if t.kind != 't' || t.at(0).kind != 'a' {
		return nil
	}
	d := &dependency{app: t.at(0).s, line: t.line, spec: t.render(false)}
	// {:name, opt: value} is {:name, [opt: value]} written without brackets.
	items := []term{}
	var kw term
	for _, it := range t.items[1:] {
		if it.kind == 'k' {
			kw.kind = 'l'
			kw.items = append(kw.items, it)
			continue
		}
		items = append(items, it)
	}
	if kw.kind == 'l' {
		items = append(items, kw)
	}
	for _, it := range items {
		switch it.kind {
		case 's':
			d.req = it.s
		case 'l':
			if v, ok := it.opt("hex"); ok {
				d.pkg = v.text()
			}
			if v, ok := it.opt("git"); ok {
				d.git = v.text()
			}
			if v, ok := it.opt("github"); ok && v.text() != "" {
				d.git = "https://github.com/" + v.text() + ".git"
			}
			if v, ok := it.opt("path"); ok {
				d.path = v.text()
			}
			if v, ok := it.opt("in_umbrella"); ok && v.isAtom("true") {
				d.umbrella = true
			}
			if v, ok := it.opt("repo"); ok {
				d.repo = hexRepo(v.text())
			}
			if v, ok := it.opt("organization"); ok && v.text() != "" {
				d.repo = "hexpm:" + v.text()
			}
			for _, k := range []string{"ref", "tag", "branch"} {
				if v, ok := it.opt(k); ok && v.text() != "" {
					d.ref, d.refKind = v.text(), k
				}
			}
		}
	}
	return d
}

func mixDepImports(tokens []token) []lang.RawImport {
	var out []lang.RawImport
	for _, d := range mixDeps(tokens) {
		out = append(out, lang.RawImport{Spec: d.spec, Module: d.app, Name: kindDep, Line: d.line})
	}
	return out
}

// mixProjectInfo reads project/0's app: and apps_path: from a mix.exs.
func mixProjectInfo(tokens []token) (app, appsPath string) {
	for i := 0; i+1 < len(tokens); i++ {
		if tokens[i].kind != tKey {
			continue
		}
		switch v := tokens[i+1]; tokens[i].val {
		case "app":
			if v.kind == tAtom && app == "" {
				app = v.val
			}
		case "apps_path":
			if v.kind == tString {
				appsPath = v.val
			}
		}
	}
	return app, appsPath
}

// ---------------------------------------------------------------- locks

// locked is one package of mix.lock or rebar.lock.
type locked struct {
	app     string
	pkg     string
	version string
	git     string
	ref     string
	repo    string      // mix.lock only: the Hex repository, "" for hex.pm's
	deps    []lockedDep // mix.lock only
	level   int         // rebar.lock only: 0 for a direct dependency
}

type lockedDep struct {
	app, pkg, req, repo string
	optional            bool
}

// hexRepo is a Hex repository name as a lang.Target.Registry: "" for hex.pm's
// public repository, which is every package's unless one is named.
//
// Implements: REQ-BEAM-013
func hexRepo(name string) string {
	if name = strings.TrimSpace(name); name == "hexpm" {
		return ""
	}
	return name
}

// readMixLock reads mix.lock: "app": {:hex, :pkg, "1.2.3", hash, managers, deps,
// "hexpm", hash} or {:git, url, sha, opts}. The repository ("hexpm:acme" for a
// private organization's package) is kept, for the package and for each of its
// requirements (their repo: option).
//
// Implements: REQ-BEAM-010, REQ-BEAM-013
func readMixLock(src []byte) map[string]*locked {
	p := &termParser{tokens: lexElixir(src)}
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
			l.pkg, l.version, l.repo = v.at(1).text(), v.at(2).text(), hexRepo(v.at(6).text())
			for _, d := range v.at(5).items {
				opts := d.at(2)
				ld := lockedDep{app: d.at(0).text(), req: d.at(1).text()}
				if h, ok := opts.opt("hex"); ok {
					ld.pkg = h.text()
				}
				if r, ok := opts.opt("repo"); ok {
					ld.repo = hexRepo(r.text())
				}
				if o, ok := opts.opt("optional"); ok && o.isAtom("true") {
					ld.optional = true
				}
				if ld.app != "" {
					l.deps = append(l.deps, ld)
				}
			}
		case v.at(0).isAtom("git"):
			l.git, l.ref = v.at(1).text(), v.at(2).text()
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
func readRebarLock(src []byte) map[string]*locked {
	forms := erlForms(src)
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
		src := e.at(1)
		switch {
		case src.at(0).isAtom("pkg"):
			l.pkg, l.version = src.at(1).text(), src.at(2).text()
		case src.at(0).isAtom("git") || src.at(0).isAtom("git_subdir"):
			l.git = src.at(1).text()
			l.ref = src.at(2).at(1).text()
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

// rebarDeps reads the deps of a rebar.config, its profiles' included: name,
// {name, "1.0.0"}, {name, {pkg, hexname}}, {name, "1.0.0", {pkg, hexname}},
// {name, {git, Url, {tag|branch|ref, X}}}, {name, ".*", {git, ...}} (rebar2) and
// {name, {git_subdir, Url, Ref, Dir}}.
//
// Implements: REQ-BEAM-009
func rebarDeps(forms []term) []*dependency {
	var out []*dependency
	seen := map[string]bool{}
	read := func(list term) {
		for _, it := range list.items {
			if d := rebarDep(it); d != nil && !seen[d.app] {
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
			for _, prof := range f.at(1).items {
				if deps, ok := prof.at(1).opt("deps"); ok {
					read(deps)
				}
			}
		}
	}
	return out
}

func rebarDep(t term) *dependency {
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
			d.req = it.s
		case it.kind == 't' && it.at(0).isAtom("pkg"):
			d.pkg = it.at(1).text()
			if v := it.at(2).text(); v != "" {
				d.req = v
			}
		case it.kind == 't' && (it.at(0).isAtom("git") || it.at(0).isAtom("git_subdir") || it.at(0).isAtom("hg")):
			d.git = it.at(1).text()
			ref := it.at(2)
			switch ref.kind {
			case 't':
				d.refKind, d.ref = ref.at(0).text(), ref.at(1).text()
			case 's':
				d.refKind, d.ref = "branch", ref.s
			}
			if d.req == ".*" || d.req == "" {
				d.req = ""
			}
		}
	}
	if d.git != "" {
		d.req = "" // rebar2's version regex beside a git source says nothing
	}
	return d
}

// rebarConfigImports makes a rebar.config's deps imports of what they declare.
func rebarConfigImports(src []byte) []lang.RawImport {
	var out []lang.RawImport
	for _, d := range rebarDeps(erlForms(src)) {
		out = append(out, lang.RawImport{Spec: d.spec, Module: d.app, Name: kindDep, Line: d.line})
	}
	return out
}

// ---------------------------------------------------------------- .app.src

// appSrc reads {application, name, [{applications, [...]}, {included_applications,
// [...]}, ...]}: the application and the ones it depends on.
func appSrc(src []byte) (name string, apps []term, line int) {
	for _, f := range erlForms(src) {
		if f.kind == 't' && f.at(0).isAtom("application") {
			name, line = f.at(1).text(), f.line
			for _, key := range []string{"applications", "included_applications"} {
				if v, ok := f.at(2).opt(key); ok {
					apps = append(apps, v.items...)
				}
			}
			return
		}
	}
	return "", nil, 0
}

// extractAppSrc makes an application resource file's applications imports.
//
// Implements: REQ-BEAM-009
func extractAppSrc(src []byte) *lang.Extraction {
	name, apps, line := appSrc(src)
	ex := &lang.Extraction{}
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
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: "application " + a.s, Module: a.s, Name: kindApp, Line: a.line})
	}
	ex.Symbols = symbols.List()
	return ex
}

// hexPinned reads a Hex requirement: "== 1.2.3" and a bare "1.2.3" name one version
// (returned bare, as OSV and the index name it); "~>", ">=", "or" and "and" float.
//
// Implements: REQ-BEAM-011
func hexPinned(req string) (string, bool) {
	s := strings.TrimSpace(req)
	if rest, ok := strings.CutPrefix(s, "=="); ok {
		s = strings.TrimSpace(rest)
	}
	if strings.Contains(s, " ") || !lang.Pinned(s) || lang.Commit(s) {
		return req, false
	}
	return s, true
}

// HexPinned is the Hex pinning rule for a requirement written without a lock:
// "== 1.2.3" and a bare "1.2.3" pin that version, anything else is kept as
// written and not pinned. The Gleam plugin shares it.
//
// Implements: REQ-BEAM-011
func HexPinned(req string) (string, bool) { return hexPinned(req) }

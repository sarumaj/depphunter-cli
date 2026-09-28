package ocaml

import (
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/opam"
)

// dune files, dune-project and dune-workspace are S-expressions. The parser keeps
// atoms and quoted strings apart only as far as dune does (both are atoms) and
// records each node's line; comments (`;`, `#| |#`, `#;` datum comments) are
// dropped. It uses an explicit stack, so any nesting parses.

type sexp struct {
	atom string
	list []*sexp
	isL  bool
	line int
}

func (s *sexp) head() string {
	if s == nil || !s.isL || len(s.list) == 0 || s.list[0].isL {
		return ""
	}
	return s.list[0].atom
}

// field returns the first sub-list of s headed by name.
func (s *sexp) field(name string) *sexp {
	if s == nil {
		return nil
	}
	for _, c := range s.list[min(1, len(s.list)):] {
		if c.head() == name {
			return c
		}
	}
	return nil
}

// atoms returns the atoms of s after its head.
func (s *sexp) atoms() []*sexp {
	var out []*sexp
	if s == nil {
		return nil
	}
	for _, c := range s.list[min(1, len(s.list)):] {
		if !c.isL {
			out = append(out, c)
		}
	}
	return out
}

func (s *sexp) value(name string) string {
	if a := s.field(name).atoms(); len(a) > 0 {
		return a[0].atom
	}
	return ""
}

func parseSexps(src []byte) []*sexp {
	root := &sexp{isL: true}
	stack := []*sexp{root}
	skip := []int{0} // datum comments pending per level
	n := len(src)
	line := 1
	add := func(s *sexp) {
		k := len(stack) - 1
		if skip[k] > 0 {
			skip[k]--
			return
		}
		stack[k].list = append(stack[k].list, s)
	}
	for i := 0; i < n; {
		c := src[i]
		switch {
		case c == '\n':
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == ';':
			for i < n && src[i] != '\n' {
				i++
			}
		case c == '#' && i+1 < n && src[i+1] == '|':
			j := i + 2
			for j < n && !(src[j] == '|' && j+1 < n && src[j+1] == '#') {
				if src[j] == '\n' {
					line++
				}
				j++
			}
			i = min(j+2, n)
		case c == '#' && i+1 < n && src[i+1] == ';':
			skip[len(skip)-1]++
			i += 2
		case c == '(':
			stack = append(stack, &sexp{isL: true, line: line})
			skip = append(skip, 0)
			i++
		case c == ')':
			if len(stack) > 1 {
				s := stack[len(stack)-1]
				stack, skip = stack[:len(stack)-1], skip[:len(skip)-1]
				add(s)
			}
			i++
		case c == '"':
			start := line
			var b strings.Builder
			j := i + 1
			for j < n && src[j] != '"' {
				if src[j] == '\\' && j+1 < n {
					j++
					switch src[j] {
					case 'n':
						b.WriteByte('\n')
					case '\n':
						line++
						for j+1 < n && (src[j+1] == ' ' || src[j+1] == '\t') {
							j++
						}
					default:
						b.WriteByte(src[j])
					}
					j++
					continue
				}
				if src[j] == '\n' {
					line++
				}
				b.WriteByte(src[j])
				j++
			}
			add(&sexp{atom: b.String(), line: start})
			i = min(j+1, n)
		default:
			j := i
			for j < n && !strings.ContainsRune(" \t\r\n();\"", rune(src[j])) {
				j++
			}
			add(&sexp{atom: string(src[i:j]), line: line})
			i = j
		}
	}
	// Unclosed lists still count.
	for len(stack) > 1 {
		s := stack[len(stack)-1]
		stack, skip = stack[:len(stack)-1], skip[:len(skip)-1]
		add(s)
	}
	return root.list
}

// stanza is a dune library, executable(s) or test(s).
type stanza struct {
	kind    string   // library, executable, executables, test, tests
	names   []string // (name x) / (names a b)
	public  []string // (public_name x) / (public_names a b)
	libs    []*sexp  // what (libraries ...) names
	pps     []*sexp  // ppx rewriters in (preprocess (pps ...)) and lint
	modules []string // (modules ...): nil means every module of the directory
	except  []string // modules taken out of :standard
	wrapped bool
	opens   []string // -open M in flags
	line    int
}

// duneFile is what a dune file says about the directory.
type duneFile struct {
	stanzas []*stanza
	// includeSubdirs is (include_subdirs unqualified|qualified), "" otherwise.
	includeSubdirs string
}

var stanzaKinds = map[string]bool{"library": true, "executable": true, "executables": true, "test": true, "tests": true}

func readDune(src []byte) *duneFile {
	d := &duneFile{}
	for _, s := range parseSexps(src) {
		switch h := s.head(); {
		case h == "include_subdirs":
			if v := s.atoms(); len(v) > 0 && v[0].atom != "no" {
				d.includeSubdirs = v[0].atom
			}
		case stanzaKinds[h]:
			d.stanzas = append(d.stanzas, readStanza(s))
		}
	}
	return d
}

func readStanza(s *sexp) *stanza {
	st := &stanza{kind: s.head(), wrapped: true, line: s.line}
	for _, a := range s.field("name").atoms() {
		st.names = append(st.names, a.atom)
	}
	for _, a := range s.field("names").atoms() {
		st.names = append(st.names, a.atom)
	}
	for _, a := range s.field("public_name").atoms() {
		st.public = append(st.public, a.atom)
	}
	for _, a := range s.field("public_names").atoms() {
		st.public = append(st.public, a.atom)
	}
	if len(st.names) == 0 {
		st.names = st.public // dune derives the name from the public name
	}
	if w := s.value("wrapped"); w == "false" {
		st.wrapped = false
	}
	if l := s.field("libraries"); l != nil {
		st.libs = libraries(l.list[1:], nil)
	}
	for _, f := range []string{"preprocess", "lint"} {
		if p := s.field(f); p != nil {
			st.pps = append(st.pps, ppxs(p, nil, 0)...)
		}
	}
	if m := s.field("modules"); m != nil {
		st.modules = []string{}
		standard, minus := false, false
		for _, a := range flatten(m.list[1:], nil, 0) {
			switch a.atom {
			case ":standard":
				standard = true
			case "\\":
				minus = true
			default:
				if strings.HasPrefix(a.atom, ":") || strings.HasPrefix(a.atom, "%") {
					continue
				}
				if minus {
					st.except = append(st.except, capitalize(a.atom))
				} else {
					st.modules = append(st.modules, capitalize(a.atom))
				}
			}
		}
		if standard {
			st.modules = nil
		}
	}
	for _, f := range []string{"flags", "ocamlc_flags", "ocamlopt_flags"} {
		atoms := flatten(s.field(f).listTail(), nil, 0)
		for i := 0; i+1 < len(atoms); i++ {
			if atoms[i].atom == "-open" {
				st.opens = append(st.opens, atoms[i+1].atom)
			}
		}
	}
	return st
}

func (s *sexp) listTail() []*sexp {
	if s == nil || len(s.list) == 0 {
		return nil
	}
	return s.list[1:]
}

// libraries reads a libraries field: names, (re_export x), and the libraries a
// (select file from (a b -> f) (-> g)) chooses between.
func libraries(items []*sexp, out []*sexp) []*sexp {
	for _, c := range items {
		switch {
		case !c.isL:
			if c.atom != "" && !strings.HasPrefix(c.atom, ":") && !strings.HasPrefix(c.atom, "%") {
				out = append(out, c)
			}
		case c.head() == "re_export":
			out = libraries(c.list[1:], out)
		case c.head() == "select":
			for _, clause := range c.list {
				if !clause.isL {
					continue
				}
				for _, a := range clause.list {
					if a.isL || a.atom == "->" {
						break
					}
					if strings.HasPrefix(a.atom, "!") {
						continue // a library whose absence selects the file
					}
					out = append(out, a)
				}
			}
		}
	}
	return out
}

// ppxs collects the rewriters of every (pps ...) and (staged_pps ...) in a
// preprocess specification, up to a `--` that starts their flags.
func ppxs(s *sexp, out []*sexp, depth int) []*sexp {
	if s == nil || !s.isL || depth > 20 {
		return out
	}
	if h := s.head(); h == "pps" || h == "staged_pps" {
		for _, a := range s.list[1:] {
			if a.isL {
				continue
			}
			if a.atom == "--" {
				break
			}
			if !strings.HasPrefix(a.atom, "-") && !strings.HasPrefix(a.atom, "%") {
				out = append(out, a)
			}
		}
		return out
	}
	for _, c := range s.list {
		out = ppxs(c, out, depth+1)
	}
	return out
}

func flatten(items []*sexp, out []*sexp, depth int) []*sexp {
	if depth > 20 {
		return out
	}
	for _, c := range items {
		if c.isL {
			out = flatten(c.list, out, depth+1)
		} else {
			out = append(out, c)
		}
	}
	return out
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	if c := s[0]; c >= 'a' && c <= 'z' {
		return string(c-'a'+'A') + s[1:]
	}
	return s
}

// extractDune turns a dune file's stanzas into symbols (the components) and
// imports (the libraries and ppx rewriters they use).
func extractDune(src []byte) *lang.Extraction {
	ex := &lang.Extraction{}
	var set lang.SymbolSet
	seen := map[string]bool{}
	for _, st := range readDune(src).stanzas {
		kind := strings.TrimSuffix(st.kind, "s")
		for _, n := range st.names {
			set.Add(kind+" "+n, "component", st.line)
		}
		add := func(field, kind string, libs []*sexp) {
			for _, l := range libs {
				spec := field + ": " + l.atom
				if seen[spec] {
					continue
				}
				seen[spec] = true
				ex.Imports = append(ex.Imports, lang.RawImport{Spec: spec, Module: l.atom, Name: kind, Line: l.line})
			}
		}
		add("libraries", kindLib, st.libs)
		add("pps", kindLib, st.pps)
	}
	ex.Symbols = set.List()
	return ex
}

const (
	kindLib = "lib" // a findlib library name: dune's libraries and pps, #require
	kindDep = "dep" // a package a manifest requires; the second line is its constraint
	kindPin = "pin" // a package pinned to a source; the second line is its URL
)

// project is what a dune-project file says: the packages it describes, their
// dependencies, and dune package management's pins.
type project struct {
	packages []duneProjectPackage
	pins     []opam.Pin
}

type duneProjectPackage struct {
	name    string
	line    int
	depends []opam.Dep
}

func readDuneProject(src []byte) *project {
	p := &project{}
	for _, s := range parseSexps(src) {
		switch s.head() {
		case "package":
			pkg := duneProjectPackage{name: s.value("name"), line: s.line}
			for _, f := range []string{"depends", "depopts"} {
				for _, d := range s.field(f).listTail() {
					pkg.depends = append(pkg.depends, duneDep(d))
				}
			}
			p.packages = append(p.packages, pkg)
		case "pin":
			u := s.value("url")
			for _, pk := range s.list[1:] {
				if pk.head() == "package" {
					p.pins = append(p.pins, opam.Pin{Name: pk.value("name"), Version: pk.value("version"), URL: u, Line: pk.line})
				}
			}
		}
	}
	return p
}

// duneDep reads a dependency of a dune-project package: `lwt`, `(lwt (>= 5.6))`,
// `(lwt (and :with-test (>= 5.6) (< 6)))`, `(yojson (= 2.1.0))`.
func duneDep(d *sexp) opam.Dep {
	if !d.isL {
		return opam.Dep{Name: d.atom, Line: d.line}
	}
	if len(d.list) == 0 || d.list[0].isL {
		return opam.Dep{}
	}
	dep := opam.Dep{Name: d.head(), Line: d.line}
	var parts []string
	ops := 0
	var walk func(s *sexp, conn string, depth int)
	walk = func(s *sexp, conn string, depth int) {
		if depth > 20 {
			return
		}
		if !s.isL {
			if strings.HasPrefix(s.atom, ":") {
				dep.Flags = append(dep.Flags, strings.TrimPrefix(s.atom, ":"))
			}
			return
		}
		switch h := s.head(); h {
		case "and", "or":
			c := "&"
			if h == "or" {
				c = "|"
			}
			for _, x := range s.list[1:] {
				walk(x, c, depth+1)
			}
		case "=", "<>", "<", "<=", ">", ">=":
			if len(s.list) == 2 && !s.list[1].isL && !strings.HasPrefix(s.list[1].atom, ":") {
				op := h
				if op == "<>" {
					op = "!="
				}
				if len(parts) > 0 {
					parts = append(parts, conn)
				}
				parts = append(parts, op+" "+s.list[1].atom)
				ops++
				if h == "=" {
					dep.Exact = s.list[1].atom
				}
			}
		}
	}
	for _, c := range d.list[1:] {
		walk(c, "&", 0)
	}
	dep.Constraint = strings.Join(parts, " ")
	if ops != 1 {
		dep.Exact = ""
	}
	return dep
}

// extractDuneProject makes a dune-project's packages symbols and their
// dependencies and pins imports.
func extractDuneProject(src []byte) *lang.Extraction {
	ex := &lang.Extraction{}
	var set lang.SymbolSet
	p := readDuneProject(src)
	seen := map[string]bool{}
	for _, pkg := range p.packages {
		set.Add("package "+pkg.name, "component", pkg.line)
		for _, d := range pkg.depends {
			manifestImport(ex, seen, "depends: ", d)
		}
	}
	for _, pin := range p.pins {
		spec := "pin: " + pin.Name
		if !seen[spec] && pin.Name != "" {
			seen[spec] = true
			ex.Imports = append(ex.Imports, lang.RawImport{Spec: spec, Module: pin.Name, Name: kindPin + "\n" + pin.URL, Line: pin.Line})
		}
	}
	ex.Symbols = set.List()
	return ex
}

func manifestImport(ex *lang.Extraction, seen map[string]bool, field string, d opam.Dep) {
	if d.Name == "" || opam.Compiler(d.Name) {
		return
	}
	spec := field + d.Name
	if seen[spec] {
		return
	}
	seen[spec] = true
	ex.Imports = append(ex.Imports, lang.RawImport{Spec: spec, Module: d.Name, Name: kindDep + "\n" + d.Constraint, Line: d.Line})
}

// extractOpam makes an opam file's dependencies, optional dependencies and pins
// imports; a lock's are its pinned versions.
func extractOpam(src []byte) *lang.Extraction {
	ex := &lang.Extraction{}
	f := opam.Read(src)
	seen := map[string]bool{}
	for _, d := range f.Depends {
		manifestImport(ex, seen, "depends: ", d)
	}
	for _, d := range f.Depopts {
		manifestImport(ex, seen, "depopts: ", d)
	}
	for _, pin := range f.Pins {
		spec := "pin-depends: " + pin.Name
		if !seen[spec] && pin.Name != "" {
			seen[spec] = true
			ex.Imports = append(ex.Imports, lang.RawImport{Spec: spec, Module: pin.Name, Name: kindPin + "\n" + pin.URL, Line: pin.Line})
		}
	}
	return ex
}

// extractWorkspace makes a dune-workspace's build contexts symbols.
func extractWorkspace(src []byte) *lang.Extraction {
	var set lang.SymbolSet
	for _, s := range parseSexps(src) {
		if s.head() != "context" {
			continue
		}
		name := "default"
		for _, c := range s.list[1:] {
			if c.isL {
				if n := c.value("name"); n != "" {
					name = n
				}
			}
		}
		set.Add("context "+name, "component", s.line)
	}
	return &lang.Extraction{Symbols: set.List()}
}

// WorkspaceRepositories reads the opam repositories a dune-workspace gives
// dune's package management: the `(repository (name x) (url u))` stanzas, in
// the order written, and the `(repositories ...)` of its first `lock_dir` that
// has one - the names the lock is solved against, in order - with listed
// false when no lock_dir lists any (dune's default is then `overlay` and
// `upstream`).
//
// Implements: REQ-SUP-054
func WorkspaceRepositories(src []byte) (defined []opam.Repository, order []string, listed bool) {
	for _, s := range parseSexps(src) {
		switch s.head() {
		case "repository":
			if r := (opam.Repository{Name: s.value("name"), URL: s.value("url")}); r.Name != "" && r.URL != "" {
				defined = append(defined, r)
			}
		case "lock_dir":
			if f := s.field("repositories"); f != nil && !listed {
				listed = true
				for _, a := range f.atoms() {
					order = append(order, a.atom)
				}
			}
		}
	}
	return defined, order, listed
}

// opamPackageName is the package an opam file describes: foo.opam and
// foo.opam.locked describe foo, a file named opam its name field or directory.
func opamPackageName(p string, f *opam.File) string {
	base := path.Base(p)
	base = strings.TrimSuffix(base, ".locked")
	if name, ok := strings.CutSuffix(base, ".opam"); ok && name != "" {
		return name
	}
	if f != nil && f.Name != "" {
		return f.Name
	}
	return path.Base(path.Dir(p))
}

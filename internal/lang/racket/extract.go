package racket

import (
	"path"
	"strconv"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Kinds of imports, carried in RawImport.Name.
const (
	kindColl       = "coll"   // a collection-based module path: racket/list, (lib "x/y.rkt")
	kindLanguage   = "lang"   // a #lang's module path, whose reader is <path>/lang/reader
	kindRelative   = "rel"    // a path relative to the file: "util.rkt", (file "x.rkt")
	kindUp         = "up"     // (path-up "x.rkt"): the nearest such file above
	kindFile       = "file"   // an absolute (file "/...") path
	kindSelf       = "self"   // a submodule of the file itself
	kindPlanet     = "planet" // a PLaneT package
	kindDependency = "deps"   // an info.rkt deps entry; Name is kindDependency + "\x00" + fields
)

// maxDepth bounds how deep module-level forms (submodules, begin) and require
// specs are followed.
const maxDepth = 32

type extractor struct {
	imports []lang.RawImport
	seen    map[string]bool
	symbols lang.SymbolSet
	subs    map[string]bool // submodule names already recorded
}

func (x *extractor) add(spec, module, kind string, line int) {
	if x.seen[spec] {
		return
	}
	x.seen[spec] = true
	x.imports = append(x.imports, lang.RawImport{Spec: spec, Module: module, Name: kind, Line: line})
}

// extractSource reads a module: its #lang (or #reader), the require forms at
// module level (submodules and begin included) and its definitions.
//
// Implements: REQ-RACKET-002, REQ-RACKET-003
func extractSource(source []byte, extension string) *lang.Extraction {
	if extension == ".rktd" {
		return &lang.Extraction{} // data: nothing to link
	}
	m := Read(source, extension == ".scrbl")
	x := &extractor{seen: map[string]bool{}, subs: map[string]bool{}}
	if m.WXME {
		return &lang.Extraction{}
	}
	if m.Language != "" {
		x.language(m.Language, m.LanguageLine)
	}
	if m.Reader != nil {
		x.modulePath(m.Reader, "#reader", 0)
	}
	forms := m.Forms
	// A file without #lang that is one (module name lang body ...) form is
	// that module: its body's definitions are the file's.
	if m.Language == "" && m.Reader == nil {
		var modules []*Node
		for _, f := range forms {
			if f.Kind == List {
				modules = append(modules, f)
			}
		}
		if len(modules) == 1 && modules[0].Head() == "module" && len(modules[0].Kids) >= 3 {
			x.modulePath(modules[0].Kids[2], "module", 0)
			forms = modules[0].Kids[3:]
		}
	}
	x.body(forms, "", true, 0)
	return &lang.Extraction{Imports: x.imports, Symbols: x.symbols.List()}
}

// metaLanguages are the #lang words that take another language after them.
var metaLanguages = map[string]bool{"at-exp": true, "errortrace": true}

// language turns a #lang line into imports: each meta-language and the language
// (a module path whose reader reads the file).
func (x *extractor) language(l string, line int) {
	words := strings.Fields(l)
	for len(words) > 0 {
		w := words[0]
		words = words[1:]
		switch {
		case w == "s-exp" || w == "reader":
			x.add("#lang "+w, w, kindColl, line)
			if len(words) > 0 {
				if f := read([]byte(strings.Join(words, " ")), false, false).Forms; len(f) > 0 {
					f[0].Line = line
					x.modulePath(f[0], "#lang "+w, 0)
				}
			}
			return
		case metaLanguages[w]:
			x.add("#lang "+w, w, kindLanguage, line)
		default:
			if validCollPath(w) {
				x.add("#lang "+w, w, kindLanguage, line)
			}
			return
		}
	}
}

// body reads module-level forms. With symbols set their definitions are
// symbols, named with prefix: the file's own module's, and a module or
// module* submodule's as sub.name; a module+ submodule's (tests, main) are
// not.
func (x *extractor) body(forms []*Node, prefix string, symbols bool, depth int) {
	if depth > maxDepth {
		return
	}
	for _, f := range forms {
		if f.Kind != List {
			continue
		}
		head := f.Head()
		arguments := f.Kids[min(1, len(f.Kids)):]
		switch head {
		case "require", "local-require", "#%require", "require-for-syntax", "require-for-template", "require-for-label":
			for _, a := range arguments {
				x.requireSpec(a, "require", depth)
			}
		case "require/typed", "require/typed/provide", "unsafe-require/typed", "require/expose":
			if len(arguments) > 0 {
				x.modulePath(arguments[0], head, depth)
			}
		case "require-typed-struct", "require/opaque-type", "require-typed-struct/provide":
			if len(arguments) > 2 { // (require/opaque-type T pred? m): the module comes last
				x.modulePath(arguments[len(arguments)-1], head, depth)
			}
		case "lazy-require", "lazy-require-syntax":
			for _, a := range arguments {
				if a.Kind == List && a.Tag == "" && len(a.Kids) > 0 {
					x.modulePath(a.Kids[0], head, depth)
				}
			}
		case "include", "include/reader", "load", "load-relative", "include-section", "include-extracted":
			if len(arguments) > 0 && (arguments[0].Kind == String || head == "include-section" || head == "include-extracted") {
				x.modulePath(arguments[0], head, depth)
			}
		case "module", "module*":
			if len(arguments) >= 2 {
				name := x.submodule(arguments[0], f.Line, prefix, symbols)
				if arguments[1].Kind != Other { // (module* name #f ...) shares the enclosing module
					x.modulePath(arguments[1], head, depth)
				}
				x.body(arguments[2:], name+".", symbols && name != "", depth+1)
			}
		case "module+":
			if len(arguments) >= 1 {
				x.submodule(arguments[0], f.Line, prefix, symbols)
				x.body(arguments[1:], "", false, depth+1)
			}
		case "begin", "begin-for-syntax", "chunk":
			if head == "chunk" && len(arguments) > 0 {
				arguments = arguments[1:] // a literate program's chunk: <name> code ...
			}
			x.body(arguments, prefix, symbols, depth+1)
		default:
			if symbols {
				x.define(f, prefix, head, arguments)
			}
		}
	}
}

// submodule records a submodule's name (once: module+ forms of one name
// add to one submodule) and returns it with the enclosing prefix.
func (x *extractor) submodule(name *Node, line int, prefix string, symbols bool) string {
	if !symbols || name.Kind != Symbol {
		return ""
	}
	full := prefix + name.Text
	if !x.subs[full] {
		x.subs[full] = true
		x.symbols.Add(full, "module", line)
	}
	return full
}

// requireSpec reads one require spec: a module path, or a form around module
// paths (only-in, prefix-in, for-syntax, ...).
func (x *extractor) requireSpec(n *Node, form string, depth int) {
	if depth > maxDepth || n == nil {
		return
	}
	if n.Kind != List || n.Tag != "" {
		x.modulePath(n, form, depth)
		return
	}
	arguments := n.Kids[min(1, len(n.Kids)):]
	switch h := n.Head(); h {
	case "only-in", "except-in", "rename-in", "combine-in", "for-syntax", "for-template", "for-label",
		"subtract-in", "prefix-all-except":
		if h == "prefix-all-except" && len(arguments) > 0 {
			arguments = arguments[1:]
		}
		if h == "only-in" || h == "except-in" || h == "rename-in" {
			arguments = arguments[:min(1, len(arguments))]
		}
		for _, a := range arguments {
			x.requireSpec(a, form, depth+1)
		}
	case "prefix-in", "for-meta", "for-space", "just-meta", "just-space", "only-meta-in", "only-space-in",
		"filtered-in", "matching-identifiers-in", "relative-in":
		if len(arguments) > 1 {
			for _, a := range arguments[1:] {
				x.requireSpec(a, form, depth+1)
			}
		}
	case "multi-in":
		for _, p := range multiIn(arguments) {
			x.add(form+" "+p, p, kindColl, n.Line)
		}
	case "path-up":
		for _, a := range arguments {
			if a.Kind == String {
				x.add(form+" (path-up "+strconv.Quote(a.Text)+")", a.Text, kindUp, a.Line)
			}
		}
	default:
		x.modulePath(n, form, depth)
	}
}

// multiIn expands racket/require's (multi-in racket (list string)): each
// argument a name or a list of alternatives, joined with /; capped at 64.
func multiIn(arguments []*Node) []string {
	out := []string{""}
	for _, a := range arguments {
		var alternatives []string
		switch {
		case a.Kind == Symbol || a.Kind == String:
			alternatives = []string{strings.TrimSuffix(a.Text, ".rkt")}
		case a.Kind == List && a.Tag == "":
			for _, k := range a.Kids {
				if k.Kind == Symbol || k.Kind == String {
					alternatives = append(alternatives, strings.TrimSuffix(k.Text, ".rkt"))
				}
			}
		}
		var next []string
		for _, o := range out {
			for _, alternative := range alternatives {
				if len(next) < 64 {
					next = append(next, strings.TrimPrefix(o+"/"+alternative, "/"))
				}
			}
		}
		out = next
	}
	var valid []string
	for _, p := range out {
		if validCollPath(p) {
			valid = append(valid, p)
		}
	}
	return valid
}

// modulePath records a module path: a symbol (racket/list), a string (a path
// relative to the file), (lib ...), (file ...), (planet ...), (submod ...),
// or a quoted submodule name.
func (x *extractor) modulePath(n *Node, form string, depth int) {
	if n == nil || depth > maxDepth {
		return
	}
	switch n.Kind {
	case Symbol:
		if validCollPath(n.Text) {
			x.add(form+" "+n.Text, n.Text, kindColl, n.Line)
		}
		return
	case String:
		if n.Text != "" && len(n.Text) < 1024 {
			x.add(form+" "+strconv.Quote(n.Text), n.Text, kindRelative, n.Line)
		}
		return
	case List:
	default:
		return
	}
	if n.Tag != "" || len(n.Kids) == 0 {
		return
	}
	arguments := n.Kids[1:]
	stringList := func() []string {
		var out []string
		for _, a := range arguments {
			if a.Kind == String {
				out = append(out, a.Text)
			}
		}
		return out
	}
	switch n.Head() {
	case "quote":
		x.add(form+" '"+render(arguments), "", kindSelf, n.Line)
	case "lib":
		s := stringList()
		if len(s) == 0 {
			return
		}
		p := s[0]
		if len(s) == 1 && !strings.Contains(p, "/") && stripExtension(p) != p {
			p = "mzlib/" + p // (lib "list.ss") is mzlib's
		}
		if len(s) > 1 { // (lib "file.rkt" "coll" "sub"): coll/sub/file.rkt
			p = strings.Join(append(append([]string{}, s[1:]...), s[0]), "/")
		}
		p = stripExtension(p)
		if validCollPath(p) {
			x.add(form+" (lib "+render(arguments)+")", p, kindColl, n.Line)
		}
	case "file":
		if s := stringList(); len(s) > 0 {
			kind := kindRelative
			if path.IsAbs(s[0]) || len(s[0]) > 1 && s[0][1] == ':' || strings.HasPrefix(s[0], "~") {
				kind = kindFile
			}
			x.add(form+" (file "+strconv.Quote(s[0])+")", s[0], kind, n.Line)
		}
	case "planet":
		if p := planetName(arguments); p != "" {
			x.add(form+" (planet "+render(arguments)+")", p, kindPlanet, n.Line)
		}
	case "submod":
		if len(arguments) == 0 {
			return
		}
		if b := arguments[0]; (b.Kind == String || b.Kind == Symbol) && (b.Text == "." || b.Text == "..") {
			x.add(form+" (submod "+render(arguments)+")", "", kindSelf, n.Line)
			return
		}
		x.modulePath(arguments[0], form, depth+1)
	}
}

// planetName reads a PLaneT module path: (planet owner/pkg:1:0/file) or
// (planet "file.rkt" ("owner" "pkg.plt" 1 0)). It returns "owner/pkg" and,
// after a colon, the version.
func planetName(arguments []*Node) string {
	if len(arguments) == 0 {
		return ""
	}
	if a := arguments[0]; a.Kind == Symbol {
		parts := strings.SplitN(a.Text, "/", 3)
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			return ""
		}
		packageName, version, _ := strings.Cut(parts[1], ":")
		return parts[0] + "/" + strings.TrimSuffix(packageName, ".plt") + versionSuffix(strings.ReplaceAll(version, ":", "."))
	}
	for _, a := range arguments[1:] {
		if a.Kind != List || len(a.Kids) < 2 || a.Kids[0].Kind != String || a.Kids[1].Kind != String {
			continue
		}
		var numbers []string
		for _, k := range a.Kids[2:] {
			if k.Kind == Other {
				numbers = append(numbers, k.Text)
			}
		}
		return a.Kids[0].Text + "/" + strings.TrimSuffix(a.Kids[1].Text, ".plt") + versionSuffix(strings.Join(numbers, "."))
	}
	return ""
}

func versionSuffix(v string) string {
	if v == "" {
		return ""
	}
	return ":" + v
}

func stripExtension(p string) string {
	for _, e := range []string{".rkt", ".ss", ".scm", ".scrbl"} {
		if strings.HasSuffix(p, e) {
			return strings.TrimSuffix(p, e)
		}
	}
	return p
}

// validCollPath reports whether s looks like a collection-based module path:
// /-separated names of letters, digits and - _ + . %, no empty segment.
func validCollPath(s string) bool {
	if s == "" || len(s) > 512 || s[0] == '/' || s[len(s)-1] == '/' || strings.Contains(s, "//") {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte("-_+./%", c) >= 0) {
			return false
		}
	}
	for _, segment := range strings.Split(s, "/") {
		if segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

// render writes a few small datums back as text for a spec.
func render(nodes []*Node) string { return renderDepth(nodes, 0) }

func renderDepth(nodes []*Node, depth int) string {
	var b strings.Builder
	for i, n := range nodes {
		if i > 0 {
			b.WriteByte(' ')
		}
		if b.Len() > 80 {
			b.WriteString("...")
			break
		}
		switch n.Kind {
		case String:
			b.WriteString(strconv.Quote(n.Text))
		case List:
			if depth < 4 {
				b.WriteString("(" + renderDepth(n.Kids[:min(len(n.Kids), 8)], depth+1) + ")")
			} else {
				b.WriteString("(...)")
			}
		case Keyword:
			b.WriteString("#:" + n.Text)
		default:
			b.WriteString(n.Text)
		}
	}
	return b.String()
}

// definers maps the definition forms to the kind of symbol they define; ""
// is func for a procedure's header, else by the value's form.
var definers = map[string]string{
	"define": "", "define/contract": "", "define-values": "var", "define-syntax": "macro",
	"define-syntax-rule": "macro", "define-syntaxes": "macro", "define-simple-macro": "macro",
	"define-syntax-parser": "macro", "define-syntax-parse-rule": "macro", "define-syntax-class": "macro",
	"define-splicing-syntax-class": "macro", "define-for-syntax": "", "define-inline": "func",
	"define-type": "type", "define-new-subtype": "type", "define-generics": "interface",
	"struct": "struct", "define-struct": "struct", "define-record-type": "struct",
	"define-signature": "interface", "define-unit": "class", "define-logger": "var", "define-match-expander": "macro",
	"define-predicate": "func", "define-sequence-syntax": "macro", "define-custom-set-types": "type",
	"define-custom-hash-types": "type", "define-runtime-path": "var", "define-parameter": "var",
}

// classMembers are the class body's method definitions.
var classMembers = map[string]bool{
	"define/public": true, "define/override": true, "define/augment": true, "define/pubment": true,
	"define/private": true, "define/public-final": true, "define/override-final": true, "define/augride": true,
	"define/overment": true, "define/augment-final": true,
}

// define records what a module-level definition defines.
func (x *extractor) define(f *Node, prefix, head string, arguments []*Node) {
	kind, ok := definers[head]
	if !ok || len(arguments) == 0 {
		return
	}
	target := arguments[0]
	switch head {
	case "define-values", "define-syntaxes":
		if target.Kind == List {
			for _, k := range target.Kids {
				if k.Kind == Symbol {
					x.symbols.Add(prefix+k.Text, kind, k.Line)
				}
			}
		}
		return
	case "struct", "define-struct", "define-record-type":
		if target.Kind == List && len(target.Kids) > 0 { // (define-struct (name super) ...)
			target = target.Kids[0]
		}
		if target.Kind == Symbol {
			x.symbols.Add(prefix+target.Text, kind, f.Line)
		}
		return
	}
	// (define (f x) ...), curried (define ((f a) b) ...), (define x v)
	isProc := false
	for n := 0; target.Kind == List && target.Tag == "" && len(target.Kids) > 0 && n < 8; n++ {
		target, isProc = target.Kids[0], true
	}
	if target.Kind != Symbol {
		return
	}
	if kind == "" {
		kind = "var"
		if isProc {
			kind = "func"
		} else if len(arguments) > 1 {
			kind = valueKind(arguments[len(arguments)-1])
		}
	}
	x.symbols.Add(prefix+target.Text, kind, f.Line)
	if kind == "class" && len(arguments) > 1 {
		x.members(prefix+target.Text, arguments[len(arguments)-1], 0)
	}
}

// valueKind is the kind a (define x value) gives x by its value's form.
func valueKind(v *Node) string {
	switch v.Head() {
	case "lambda", "λ", "case-lambda", "opt-lambda", "match-lambda", "match-lambda*", "match-λ":
		return "func"
	case "class", "class*", "mixin", "class/derived":
		return "class"
	case "interface", "interface*":
		return "interface"
	}
	return "var"
}

// members records a class body's methods as Class.method.
func (x *extractor) members(owner string, class *Node, depth int) {
	if depth > 2 {
		return
	}
	for _, k := range class.Kids {
		if k.Kind != List {
			continue
		}
		h := k.Head()
		if h == "begin" {
			x.members(owner, k, depth+1)
			continue
		}
		if !classMembers[h] || len(k.Kids) < 2 {
			continue
		}
		t := k.Kids[1]
		for n := 0; t.Kind == List && t.Tag == "" && len(t.Kids) > 0 && n < 8; n++ {
			t = t.Kids[0]
		}
		if t.Kind == Symbol {
			x.symbols.Add(owner+"."+t.Text, "method", k.Line)
		}
	}
}

// extractInfo reads an info.rkt: its deps and build-deps entries are imports.
//
// Implements: REQ-RACKET-006
func extractInfo(source []byte) *lang.Extraction {
	in := readInfo(source)
	var out []lang.RawImport
	seen := map[string]bool{}
	for _, d := range in.dependencies {
		key := "deps"
		if d.build {
			key = "build-deps"
		}
		spec := key + " " + strconv.Quote(d.source)
		if seen[spec] {
			continue
		}
		seen[spec] = true
		out = append(out, lang.RawImport{Spec: spec, Module: d.source, Name: strings.Join([]string{kindDependency, d.version, d.checksum}, "\x00"), Line: d.line})
	}
	return &lang.Extraction{Imports: out}
}

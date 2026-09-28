package beam

// cSpell: words: behaviour

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Import kinds, carried in RawImport.Name.
const (
	kindElixir         = "elixir"      // an Elixir module: Module is Foo.Bar
	kindErlang         = "erl"         // an Erlang module: Module is its atom
	kindInclude        = "include"     // -include("x.hrl")
	kindIncludeLibrary = "include_lib" // -include_lib("app/include/x.hrl")
	kindDependency     = "dep"         // a dependency a mix.exs or rebar.config declares
	kindApp            = "app"         // an application a .app.src depends on
)

// exFrame is an open do/end (or fn/end) block; a module's names the module.
type exFrame struct {
	module string // set for defmodule, defprotocol and defimpl bodies
	kind   string
	scope  string // a Phoenix router scope's alias, prefixed to the controllers in it
	quote  bool   // a quote block: code injected into the modules that use this one
}

type exReader struct {
	tokens          []token
	stack           []exFrame
	aliases         map[string]string // As -> Full.Module
	defined         map[string]bool   // modules this file defines
	implementations map[string]bool   // of which protocol implementations (Proto.Type)
	symbols         lang.SymbolSet
	functions       map[string]bool
	imports         []lang.RawImport
	seen            map[string]bool // kind|module already imported
	scopeAt         map[int]string  // the do opening a router scope -> the scope's alias
	quoteAt         int             // the do opening a quote block, or -1
	used            []string        // modules this file uses (use Foo), in order
	// injected are, per module of this file, the aliases its quote blocks make and
	// the modules they use: what `use` of that module brings into the caller.
	injected map[string]*injection
}

// injection is what a module's quote blocks alias and use.
type injection struct {
	aliases map[string]string
	uses    []string
}

// extractElixir reads an Elixir file's modules and definitions, the modules it names
// in alias, import, require and use and in any other reference (a remote call, a
// struct, a behavior, an argument), and the Erlang modules it calls (:ets.new). A
// mix.exs (a module that uses Mix.Project) also imports its dependencies.
//
// Implements: REQ-BEAM-002, REQ-BEAM-003, REQ-BEAM-009
func extractElixir(source []byte) *lang.Extraction {
	r := newExReader(source)
	r.read()
	var imports []lang.RawImport
	for _, rawImport := range r.imports {
		if strings.HasPrefix(rawImport.Name, kindElixir) && r.defined[rawImport.Module] {
			continue // a module of this very file
		}
		imports = append(imports, rawImport)
	}
	if r.mixProject() {
		imports = append(imports, mixDependencyImports(r.tokens)...)
	}
	slices.SortStableFunc(imports, func(a, b lang.RawImport) int {
		return cmp.Or(cmp.Compare(a.Line, b.Line), cmp.Compare(a.Spec, b.Spec))
	})
	return &lang.Extraction{Imports: imports, Symbols: r.symbols.List()}
}

func newExReader(source []byte) *exReader {
	return &exReader{tokens: lexElixir(source), aliases: map[string]string{}, defined: map[string]bool{},
		implementations: map[string]bool{}, functions: map[string]bool{}, seen: map[string]bool{}, scopeAt: map[int]string{},
		quoteAt: -1, injected: map[string]*injection{}}
}

func (r *exReader) token(i int) token {
	if i >= 0 && i < len(r.tokens) {
		return r.tokens[i]
	}
	return token{kind: tPunctuation, value: "eof"}
}

func (r *exReader) isPunctuation(i int, v string) bool {
	t := r.token(i)
	return t.kind == tPunctuation && t.value == v
}

// bare reports whether token i is a bare word: not a field or function after a dot,
// not a keyword key.
func (r *exReader) bare(i int, word string) bool {
	t := r.token(i)
	return t.kind == tIdentifier && t.value == word && !r.isPunctuation(i-1, ".")
}

// module is the innermost module being defined, or "".
func (r *exReader) module() string {
	for i := len(r.stack) - 1; i >= 0; i-- {
		if r.stack[i].module != "" {
			return r.stack[i].module
		}
	}
	return ""
}

// inModuleBody reports whether the innermost open block is a module body, where
// definitions are made (not a function body, a quote or an if).
func (r *exReader) inModuleBody() bool {
	return len(r.stack) > 0 && r.stack[len(r.stack)-1].module != ""
}

func (r *exReader) read() {
	for i := 0; i < len(r.tokens); {
		i = r.step(i)
	}
}

// step handles the token at i and returns the index of the next one to read.
func (r *exReader) step(i int) int {
	t := r.tokens[i]
	switch t.kind {
	case tIdentifier:
		if r.isPunctuation(i-1, ".") {
			return i + 1
		}
		switch t.value {
		case "do", "fn":
			f := exFrame{kind: t.value, scope: r.scope()}
			if a, ok := r.scopeAt[i]; ok {
				f.scope = joinModule(f.scope, a)
			}
			f.quote = i == r.quoteAt || r.inQuote()
			r.stack = append(r.stack, f)
		case "scope":
			return r.routerScope(i)
		case "quote":
			// quote do ... end, quote location: :keep do ... end
			for j := i + 1; j < len(r.tokens) && j < i+12; j++ {
				if r.bare(j, "do") {
					r.quoteAt = j
					break
				}
			}
		case "end":
			if len(r.stack) > 0 {
				r.stack = r.stack[:len(r.stack)-1]
			}
		case "defmodule", "defprotocol":
			return r.defmodule(i)
		case "defimpl":
			return r.defimpl(i)
		case "def", "defp", "defmacro", "defmacrop", "defguard", "defguardp", "defdelegate", "defn", "defnp":
			if r.inModuleBody() {
				r.definition(i)
			}
		case "defstruct", "defexception":
			if m := r.module(); m != "" && r.inModuleBody() {
				kind := "struct"
				if t.value == "defexception" {
					kind = "exception"
				}
				r.symbols.Add("%"+m+"{}", kind, t.line)
			}
		case "alias", "import", "require", "use":
			return r.directive(i)
		case "__MODULE__":
			if r.isPunctuation(i+1, ".") && r.token(i+2).kind == tAlias {
				segments, next := r.chain(i + 2)
				r.reference(r.module()+"."+strings.Join(segments, "."), t.line)
				return next
			}
		}
	case tAlias:
		if r.isPunctuation(i-1, ".") {
			return i + 1
		}
		segments, next := r.chain(i)
		if len(segments) == 1 && segments[0] == "Elixir" {
			return next // the root of every alias, as in Module.concat(Elixir, name)
		}
		if sc := r.scope(); sc != "" {
			r.reference(sc+"."+strings.Join(segments, "."), t.line)
			return next
		}
		if _, aliased := r.aliases[segments[0]]; !aliased && len(r.used) > 0 && segments[0] != "Elixir" {
			// Maybe aliased by a module this file uses: the resolver knows what
			// their quote blocks alias.
			r.add(lang.RawImport{Spec: strings.Join(segments, "."), Module: strings.Join(segments, "."),
				Name: kindElixir + ":" + strings.Join(r.used, ","), Line: t.line})
			return next
		}
		r.reference(r.expand(segments), t.line)
		return next
	case tAtom:
		// :ets.new(...) - an Erlang module called from Elixir.
		if r.isPunctuation(i+1, ".") && r.token(i+2).kind == tIdentifier && !r.isPunctuation(i-1, ".") {
			r.erlReference(t.value, t.value, t.line)
		}
	case tPunctuation:
		if t.value == "@" && r.token(i+1).kind == tIdentifier {
			return r.attribute(i)
		}
	}
	return i + 1
}

func (r *exReader) inQuote() bool {
	return len(r.stack) > 0 && r.stack[len(r.stack)-1].quote
}

// inject records an alias (as != "") or a use made in a quote block of the current
// module.
func (r *exReader) inject(as, module string) {
	m := r.module()
	if m == "" || !r.inQuote() {
		return
	}
	in := r.injected[m]
	if in == nil {
		in = &injection{aliases: map[string]string{}}
		r.injected[m] = in
	}
	if as != "" {
		in.aliases[as] = module
	} else {
		in.uses = append(in.uses, module)
	}
}

// scope is the router scope alias in effect.
func (r *exReader) scope() string {
	if len(r.stack) == 0 {
		return ""
	}
	return r.stack[len(r.stack)-1].scope
}

func joinModule(a, b string) string {
	if a == "" {
		return b
	}
	return a + "." + b
}

// routerScope reads a Phoenix router's `scope "/path", Alias, opts do` (or
// `alias: Alias`): the controllers, live views and plugs routed in its block are
// named relative to the alias.
func (r *exReader) routerScope(i int) int {
	alias, next := "", i+1
	depth := 0
	for j := i + 1; j < len(r.tokens) && j < i+40; j++ {
		t := r.tokens[j]
		switch {
		case t.kind == tPunctuation && (t.value == "(" || t.value == "[" || t.value == "{"):
			depth++
		case t.kind == tPunctuation && (t.value == ")" || t.value == "]" || t.value == "}"):
			depth--
		case t.kind == tAlias && depth == 0 && alias == "" && !r.isPunctuation(j-1, "."):
			if previous := r.token(j - 1); previous.kind == tKey && previous.value != "alias" {
				continue
			}
			segments, n := r.chain(j)
			alias, next = strings.Join(segments, "."), n
			j = n - 1
		case t.kind == tIdentifier && t.value == "do" && depth == 0:
			if alias != "" {
				r.scopeAt[j] = alias
				return next
			}
			return i + 1
		case t.kind == tIdentifier && t.value == "end", t.kind == tKey && t.value == "do":
			return i + 1
		}
	}
	return i + 1
}

// chain reads Foo.Bar.Baz from i, returning its segments and the index after it.
func (r *exReader) chain(i int) ([]string, int) {
	segments := []string{r.tokens[i].value}
	i++
	for r.isPunctuation(i, ".") && r.token(i+1).kind == tAlias {
		segments = append(segments, r.tokens[i+1].value)
		i += 2
	}
	return segments, i
}

// expand resolves an alias chain's first segment through the aliases in effect.
func (r *exReader) expand(segments []string) string {
	if segments[0] == "Elixir" && len(segments) > 1 {
		return strings.Join(segments[1:], ".")
	}
	if full, ok := r.aliases[segments[0]]; ok {
		return strings.Join(append([]string{full}, segments[1:]...), ".")
	}
	return strings.Join(segments, ".")
}

// moduleName reads a module name at i - an alias chain, or __MODULE__ followed by
// one - expanded; ok is false for anything else (unquote(name), an atom).
func (r *exReader) moduleName(i int, expand bool) (name string, segments []string, next int, ok bool) {
	t := r.token(i)
	switch {
	case t.kind == tAlias:
		segments, next = r.chain(i)
		if expand {
			return r.expand(segments), segments, next, true
		}
		return strings.Join(segments, "."), segments, next, true
	case t.kind == tIdentifier && t.value == "__MODULE__":
		current := r.module()
		if current == "" {
			return "", nil, i + 1, false
		}
		if r.isPunctuation(i+1, ".") && r.token(i+2).kind == tAlias {
			segments, next = r.chain(i + 2)
			return current + "." + strings.Join(segments, "."), segments, next, true
		}
		return current, strings.Split(current, "."), i + 1, true
	}
	return "", nil, i + 1, false
}

// defmodule reads `defmodule Name do`: a nested module's name is qualified by the
// enclosing one's, which also aliases its first segment.
func (r *exReader) defmodule(i int) int {
	keyword := r.tokens[i]
	name, segments, next, ok := r.moduleName(i+1, false)
	if !ok {
		return i + 1
	}
	if r.token(i+1).kind == tAlias {
		if current := r.module(); current != "" {
			name = current + "." + name
			r.aliases[segments[0]] = current + "." + segments[0]
		} else if segments[0] == "Elixir" && len(segments) > 1 {
			name = strings.Join(segments[1:], ".")
		}
	}
	kind := "module"
	if keyword.value == "defprotocol" {
		kind = "protocol"
	}
	r.defined[name] = true
	r.symbols.Add(name, kind, keyword.line)
	return r.openModule(name, next)
}

// openModule pushes a module body when a do block follows its head at i.
func (r *exReader) openModule(name string, i int) int {
	if r.bare(i, "do") {
		r.stack = append(r.stack, exFrame{module: name, kind: "do"})
		return i + 1
	}
	return i
}

// defimpl reads `defimpl Proto, for: Type do`: the implementation is the module
// Proto.Type, and the protocol (and type) are references.
func (r *exReader) defimpl(i int) int {
	keyword := r.tokens[i]
	proto, _, next, ok := r.moduleName(i+1, true)
	if !ok {
		return i + 1
	}
	r.reference(proto, keyword.line)
	target := r.module()
	if r.isPunctuation(next, ",") && r.token(next+1).kind == tKey && r.token(next+1).value == "for" {
		if name, _, n, ok := r.moduleName(next+2, true); ok {
			target, next = name, n
			r.reference(name, keyword.line)
		}
	}
	for r.isPunctuation(next, ",") || r.token(next).kind == tKey && r.token(next).value != "do" {
		next++ // further options
	}
	name := proto
	if target != "" {
		name += "." + target
	}
	r.defined[name] = true
	r.implementations[name] = true
	r.symbols.Add(name, "impl", keyword.line)
	return r.openModule(name, next)
}

// definition records `def name(args)` as Module.name/arity, once per name and arity.
func (r *exReader) definition(i int) {
	keyword := r.tokens[i]
	n := r.token(i + 1)
	if n.kind != tIdentifier || n.value == "unquote" {
		return
	}
	arity := 0
	switch next := r.token(i + 2); {
	case next.kind == tPunctuation && next.value == "(":
		arity = r.arity(i + 2)
	case next.kind == tPunctuation && next.value == ",", next.kind == tKey, next.kind == tIdentifier && (next.value == "do" || next.value == "when"),
		next.kind == tPunctuation && next.value == "eof":
	default:
		return // an operator definition (def a <> b), or something else
	}
	name := r.module() + "." + n.value + "/" + strconv.Itoa(arity)
	if r.functions[name] {
		return
	}
	r.functions[name] = true
	kind := "function"
	switch keyword.value {
	case "defp", "defnp":
		kind = "func"
	case "defmacro", "defmacrop", "defguard", "defguardp":
		kind = "macro"
	}
	r.symbols.Add(name, kind, keyword.line)
}

// arity counts the arguments of the parenthesized list opening at i.
func (r *exReader) arity(i int) int {
	depth, n, any := 0, 0, false
	for j := i; j < len(r.tokens); j++ {
		t := r.tokens[j]
		if t.kind == tIdentifier && (t.value == "do" || t.value == "fn") {
			depth++
			continue
		}
		if t.kind == tIdentifier && t.value == "end" {
			depth--
			continue
		}
		if t.kind != tPunctuation {
			any = true
			continue
		}
		switch t.value {
		case "(", "[", "{", "<<":
			depth++
			if depth > 1 {
				any = true
			}
		case ")", "]", "}", ">>":
			depth--
			if depth == 0 {
				if any {
					n++
				}
				return n
			}
		case ",":
			if depth == 1 {
				n++
			}
		default:
			any = true
		}
	}
	return n
}

// attribute reads @type/@typep/@opaque names and an Erlang @behaviour.
func (r *exReader) attribute(i int) int {
	name := r.tokens[i+1]
	switch name.value {
	case "type", "typep", "opaque":
		if t := r.token(i + 2); t.kind == tIdentifier && r.inModuleBody() {
			r.symbols.Add(r.module()+"."+t.value, "type", name.line)
			return i + 3
		}
	case "behaviour", "behavior":
		if t := r.token(i + 2); t.kind == tAtom {
			r.erlReference(t.value, t.value, name.line)
			return i + 3
		}
	}
	return i + 2
}

// directive reads alias, import, require and use: `alias Foo.Bar`, `alias Foo.Bar,
// as: B`, `alias Foo.{A, B.C}`, `alias __MODULE__.X`, `import :lists`.
func (r *exReader) directive(i int) int {
	keyword := r.tokens[i]
	if r.isPunctuation(i+1, "(") {
		i++ // alias(Foo.Bar)
	}
	if t := r.token(i + 1); t.kind == tAtom {
		r.erlReference(t.value, keyword.value+" :"+t.value, keyword.line)
		return i + 2
	}
	full, segments, next, ok := r.moduleName(i+1, true)
	if !ok {
		return i + 1
	}
	// alias Foo.{A, B.C}
	if r.isPunctuation(next, ".") && r.isPunctuation(next+1, "{") {
		j := next + 2
		for j < len(r.tokens) && !r.isPunctuation(j, "}") {
			if r.token(j).kind == tAlias {
				inner, n := r.chain(j)
				name := full + "." + strings.Join(inner, ".")
				if keyword.value == "alias" {
					r.aliases[inner[len(inner)-1]] = name
					r.inject(inner[len(inner)-1], name)
				}
				r.directiveImport(keyword, name)
				j = n
				continue
			}
			j++
		}
		return j + 1
	}
	as := segments[len(segments)-1]
	explicit := false
	if (keyword.value == "alias" || keyword.value == "require") && r.isPunctuation(next, ",") && r.token(next+1).kind == tKey && r.token(next+1).value == "as" && r.token(next+2).kind == tAlias {
		as, explicit = r.token(next+2).value, true
		next += 3
	}
	if (keyword.value == "alias" || explicit) && as != full {
		r.aliases[as] = full
		r.inject(as, full)
	}
	if keyword.value == "use" {
		r.used = append(r.used, full)
		r.inject("", full)
	}
	r.directiveImport(keyword, full)
	return next
}

func (r *exReader) directiveImport(keyword token, module string) {
	r.add(lang.RawImport{Spec: keyword.value + " " + module, Module: module, Name: kindElixir, Line: keyword.line})
}

// reference records a reference to an Elixir module.
func (r *exReader) reference(module string, line int) {
	if module == "" || module == "Elixir" {
		return
	}
	r.add(lang.RawImport{Spec: module, Module: module, Name: kindElixir, Line: line})
}

func (r *exReader) erlReference(module, spec string, line int) {
	if !strings.HasPrefix(spec, ":") && !strings.Contains(spec, " ") {
		spec = ":" + spec
	}
	r.add(lang.RawImport{Spec: spec, Module: module, Name: kindErlang, Line: line})
}

// add keeps the first import of each module.
func (r *exReader) add(rawImport lang.RawImport) {
	kind, _, _ := strings.Cut(rawImport.Name, ":")
	key := kind + "|" + rawImport.Module
	if r.seen[key] {
		return
	}
	r.seen[key] = true
	r.imports = append(r.imports, rawImport)
}

// mixProject reports whether the file defines a Mix project (use Mix.Project).
func (r *exReader) mixProject() bool {
	for i := range r.tokens {
		if r.bare(i, "use") && r.token(i+1).value == "Mix" && r.isPunctuation(i+2, ".") && r.token(i+3).value == "Project" {
			return true
		}
	}
	return false
}

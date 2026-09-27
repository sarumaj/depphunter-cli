package clojure

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/edn"
)

// Import kinds, in RawImport.Name.
const (
	kindNS       = "ns"        // a namespace: :require, :use, require, use
	kindNPM      = "npm"       // a ClojureScript string require: ["react" :as react]
	kindClass    = "class"     // a Java (or Closure Library) class: :import, import
	kindLoad     = "load"      // (load "x/y"): relative to the namespace's directory
	kindLoadFile = "load-file" // (load-file "path"): relative to the working directory
	kindDep      = "dep"       // a manifest's dependency ("dep:<alias or profile>")
	kindPlugin   = "plugin"    // a Leiningen plugin
)

// defKinds are the definition forms read as symbols, and their kinds.
var defKinds = map[string]string{
	"def": "var", "defonce": "var", "defn": "func", "defn-": "func", "defmacro": "macro",
	"defmulti": "func", "defmethod": "method", "defprotocol": "interface", "definterface": "interface",
	"defrecord": "class", "deftype": "class", "defstruct": "type", "deftest": "test",
}

// qualifiedDefs are the definition forms also read when a library's macro of the same
// name stands in (schema's s/defn, malli's mu/defn); def and defonce are not among
// them: s/def defines a spec, not a var.
var qualifiedDefs = map[string]bool{
	"defn": true, "defn-": true, "defmacro": true, "defmethod": true, "defmulti": true,
	"defprotocol": true, "defrecord": true, "deftype": true,
}

// source is what one Clojure file declares and requires.
type source struct {
	ns      string
	imports []lang.RawImport
	seen    map[string]bool
	symbols []lang.Symbol
	names   map[string]bool
}

// readSource reads a Clojure, ClojureScript or babashka file's ns form, top-level
// require/use/import/load calls and definitions.
//
// Implements: REQ-CLOJURE-002, REQ-CLOJURE-003
func readSource(src []byte) *source {
	s := &source{seen: map[string]bool{}, names: map[string]bool{}}
	for _, form := range edn.Read(src) {
		s.topLevel(form, 0)
	}
	return s
}

func (s *source) topLevel(form *edn.Node, depth int) {
	head := form.Head()
	if head == "" || depth > 8 {
		return
	}
	name := head
	if i := strings.LastIndexByte(head, '/'); i > 0 {
		name = head[i+1:]
		if strings.HasPrefix(head, "clojure.core/") {
			head = name
		}
	}
	args := form.Kids[1:]
	switch head {
	case "ns":
		s.nsForm(form)
		return
	case "require", "use", "require-macros", "use-macros":
		for _, a := range args {
			s.libspec(edn.Unquote(a), "", form.Line)
		}
		return
	case "import":
		for _, a := range args {
			s.classSpec(edn.Unquote(a))
		}
		return
	case "load":
		for _, a := range args {
			if a.Kind == edn.String {
				s.add(kindLoad, a.Text, `(load "`+a.Text+`")`, a.Line)
			}
		}
		return
	case "load-file":
		if len(args) > 0 && args[0].Kind == edn.String {
			s.add(kindLoadFile, args[0].Text, `(load-file "`+args[0].Text+`")`, args[0].Line)
		}
		return
	case "do":
		for _, a := range args {
			s.topLevel(a, depth+1)
		}
		return
	}
	kind, ok := defKinds[head]
	if !ok && head != name && qualifiedDefs[name] {
		kind, ok = defKinds[name], true
		head = name
	}
	if !ok || len(args) == 0 || args[0].Kind != edn.Symbol {
		return
	}
	sym := args[0].Text
	switch head {
	case "defmethod":
		if len(args) > 1 {
			sym += " " + edn.Render(args[1], 40)
		}
	case "defprotocol", "definterface":
		s.symbol(sym, kind, form.Line)
		for _, m := range args[1:] {
			if m.Kind == edn.List && len(m.Kids) > 0 && m.Kids[0].Kind == edn.Symbol {
				s.symbol(sym+"."+m.Kids[0].Text, "method", m.Line)
			}
		}
		return
	}
	s.symbol(sym, kind, form.Line)
}

func (s *source) symbol(name, kind string, line int) {
	if !s.names[name] { // multi-arity or both branches of a reader conditional
		s.names[name] = true
		s.symbols = append(s.symbols, lang.Symbol{Name: name, Kind: kind, Line: line})
	}
}

func (s *source) add(kind, module, spec string, line int) {
	if module == "" || s.seen[kind+" "+module] {
		return
	}
	s.seen[kind+" "+module] = true
	s.imports = append(s.imports, lang.RawImport{Spec: spec, Module: module, Name: kind, Line: line})
}

// nsForm reads (ns name docstring? attr-map? references...).
func (s *source) nsForm(form *edn.Node) {
	if len(form.Kids) < 2 || form.Kids[1].Kind != edn.Symbol {
		return
	}
	if s.ns == "" {
		s.ns = form.Kids[1].Text
		s.symbol(s.ns, "namespace", form.Line)
	}
	for _, ref := range form.Kids[2:] {
		if (ref.Kind != edn.List && ref.Kind != edn.Vector) || len(ref.Kids) == 0 {
			continue
		}
		k := ref.Kids[0]
		key := k.Text
		if k.Kind != edn.Keyword && k.Kind != edn.Symbol {
			continue
		}
		switch key {
		case "require", "use", "require-macros", "use-macros":
			for _, a := range ref.Kids[1:] {
				s.libspec(a, "", ref.Line)
			}
		case "import":
			for _, a := range ref.Kids[1:] {
				s.classSpec(a)
			}
		case "load":
			for _, a := range ref.Kids[1:] {
				if a.Kind == edn.String {
					s.add(kindLoad, a.Text, `(load "`+a.Text+`")`, a.Line)
				}
			}
		}
	}
}

// libspec reads one argument of require/use: a symbol, a ClojureScript string, a
// libspec vector [lib & options] or a prefix list [prefix lib1 [lib2 :as x]].
//
// Implements: REQ-CLOJURE-002
func (s *source) libspec(n *edn.Node, prefix string, line int) {
	full := func(name string) string {
		if prefix == "" {
			return name
		}
		return prefix + "." + name
	}
	switch n.Kind {
	case edn.Symbol:
		s.add(kindNS, full(n.Text), full(n.Text), n.Line)
	case edn.String:
		if prefix == "" {
			s.add(kindNPM, n.Text, `"`+n.Text+`"`, n.Line)
		}
	case edn.Vector, edn.List:
		if len(n.Kids) == 0 {
			return
		}
		first, rest := n.Kids[0], n.Kids[1:]
		switch first.Kind {
		case edn.String:
			if prefix == "" {
				s.add(kindNPM, first.Text, `"`+first.Text+`"`, first.Line)
			}
		case edn.Symbol:
			if len(rest) > 0 && rest[0].Kind != edn.Keyword {
				// A prefix list: the rest are libs under the prefix.
				for _, r := range rest {
					s.libspec(r, full(first.Text), line)
				}
				return
			}
			// :as-alias only names a namespace for keywords; nothing is loaded.
			if hasOption(rest, "as-alias") && !hasOption(rest, "as") && !hasOption(rest, "refer") {
				return
			}
			s.add(kindNS, full(first.Text), full(first.Text), first.Line)
		}
	}
}

func hasOption(opts []*edn.Node, key string) bool {
	for _, o := range opts {
		if o.Kind == edn.Keyword && o.Text == key {
			return true
		}
	}
	return false
}

// classSpec reads one argument of :import: a class symbol (java.util.Date) or a
// package list [java.util Date UUID] / (java.io File).
//
// Implements: REQ-CLOJURE-002
func (s *source) classSpec(n *edn.Node) {
	switch n.Kind {
	case edn.Symbol:
		s.add(kindClass, n.Text, n.Text, n.Line)
	case edn.Vector, edn.List:
		if len(n.Kids) == 0 || n.Kids[0].Kind != edn.Symbol {
			return
		}
		pkg := n.Kids[0].Text
		for _, c := range n.Kids[1:] {
			if c.Kind == edn.Symbol {
				s.add(kindClass, pkg+"."+c.Text, pkg+"."+c.Text, c.Line)
			}
		}
	}
}

// nsName reads only a file's namespace: the first ns (or in-ns) form.
func nsName(src []byte) string {
	name, forms := "", 0
	edn.ReadTop(src, func(n *edn.Node) bool {
		forms++
		switch n.Head() {
		case "ns":
			if len(n.Kids) > 1 && n.Kids[1].Kind == edn.Symbol {
				name = n.Kids[1].Text
			}
			return false
		case "in-ns":
			if len(n.Kids) > 1 {
				if q := edn.Unquote(n.Kids[1]); q.Kind == edn.Symbol {
					name = q.Text
				}
			}
			return false
		}
		return forms < 32 // a namespace is declared first, if at all
	})
	return name
}

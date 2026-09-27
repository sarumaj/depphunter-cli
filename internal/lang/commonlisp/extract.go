package commonlisp

import (
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Import kinds, carried in RawImport.Name ("system" takes the version
// asked for after a NUL).
const (
	kindPackage   = "package"    // a defpackage's :use, :import-from, :local-nicknames ...
	kindInPackage = "in-package" // (in-package :foo)
	kindRef       = "ref"        // a qualified symbol pkg:sym
	kindSystem    = "system"     // an ASDF system: :depends-on, quickload, load-system
	kindRequire   = "require"    // (require :sb-posix), (:require "x") in :depends-on
	kindLoad      = "load"       // (load "x.lisp")
	kindComponent = "component"  // a file of a defsystem's :components
	kindQlfile    = "qlfile"     // an entry of a qlfile
	kindLock      = "lock"       // an entry of a qlfile.lock
	kindOcicl     = "ocicl"      // a row of ocicl.csv
)

// system is a defsystem form of an .asd file.
type system struct {
	name string
	line int
	// base is the directory its components are relative to, relative to the
	// .asd file's directory ("." for the same): the system's :pathname.
	base string
	pis  bool // :class :package-inferred-system
	deps []sysDep
	// files are its components' paths, relative to the .asd file's directory.
	files []string
}

type sysDep struct {
	name, version string
	require       bool
	option        string // depends-on, defsystem-depends-on, weakly-depends-on
	line          int
}

// pkgDef is a defpackage form: the package's name and nicknames.
type pkgDef struct {
	names []string
	nicks map[string]string // its local nicknames
	line  int
}

// info is what a Lisp file (source or .asd) says: its imports, symbols, the
// packages and systems it defines, and the packages .asd files register for
// systems (asdf:register-system-packages).
type info struct {
	imports    []lang.RawImport
	symbols    []lang.Symbol
	packages   []pkgDef
	systems    []*system
	registered map[string]string // package -> system
}

type extractor struct {
	info
	seen     map[string]bool // specs taken
	defined  map[string]bool // symbol names taken
	nick     map[string]string
	imported map[string]bool // packages imported by a defpackage
	refs     map[string]bool
	current  string // the package the first in-package names
}

// maxDepth bounds how deeply top-level forms (progn, eval-when, let ...)
// are followed.
const maxDepth = 64

// read reads a Lisp source or .asd file.
//
// Implements: REQ-COMMONLISP-002, REQ-COMMONLISP-003, REQ-COMMONLISP-004
func read(src []byte) *info {
	x := &extractor{seen: map[string]bool{}, defined: map[string]bool{}, nick: map[string]string{}, imported: map[string]bool{}, refs: map[string]bool{}}
	forms := Read(src)
	for _, f := range forms {
		x.top(f, 0)
	}
	x.qualified(forms)
	return &x.info
}

func (x *extractor) add(spec, module, name string, line int) {
	if module == "" || x.seen[spec] {
		return
	}
	x.seen[spec] = true
	x.imports = append(x.imports, lang.RawImport{Spec: spec, Module: module, Name: name, Line: line})
}

func (x *extractor) symbol(name, kind string, line int) {
	if name == "" || x.defined[name] {
		return
	}
	x.defined[name] = true
	x.symbols = append(x.symbols, lang.Symbol{Name: name, Kind: kind, Line: line})
}

// name is the lower-case name a string designator gives: a symbol (without
// its package), a keyword, #:uninterned or a string.
func name(n *Node) string {
	if n == nil {
		return ""
	}
	switch n.Kind {
	case Symbol, Keyword, String:
		return strings.ToLower(n.Text)
	}
	return ""
}

// head is the lower-case name of the symbol a plain list starts with, its
// package prefix dropped (asdf:defsystem is defsystem).
func head(n *Node) string {
	if n == nil || n.Kind != List || n.Tag != "" || len(n.Kids) == 0 {
		return ""
	}
	if k := n.Kids[0]; k.Kind == Symbol || k.Kind == Keyword {
		return strings.ToLower(k.Text)
	}
	return ""
}

// unquote strips quote and function wrappers: 'x and #'x are x.
func unquote(n *Node) *Node {
	for n != nil && len(n.Kids) == 2 && (head(n) == "quote" || head(n) == "function") && n.Kids[0].Pkg == "" {
		n = n.Kids[1]
	}
	return n
}

// words renders a symbol or a list of them for a method's qualifiers and
// specializers: (eql :x), (setf name). Lists nested deeper than two levels,
// or longer than eight elements, are cut short with "...".
func words(n *Node) string { return wordsAt(n, 0) }

func wordsAt(n *Node, depth int) string {
	switch n.Kind {
	case List:
		if depth >= 2 {
			return "(...)"
		}
		var parts []string
		for i, k := range n.Kids {
			if i == 8 {
				parts = append(parts, "...")
				break
			}
			parts = append(parts, wordsAt(k, depth+1))
		}
		return "(" + strings.Join(parts, " ") + ")"
	case Keyword:
		return ":" + n.Text
	case String:
		return `"` + n.Text + `"`
	}
	return n.Text
}

// definers are the defining forms whose names are symbols, with their kinds.
var definers = map[string]string{
	"defun": "func", "defmacro": "macro", "defgeneric": "func", "defclass": "class",
	"define-condition": "class", "defstruct": "struct", "deftype": "type",
	"defvar": "var", "defparameter": "var", "defconstant": "const",
}

// top reads a top-level form and the forms a progn, eval-when, let or
// conditional around them keeps at top level.
func (x *extractor) top(n *Node, depth int) {
	h := head(n)
	if h == "" || depth > maxDepth {
		return
	}
	kids := n.Kids
	switch h {
	case "progn", "locally":
		x.body(kids[1:], depth)
	case "eval-when", "when", "unless", "let", "let*", "macrolet", "symbol-macrolet", "flet", "labels":
		if len(kids) > 2 {
			x.body(kids[2:], depth)
		}
	case "in-package":
		if len(kids) > 1 {
			if p := name(unquote(kids[1])); p != "" {
				if x.current == "" {
					x.current = p
				}
				x.add("in-package "+p, p, kindInPackage, n.Line)
			}
		}
	case "defpackage", "define-package":
		x.defpackage(n)
	case "defsystem":
		x.defsystem(n)
	case "register-system-packages":
		// (register-system-packages "closer-mop" '(:c2mop))
		if len(kids) > 2 {
			sys := name(kids[1])
			ps := unquote(kids[2])
			if ps.Kind != List {
				ps = &Node{Kind: List, Kids: []*Node{ps}}
			}
			for _, p := range ps.Kids {
				if pn := name(p); pn != "" && sys != "" {
					if x.registered == nil {
						x.registered = map[string]string{}
					}
					x.registered[pn] = sys
				}
			}
		}
	case "load":
		if len(kids) > 1 && kids[1].Kind == String && kids[1].Text != "" {
			x.add(`load "`+kids[1].Text+`"`, kids[1].Text, kindLoad, n.Line)
		}
	case "quickload", "load-system", "load-systems", "require-system", "test-system", "make":
		if h == "make" && !strings.EqualFold(kids[0].Pkg, "asdf") {
			return
		}
		for _, a := range kids[1:] {
			a = unquote(a)
			if a.Kind == Keyword && len(a.Text) > 0 && (a.Text == "verbose" || a.Text == "silent" || a.Text == "prompt" || a.Text == "force") {
				break // the keyword arguments after the systems
			}
			x.systemArgs(h, a, n.Line)
		}
	case "operate", "oos":
		if len(kids) > 2 {
			x.systemArgs(h, unquote(kids[2]), n.Line)
		}
	case "require":
		if len(kids) > 1 {
			if m := name(unquote(kids[1])); m != "" {
				x.add("require "+m, m, kindRequire, n.Line)
			}
		}
	case "defmethod":
		x.defmethod(n)
	default:
		kind, ok := definers[h]
		if !ok || len(kids) < 2 {
			return
		}
		target := kids[1]
		if h == "defstruct" && target.Kind == List && len(target.Kids) > 0 {
			target = target.Kids[0] // (defstruct (point (:conc-name p-)) ...)
		}
		switch {
		case target.Kind == Symbol:
			x.symbol(target.Text, kind, n.Line)
		case target.Kind == List && h == "defun" || target.Kind == List && h == "defgeneric":
			x.symbol(words(target), kind, n.Line) // (defun (setf name) ...)
		}
	}
}

func (x *extractor) body(forms []*Node, depth int) {
	for _, f := range forms {
		x.top(f, depth+1)
	}
}

// systemArgs reads the systems a quickload or load-system names: one
// designator or a list of them.
func (x *extractor) systemArgs(verb string, a *Node, line int) {
	list := []*Node{a}
	if a.Kind == List && a.Tag == "" {
		list = a.Kids
	}
	for _, s := range list {
		if sys := name(unquote(s)); sys != "" {
			x.add(verb+" "+sys, sys, kindSystem+"\x00", line)
		}
	}
}

// defmethod names a method by its generic function, qualifiers and
// specializers: print-object (point t), initialize-instance :after (shop).
func (x *extractor) defmethod(n *Node) {
	kids := n.Kids
	if len(kids) < 2 {
		return
	}
	var fn string
	switch kids[1].Kind {
	case Symbol:
		fn = kids[1].Text
	case List:
		fn = words(kids[1]) // (setf name)
	default:
		return
	}
	var quals []string
	var specs []string
	for _, k := range kids[2:] {
		if k.Kind != List {
			quals = append(quals, words(k))
			continue
		}
		for _, arg := range k.Kids {
			if arg.Kind == Symbol && strings.HasPrefix(arg.Text, "&") {
				break
			}
			if arg.Kind == List && len(arg.Kids) > 1 {
				specs = append(specs, words(arg.Kids[1]))
			} else {
				specs = append(specs, "t")
			}
		}
		break
	}
	sig := fn
	if len(quals) > 0 {
		sig += " " + strings.Join(quals, " ")
	}
	x.symbol(sig+" ("+strings.Join(specs, " ")+")", "method", n.Line)
}

// defpackageOptions are the options of defpackage and uiop:define-package
// that name other packages.
var defpackageOptions = map[string]bool{
	"use": true, "import-from": true, "shadowing-import-from": true, "local-nicknames": true,
	"mix": true, "reexport": true, "use-reexport": true, "mix-reexport": true,
}

// defpackage reads a package definition: its name and nicknames, and the
// packages it uses, imports from or nicknames locally.
func (x *extractor) defpackage(n *Node) {
	kids := n.Kids
	if len(kids) < 2 {
		return
	}
	pn := name(kids[1])
	if pn == "" {
		return
	}
	def := pkgDef{names: []string{pn}, line: n.Line}
	x.symbol(pn, "package", n.Line)
	for _, opt := range kids[2:] {
		if opt.Kind != List || len(opt.Kids) == 0 {
			continue
		}
		o := name(opt.Kids[0])
		args := opt.Kids[1:]
		switch {
		case o == "nicknames":
			for _, a := range args {
				if nn := name(a); nn != "" {
					def.names = append(def.names, nn)
				}
			}
		case o == "local-nicknames":
			for _, a := range args {
				if a.Kind == List && len(a.Kids) == 2 {
					nick, target := name(a.Kids[0]), name(a.Kids[1])
					if nick != "" && target != "" {
						x.nick[nick] = target
						if def.nicks == nil {
							def.nicks = map[string]string{}
						}
						def.nicks[nick] = target
						x.imported[target] = true
						x.add("local-nicknames "+target, target, kindPackage, a.Line)
					}
				}
			}
		case o == "import-from" || o == "shadowing-import-from":
			if len(args) > 0 {
				if p := name(args[0]); p != "" {
					x.imported[p] = true
					x.add(o+" "+p, p, kindPackage, opt.Line)
				}
			}
		case defpackageOptions[o]:
			for _, a := range args {
				if p := name(a); p != "" {
					x.imported[p] = true
					x.add(o+" "+p, p, kindPackage, a.Line)
				}
			}
		}
	}
	x.packages = append(x.packages, def)
}

// defsystem reads an ASDF system definition: its name, :pathname, :class,
// dependencies and components.
func (x *extractor) defsystem(n *Node) {
	kids := n.Kids
	if len(kids) < 2 {
		return
	}
	sn := name(kids[1])
	if sn == "" {
		return
	}
	s := &system{name: sn, line: n.Line, base: "."}
	x.symbol(sn, "system", n.Line)
	opts := plist(kids[2:])
	if p := pathname(opts["pathname"]); p != "" {
		s.base = path.Clean(p)
	}
	if c := opts["class"]; c != nil && strings.Contains(name(unquote(c)), "package-inferred-system") {
		s.pis = true
	}
	for _, opt := range []string{"defsystem-depends-on", "depends-on", "weakly-depends-on"} {
		if l := opts[opt]; l != nil && l.Kind == List {
			for _, d := range l.Kids {
				x.dependency(s, opt, d)
			}
		}
	}
	if c := opts["components"]; c != nil {
		x.components(s, c, s.base, 0)
	}
	x.systems = append(x.systems, s)
}

// plist reads keyword options: (:key value ...).
func plist(kids []*Node) map[string]*Node {
	out := map[string]*Node{}
	for i := 0; i+1 < len(kids); i += 2 {
		if kids[i].Kind != Keyword {
			i-- // out of step: look for the next keyword
			continue
		}
		if k := strings.ToLower(kids[i].Text); out[k] == nil {
			out[k] = kids[i+1]
		}
	}
	return out
}

// pathname is a :pathname option's value when it is a string or #p"...".
func pathname(n *Node) string {
	if n == nil || n.Kind != String {
		return ""
	}
	return n.Text
}

// dependency reads one :depends-on entry: a system name, (:version name v),
// (:feature f entry) or (:require module).
func (x *extractor) dependency(s *system, opt string, d *Node) {
	d = unquote(d)
	var dep sysDep
	switch d.Kind {
	case Symbol, Keyword, String:
		dep = sysDep{name: name(d)}
	case List:
		if len(d.Kids) < 2 || d.Tag != "" {
			return
		}
		switch name(d.Kids[0]) {
		case "version":
			if len(d.Kids) < 3 {
				return
			}
			dep = sysDep{name: name(d.Kids[1]), version: name(d.Kids[2])}
		case "feature":
			if len(d.Kids) == 3 {
				x.dependency(s, opt, d.Kids[2])
			}
			return
		case "require":
			dep = sysDep{name: name(d.Kids[1]), require: true}
		default:
			return
		}
	default:
		return
	}
	if dep.name == "" {
		return
	}
	dep.option, dep.line = opt, d.Line
	s.deps = append(s.deps, dep)
	if dep.require {
		x.add("require "+dep.name, dep.name, kindRequire, d.Line)
		return
	}
	v := ""
	if dep.version != "" {
		v = ">= " + dep.version
	}
	x.add(opt+" "+dep.name, dep.name, kindSystem+"\x00"+v, d.Line)
}

// fileTypes are the file extensions of ASDF's component types; a type not
// listed is a Lisp source file (cffi-grovel-file and the like).
var fileTypes = map[string]string{
	"static-file": "", "doc-file": "", "html-file": ".html", "c-source-file": ".c",
	"java-source-file": ".java", "cl-source-file.cl": ".cl", "cl-source-file.lsp": ".lsp",
	"jar-file": ".jar",
}

// components reads a :components list whose paths are relative to dir,
// recursing into modules.
func (x *extractor) components(s *system, list *Node, dir string, depth int) {
	if list.Kind != List || depth > maxDepth {
		return
	}
	for _, c := range list.Kids {
		if c.Kind == String {
			c = &Node{Kind: List, Line: c.Line, Kids: []*Node{{Kind: Keyword, Text: "file"}, c}}
		}
		if c.Kind != List || c.Tag != "" || len(c.Kids) < 2 {
			continue
		}
		typ, cn := name(c.Kids[0]), c.Kids[1]
		if cn.Kind != String && cn.Kind != Symbol && cn.Kind != Keyword {
			continue
		}
		written := cn.Text
		if cn.Kind != String {
			written = strings.ToLower(written) // ASDF downcases a symbol's name
		}
		opts := plist(c.Kids[2:])
		p, hasPath := written, false
		if pn := opts["pathname"]; pn != nil && pn.Kind == String {
			p, hasPath = pn.Text, true
		}
		if sub := opts["components"]; typ == "module" || sub != nil {
			d := path.Join(dir, p)
			if sub != nil {
				x.components(s, sub, d, depth+1)
			}
			continue
		}
		if p == "" {
			continue
		}
		ext, known := fileTypes[typ]
		if !known {
			ext = ".lisp"
		}
		if t := opts["type"]; t != nil && t.Kind == String {
			ext = "." + t.Text
		}
		if !(hasPath && path.Ext(p) != "") && ext != "" {
			p += ext
		}
		rel := path.Join(dir, p)
		s.files = append(s.files, rel)
		x.add("component "+rel, rel, kindComponent, c.Line)
	}
}

// qualified records the packages of qualified symbols (pkg:sym, pkg::sym)
// the file writes, once each, a local nickname read as its package; a
// package the file's defpackage already imports is not repeated. The
// package the file is in rides along, for the local nicknames its
// defpackage (in another file) gives.
func (x *extractor) qualified(forms []*Node) {
	stack := append([]*Node(nil), forms...)
	var order []*Node
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n.Kind == List {
			for i := len(n.Kids) - 1; i >= 0; i-- {
				stack = append(stack, n.Kids[i])
			}
			continue
		}
		if n.Kind == Symbol && n.Pkg != "" {
			order = append(order, n)
		}
	}
	for _, n := range order {
		written := strings.ToLower(n.Pkg)
		p := written
		if t, ok := x.nick[p]; ok {
			p = t
		}
		if x.imported[p] || x.refs[written] {
			continue
		}
		x.refs[written] = true
		x.add(written+":", p, kindRef+"\x00"+x.current, n.Line)
	}
}

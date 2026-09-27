package nix

import (
	"path"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Import kinds (RawImport.Name).
const (
	kImport  = "import"  // import / callPackage / imports = [...]: a directory means its default.nix
	kPath    = "path"    // any other path literal: a file or directory
	kChannel = "channel" // <nixpkgs>
	kFetch   = "fetch"   // builtins.fetchTarball / fetchGit / fetchTree / getFlake; Module = ref.encode()
	kInput   = "input"   // a flake input; Module = name \n ref.encode() \n follows
	kUse     = "use"     // inputs.<name> outside flake.nix; Module = name
	kPin     = "pin"     // sources.<name> of niv or npins; Module = sources dir \n name
	kPkg     = "pkg"     // a nixpkgs attribute in a package list; Module = attribute path
)

// scope is a lexical scope of the walk: names bound by let, rec, a lambda, or a
// with whose attributes are nixpkgs'.
type scope struct {
	parent  *scope
	names   map[string]bool
	formals bool   // names are the file's callPackage-style formals: packages
	with    bool   // a with over nixpkgs
	prefix  string // what the with's attributes are under: "" or "python3Packages."
}

// lookup says how a bare name resolves: "formal", "bound", "with" (through a with
// over nixpkgs, with its prefix) or "" (unknown). Lexical bindings win over any
// with, as in Nix.
func (s *scope) lookup(name string) (string, string) {
	var with *scope
	for c := s; c != nil; c = c.parent {
		if c.names[name] {
			if c.formals {
				return "formal", ""
			}
			return "bound", ""
		}
		if c.with && with == nil {
			with = c
		}
	}
	if with != nil {
		return "with", with.prefix
	}
	return "", ""
}

type extractor struct {
	flake    bool
	imports  []lang.RawImport
	seen     map[string]bool
	consumed map[*node]bool
	sources  map[string]string // name bound to niv's or npins' import -> sources dir as written
	symbols  []lang.Symbol
	symSeen  map[string]bool
	visits   int
}

// maxVisits bounds the walk; a tree is at most a few nodes per token, so this is
// only reached by pathological input.
const maxVisits = 4_000_000

func extract(src []byte, flake bool) *lang.Extraction {
	root := parse(src)
	e := &extractor{flake: flake, seen: map[string]bool{}, consumed: map[*node]bool{}, sources: map[string]string{}, symSeen: map[string]bool{}}
	if flake {
		e.flakeInputs(root)
		e.flakeSymbols(root)
	} else {
		e.fileSymbols(root)
	}
	e.walk(root, nil, true, 0)
	sort.SliceStable(e.imports, func(i, j int) bool {
		if e.imports[i].Line != e.imports[j].Line {
			return e.imports[i].Line < e.imports[j].Line
		}
		return e.imports[i].Spec < e.imports[j].Spec
	})
	var set lang.SymbolSet
	for _, s := range e.symbols {
		set.Add(s.Name, s.Kind, s.Line)
	}
	symbols := set.List()
	if symbols == nil {
		symbols = []lang.Symbol{}
	}
	return &lang.Extraction{Imports: e.imports, Symbols: symbols}
}

func (e *extractor) emit(kind, spec, module string, line int) {
	key := kind + "\x00" + module
	switch kind {
	case kInput:
		key = kind + "\x00" + spec
	case kImport: // one edge per path literal, whichever way it is read first
		key = kPath + "\x00" + module
	}
	if e.seen[key] {
		return
	}
	e.seen[key] = true
	e.imports = append(e.imports, lang.RawImport{Spec: spec, Module: module, Name: kind, Line: line})
}

func (e *extractor) symbol(name, kind string, line int) {
	if name == "" || e.symSeen[name] {
		return
	}
	e.symSeen[name] = true
	e.symbols = append(e.symbols, lang.Symbol{Name: name, Kind: kind, Line: line})
}

func unparen(n *node) *node {
	for n != nil && n.kind == nParen && len(n.kids) > 0 {
		n = n.kids[0]
	}
	return n
}

func static(p []string) bool {
	if len(p) == 0 {
		return false
	}
	for _, s := range p {
		if s == "" {
			return false
		}
	}
	return true
}

// fnName is the name an application calls: `import`, `callPackage` of
// `pkgs.callPackage`, `fetchTarball` of `builtins.fetchTarball`.
func fnName(n *node) (name string, builtin bool) {
	n = unparen(n)
	switch n.kind {
	case nIdent:
		return n.text, false
	case nSelect:
		if len(n.path) > 0 && n.path[len(n.path)-1] != "" {
			b := unparen(n.kids[0])
			return n.path[len(n.path)-1], b.kind == nIdent && b.text == "builtins" && len(n.path) == 1
		}
	}
	return "", false
}

func (e *extractor) walk(n *node, sc *scope, top bool, depth int) {
	if n == nil || depth > 2*maxDepth {
		return
	}
	if e.visits++; e.visits > maxVisits {
		return
	}
	d := depth + 1
	switch n.kind {
	case nSelect:
		if b := unparen(n.kids[0]); b.kind == nIdent && len(n.path) > 0 && n.path[0] != "" {
			if b.text == "inputs" && !e.flake {
				e.emit(kUse, "inputs."+n.path[0], n.path[0], n.line)
			}
			if dir, ok := e.sources[b.text]; ok {
				e.emit(kPin, b.text+"."+n.path[0], dir+"\n"+n.path[0], n.line)
			}
		}
	case nPath:
		if !n.interpolate && !e.consumed[n] {
			e.path(kPath, n)
		}
	case nSPath:
		e.emit(kChannel, "<"+n.text+">", strings.Split(n.text, "/")[0], n.line)
	case nAttrs:
		inner := sc
		if n.rec {
			inner = &scope{parent: sc, names: bindNames(n.binds)}
		}
		e.sourcesOf(n.binds)
		for _, b := range n.binds {
			e.bind(b, inner, d)
		}
		return
	case nLet:
		inner := &scope{parent: sc, names: bindNames(n.binds)}
		e.sourcesOf(n.binds)
		for _, b := range n.binds {
			e.bind(b, inner, d)
		}
		for _, k := range n.kids {
			e.walk(k, inner, top, d)
		}
		return
	case nWith:
		e.walk(n.kids[0], sc, false, d)
		inner := &scope{parent: sc}
		inner.with, inner.prefix = pkgsEnv(n.kids[0])
		if len(n.kids) > 1 {
			e.walk(n.kids[1], inner, top, d)
		}
		return
	case nLambda:
		names := map[string]bool{}
		for _, f := range n.formals {
			names[f] = true
		}
		if n.text != "" {
			names[n.text] = true
		}
		e.sourcesOf(n.binds)
		for _, b := range n.binds { // defaults: `pkgs ? import <nixpkgs> { }`
			e.walk(b.value, sc, false, d)
		}
		inner := &scope{parent: sc, names: names, formals: top && n.set && !e.flake}
		e.walk(n.kids[0], inner, top, d)
		return
	case nApply:
		e.apply(n)
	}
	for _, k := range n.kids {
		e.walk(k, sc, false, d)
	}
}

func bindNames(bs []*bind) map[string]bool {
	m := map[string]bool{}
	for _, b := range bs {
		if len(b.path) > 0 && b.path[0] != "" {
			m[b.path[0]] = true
		}
	}
	return m
}

// packageLists are the attributes whose lists name nixpkgs packages a project
// builds with or installs.
var packageLists = map[string]bool{
	"buildInputs": true, "nativeBuildInputs": true, "propagatedBuildInputs": true,
	"propagatedNativeBuildInputs": true, "checkInputs": true, "nativeCheckInputs": true,
	"packages": true, "systemPackages": true,
}

func (e *extractor) bind(b *bind, sc *scope, depth int) {
	if b.value == nil {
		return
	}
	if !b.inherit && len(b.path) > 0 {
		switch last := b.path[len(b.path)-1]; {
		case packageLists[last]:
			e.pkgList(b.value, sc, 0)
		case last == "imports":
			e.moduleImports(b.value, 0)
		}
	}
	e.walk(b.value, sc, false, depth)
}

// moduleImports reads a NixOS module's `imports = [ ./a.nix ./b ]` (lists joined
// with ++ too): paths there are modules, a directory meaning its default.nix.
func (e *extractor) moduleImports(v *node, depth int) {
	v = unparen(v)
	if v == nil || depth > 16 {
		return
	}
	switch v.kind {
	case nList:
		for _, k := range v.kids {
			if k = unparen(k); k.kind == nPath && !k.interpolate {
				e.consumed[k] = true
				e.path(kImport, k)
			}
		}
	case nBinary:
		if v.text == "++" {
			for _, k := range v.kids {
				e.moduleImports(k, depth+1)
			}
		}
	}
}

// path records a path literal: relative ones only (an absolute path or ~/ is the
// machine's, not the repository's), and not the file's own directory.
func (e *extractor) path(kind string, n *node) {
	p := n.text
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "~") {
		return
	}
	if c := path.Clean(p); c == "." {
		return
	}
	e.emit(kind, p, p, n.line)
}

// apply handles the applications that name something: import and callPackage of a
// path, and Nix's own fetchers with literal arguments (not nixpkgs' fetchers of
// package sources).
//
// Implements: REQ-NIX-002, REQ-NIX-004, REQ-NIX-010
func (e *extractor) apply(n *node) {
	name, builtin := fnName(n.kids[0])
	args := n.kids[1:]
	if len(args) == 0 {
		return
	}
	switch name {
	case "import", "callPackage", "callPackages", "callPackageWith", "callPackagesWith":
		a := args[0]
		if strings.HasSuffix(name, "With") {
			if len(args) < 2 {
				return
			}
			a = args[1]
		}
		if a = unparen(a); a.kind == nPath && !a.interpolate {
			e.consumed[a] = true
			e.path(kImport, a)
		}
	case "fetchTarball", "fetchGit", "fetchTree", "getFlake", "fetchMercurial":
		if fn := unparen(n.kids[0]); !builtin && fn.kind != nIdent {
			return // pkgs.fetchgit and friends fetch a package's sources
		}
		a := unparen(args[0])
		var r ref
		spec := ""
		switch {
		case a.kind == nString && !a.interpolate || a.kind == nURI:
			spec = a.text
			switch name {
			case "fetchTarball":
				r = ref{typ: "tarball", url: a.text}
			case "fetchGit":
				r = ref{typ: "git", url: a.text}
			case "fetchMercurial":
				r = ref{typ: "hg", url: a.text}
			default:
				r = parseRef(a.text)
			}
		case a.kind == nAttrs:
			f := literalFields(a)
			switch name {
			case "fetchTarball":
				r = ref{typ: "tarball", url: f["url"], narHash: f["sha256"]}
			case "fetchGit", "fetchMercurial":
				r = ref{typ: "git", url: f["url"], ref: f["ref"], rev: f["rev"], narHash: f["narHash"]}
				if name == "fetchMercurial" {
					r.typ = "hg"
				}
			default:
				r = attrsRef(f)
			}
		default:
			return
		}
		switch r.typ {
		case "github", "gitlab", "sourcehut":
			if r.owner == "" || r.repo == "" {
				return // computed: owner = gh.owner
			}
		case "indirect":
			if r.id == "" {
				return
			}
		default:
			if r.url == "" {
				return
			}
		}
		e.emit(kFetch, name+" "+first(spec, r.url, r.owner+"/"+r.repo, r.id), r.encode(), n.line)
	}
}

// literalFields reads an attribute set's string (and boolean) values.
func literalFields(a *node) map[string]string {
	f := map[string]string{}
	for _, b := range a.binds {
		if len(b.path) != 1 || b.value == nil || b.inherit {
			continue
		}
		switch v := unparen(b.value); {
		case v.kind == nString && !v.interpolate, v.kind == nURI:
			f[b.path[0]] = v.text
		case v.kind == nIdent && (v.text == "true" || v.text == "false"):
			f[b.path[0]] = v.text
		case v.kind == nPath && !v.interpolate:
			f[b.path[0]] = v.text
		}
	}
	return f
}

// sourcesOf notes bindings of niv's or npins' sources: `sources = import
// ./nix/sources.nix;`, `pins = import ./npins;`.
func (e *extractor) sourcesOf(bs []*bind) {
	for _, b := range bs {
		if b.inherit || len(b.path) != 1 || b.path[0] == "" {
			continue
		}
		v := unparen(b.value)
		if v == nil || v.kind != nApply {
			continue
		}
		if name, _ := fnName(v.kids[0]); name != "import" || len(v.kids) < 2 {
			continue
		}
		p := unparen(v.kids[1])
		if p.kind != nPath || p.interpolate {
			continue
		}
		c := path.Clean(p.text)
		switch {
		case path.Base(c) == "sources.nix":
			e.sources[b.path[0]] = path.Dir(p.text)
		case path.Base(c) == "npins":
			e.sources[b.path[0]] = p.text
		case strings.HasSuffix(c, "npins/default.nix"):
			e.sources[b.path[0]] = path.Dir(p.text)
		}
	}
}

// pkgsEnv reports whether a with's environment is nixpkgs (pkgs, import <nixpkgs>
// {}, nixpkgs.legacyPackages.x86_64-linux) or a package set in it
// (pkgs.python3Packages, python3Packages), and the attribute prefix of its names.
func pkgsEnv(env *node) (bool, string) {
	env = unparen(env)
	switch env.kind {
	case nIdent:
		if pkgsName(env.text) {
			return true, ""
		}
		if strings.HasSuffix(env.text, "Packages") && env.text != "legacyPackages" {
			return true, env.text + "."
		}
	case nSelect:
		b := unparen(env.kids[0])
		if b.kind != nIdent || !static(env.path) {
			if b.kind == nIdent && len(env.path) > 0 && env.path[0] == "legacyPackages" && (pkgsName(b.text) || b.text == "nixpkgs") {
				return true, ""
			}
			return false, ""
		}
		p := env.path
		if p[0] == "legacyPackages" && (pkgsName(b.text) || b.text == "nixpkgs") {
			if len(p) <= 2 {
				return true, ""
			}
			p = p[2:]
		} else if !pkgsName(b.text) {
			return false, ""
		}
		return true, strings.Join(p, ".") + "."
	case nApply:
		if name, _ := fnName(env.kids[0]); name == "import" && len(env.kids) > 1 {
			a := unparen(env.kids[1])
			return a.kind == nSPath && strings.HasPrefix(a.text, "nixpkgs") || a.kind == nIdent && a.text == "nixpkgs", ""
		}
	}
	return false, ""
}

// pkgsName is a name conventionally bound to a nixpkgs package set: pkgs,
// unstablePkgs, pkgs-unstable, pkgs'.
func pkgsName(s string) bool { return strings.Contains(strings.ToLower(s), "pkgs") }

// notPackages are names a package list may hold that are not nixpkgs packages.
var notPackages = map[string]bool{
	"stdenv": true, "lib": true, "config": true, "options": true, "pkgs": true, "self": true,
	"super": true, "final": true, "prev": true, "inputs": true, "system": true, "builtins": true,
	"callPackage": true, "modulesPath": true, "specialArgs": true, "true": true, "false": true,
	"null": true, "import": true, "stdenvNoCC": true,
}

// wrappers are the calls around a package list: lib.optionals cond [ ... ].
var wrappers = map[string]bool{"optionals": true, "optional": true, "mkIf": true, "mkDefault": true,
	"mkForce": true, "mkBefore": true, "mkAfter": true, "mkOverride": true, "mkOrder": true}

// pkgList reads the nixpkgs packages of a package list's value: list literals
// joined with ++, under with pkgs;, in both branches of an if and inside
// lib.optionals and mkIf.
//
// Implements: REQ-NIX-007
func (e *extractor) pkgList(v *node, sc *scope, depth int) {
	v = unparen(v)
	if v == nil || depth > 32 {
		return
	}
	switch v.kind {
	case nList:
		for _, k := range v.kids {
			e.pkgElem(k, sc, true)
		}
	case nBinary:
		if v.text == "++" {
			for _, k := range v.kids {
				e.pkgList(k, sc, depth+1)
			}
		}
	case nWith:
		inner := &scope{parent: sc}
		inner.with, inner.prefix = pkgsEnv(v.kids[0])
		if len(v.kids) > 1 {
			e.pkgList(v.kids[1], inner, depth+1)
		}
	case nLet:
		inner := &scope{parent: sc, names: bindNames(v.binds)}
		if len(v.kids) > 0 {
			e.pkgList(v.kids[0], inner, depth+1)
		}
	case nIf:
		for _, k := range v.kids[min(1, len(v.kids)):] {
			e.pkgList(k, sc, depth+1)
		}
	case nApply:
		if name, _ := fnName(v.kids[0]); wrappers[name] && len(v.kids) > 1 {
			last := v.kids[len(v.kids)-1]
			if name == "optional" {
				e.pkgElem(last, sc, true)
			} else {
				e.pkgList(last, sc, depth+1)
			}
		}
	}
}

// strip are the calls a package goes through in a list that still name it:
// pkgs.hello.override { }, python3.withPackages (ps: [ ]).
var strip = map[string]bool{"override": true, "overrideAttrs": true, "overrideDerivation": true,
	"overridePythonAttrs": true, "withPackages": true, "withPlugins": true, "withExtensions": true}

func (e *extractor) pkgElem(n *node, sc *scope, bare bool) {
	n = unparen(n)
	switch n.kind {
	case nApply:
		// Only a package's own call (pkgs.hello.override ...); a bare function
		// (writeShellScriptBin "x" ...) makes a new derivation.
		if f := unparen(n.kids[0]); f.kind == nSelect && len(f.path) > 0 && strip[f.path[len(f.path)-1]] {
			e.pkgElem(f, sc, false)
		}
	case nIdent:
		if !bare || notPackages[n.text] {
			return
		}
		switch kind, prefix := sc.lookup(n.text); kind {
		case "formal", "with":
			e.emit(kPkg, n.text, prefix+n.text, n.line)
		}
	case nSelect:
		b := unparen(n.kids[0])
		if b.kind != nIdent || !static(n.path) || notPackages[b.text] && b.text != "pkgs" {
			return
		}
		p := n.path
		for len(p) > 0 && strip[p[len(p)-1]] {
			p = p[:len(p)-1]
		}
		spec := b.text + "." + strings.Join(p, ".")
		if len(p) == 0 {
			return
		}
		if len(p) >= 3 && p[0] == "legacyPackages" && (pkgsName(b.text) || b.text == "nixpkgs") {
			p = p[2:]
		} else if kind, prefix := sc.lookup(b.text); !pkgsName(b.text) {
			if kind != "formal" && kind != "with" {
				return
			}
			p = append(strings.Split(prefix+b.text, "."), p...)
		}
		e.emit(kPkg, spec, strings.Join(p, "."), n.line)
	}
}

// fileSymbols: the file's top-level let bindings and the attributes of the
// attribute set it returns (through lambdas, let, with, assert and //), their
// paths cut at two names.
//
// Implements: REQ-NIX-003
func (e *extractor) fileSymbols(root *node) {
	n := e.unwrapTop(root, true)
	for _, a := range returned(n, false, 0) {
		for _, b := range a.binds {
			if !static(b.path) {
				continue
			}
			name := strings.Join(b.path[:min(2, len(b.path))], ".")
			e.symbol(name, kindOf(b), b.line)
		}
	}
}

func kindOf(b *bind) string {
	if v := unparen(b.value); v != nil && v.kind == nLambda && !b.inherit {
		return "function"
	}
	return "attr"
}

// unwrapTop steps through the file's function head, with, assert and let (whose
// bindings become symbols when lets is set) to what the file returns.
func (e *extractor) unwrapTop(n *node, lets bool) *node {
	for i := 0; n != nil && i < 64; i++ {
		n = unparen(n)
		switch n.kind {
		case nLambda:
			n = n.kids[0]
		case nWith, nAssert:
			if len(n.kids) < 2 {
				return n
			}
			n = n.kids[1]
		case nLet:
			if lets {
				for _, b := range n.binds {
					if static(b.path) {
						kind := "var"
						if v := unparen(b.value); v != nil && v.kind == nLambda {
							kind = "function"
						}
						e.symbol(b.path[0], kind, b.line)
					}
				}
			}
			if len(n.kids) == 0 {
				return n
			}
			n = n.kids[0]
		default:
			return n
		}
	}
	return n
}

// returned lists the attribute sets an expression evaluates to: both sides of //,
// both branches of an if, and with calls the last argument (flake-utils'
// eachDefaultSystem (system: { ... })).
func returned(n *node, calls bool, depth int) []*node {
	n = unparen(n)
	if n == nil || depth > 8 {
		return nil
	}
	switch n.kind {
	case nAttrs:
		return []*node{n}
	case nBinary:
		if n.text == "//" {
			var out []*node
			for _, k := range n.kids {
				out = append(out, returned(k, calls, depth+1)...)
			}
			return out
		}
	case nIf:
		var out []*node
		for _, k := range n.kids[min(1, len(n.kids)):] {
			out = append(out, returned(k, calls, depth+1)...)
		}
		return out
	case nLambda, nLet:
		return returned(n.kids[0], calls, depth+1)
	case nWith, nAssert:
		if len(n.kids) > 1 {
			return returned(n.kids[1], calls, depth+1)
		}
	case nApply:
		if calls {
			return returned(n.kids[len(n.kids)-1], calls, depth+1)
		}
	}
	return nil
}

// topAttrs is the attribute set a flake.nix consists of.
func topAttrs(root *node) *node {
	n := unparen(root)
	if n != nil && n.kind == nAttrs {
		return n
	}
	return nil
}

// flakeSymbols: the flake's outputs, as paths of up to two names
// (packages.x86_64-linux, nixosModules.default, or packages.default under
// flake-utils).
//
// Implements: REQ-NIX-003
func (e *extractor) flakeSymbols(root *node) {
	top := topAttrs(root)
	if top == nil {
		return
	}
	for _, b := range top.binds {
		if len(b.path) == 0 || b.path[0] != "outputs" {
			continue
		}
		if len(b.path) > 1 { // outputs.x = ...: not a function, but read it anyway
			e.symbol(strings.Join(b.path[1:min(3, len(b.path))], "."), "output", b.line)
			continue
		}
		for _, a := range returned(b.value, true, 0) {
			for _, ob := range a.binds {
				if !static(ob.path) {
					continue
				}
				e.symbol(ob.path[0], "output", ob.line)
				if len(ob.path) >= 2 {
					e.symbol(ob.path[0]+"."+ob.path[1], "output", ob.line)
					continue
				}
				for _, sub := range returned(ob.value, true, 0) {
					for _, sb := range sub.binds {
						if len(sb.path) > 0 && sb.path[0] != "" {
							e.symbol(ob.path[0]+"."+sb.path[0], "output", sb.line)
						}
					}
				}
			}
		}
	}
}

type input struct {
	name    string
	line    int
	fields  map[string]string
	follows string
}

// flakeInputs reads flake.nix's inputs (`inputs.x.url = ...;`, `inputs = { x = {
// url; flake = false; inputs.y.follows = "y"; }; }`, attribute-set references)
// and the outputs function's arguments no input declares, which the flake
// registry supplies.
//
// Implements: REQ-NIX-004
func (e *extractor) flakeInputs(root *node) {
	top := topAttrs(root)
	if top == nil {
		return
	}
	var order []*input
	byName := map[string]*input{}
	get := func(name string, line int) *input {
		in := byName[name]
		if in == nil {
			in = &input{name: name, line: line, fields: map[string]string{}}
			byName[name] = in
			order = append(order, in)
		}
		return in
	}
	var field func(in *input, p []string, v *node)
	field = func(in *input, p []string, v *node) {
		v = unparen(v)
		if v == nil {
			return
		}
		if len(p) == 0 {
			if v.kind == nAttrs {
				for _, b := range v.binds {
					if static(b.path) && !b.inherit {
						field(in, b.path, b.value)
					}
				}
			} else if v.kind == nString && !v.interpolate || v.kind == nURI {
				in.fields["url"] = v.text
			}
			return
		}
		if p[0] == "inputs" { // the input's own inputs: follows, read from the lock
			return
		}
		if len(p) > 1 {
			return
		}
		switch {
		case v.kind == nString && !v.interpolate, v.kind == nURI, v.kind == nPath && !v.interpolate:
			if p[0] == "follows" {
				in.follows = v.text
			} else {
				in.fields[p[0]] = v.text
			}
		case v.kind == nIdent && (v.text == "true" || v.text == "false"):
			in.fields[p[0]] = v.text
		}
	}
	var outputs *node
	for _, b := range top.binds {
		if !static(b.path) || b.inherit {
			continue
		}
		switch b.path[0] {
		case "inputs":
			if len(b.path) == 1 {
				if v := unparen(b.value); v != nil && v.kind == nAttrs {
					for _, ib := range v.binds {
						if static(ib.path) && !ib.inherit {
							field(get(ib.path[0], ib.line), ib.path[1:], ib.value)
						}
					}
				}
				continue
			}
			field(get(b.path[1], b.line), b.path[2:], b.value)
		case "outputs":
			if len(b.path) == 1 {
				outputs = unparen(b.value)
			}
		}
	}
	if outputs != nil && outputs.kind == nLambda && outputs.set {
		for _, f := range outputs.formals {
			if f != "self" && byName[f] == nil {
				in := get(f, outputs.line)
				in.fields["url"] = "flake:" + f
			}
		}
	}
	for _, in := range order {
		var r ref
		if len(in.fields) > 0 {
			r = attrsRef(in.fields)
			if r.typ == "" && r.url == "" && in.follows == "" {
				r = ref{typ: "indirect", id: in.name} // { flake = false; } alone: the registry's
			}
		} else if in.follows == "" {
			r = ref{typ: "indirect", id: in.name}
		}
		e.emit(kInput, "inputs."+in.name, in.name+"\n"+r.encode()+"\n"+in.follows, in.line)
	}
}

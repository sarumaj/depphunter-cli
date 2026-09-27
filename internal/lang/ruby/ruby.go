// Package ruby analyzes Ruby with tree-sitter. `require`, `require_relative`, `load`
// and `autoload` of a path the file spells out resolve to project files on a guessed
// $LOAD_PATH (resolve.go), to Ruby's standard library and default gems (std.go), and
// to gems by what Bundler records - Gemfile, *.gemspec and Gemfile.lock
// (bundler.go). A Gemfile's `gem` lines and a gemspec's add_dependency calls are
// imports of the gems they declare, and in a Rails application constants resolve to
// the files Zeitwerk would load them from.
package ruby

import (
	"cmp"
	"path"
	"slices"
	"strings"

	"github.com/odvcencio/gotreesitter/grammars/ruby"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/treesitter"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecoGems = "rubygems"
	ecoStd  = "ruby-std"
)

// exts are the extensions the plugin claims: Ruby, Rake tasks, gem specifications
// and Rack configurations.
//
// Implements: REQ-RUBY-001
var exts = map[string]bool{".rb": true, ".rake": true, ".gemspec": true, ".ru": true}

// names are the Ruby files known by their name alone: Bundler's, Rake's, Guard's and
// Capistrano's.
//
// Implements: REQ-RUBY-001
var names = map[string]bool{"Gemfile": true, "Rakefile": true, "Guardfile": true, "Capfile": true}

// @fn/@args is a call (its arguments parsed in Go); @recv its receiver when it has
// one; @bare a call without arguments at the top of a file (`gemspec`); @ref a
// constant code names; @def.<kind> a definition; @attr/@asym an attr_* call and one
// of the attributes it defines. Where a definition sits is read from its ancestors.
const query = `
(call !receiver method: (identifier) @fn arguments: (argument_list) @args)
(call receiver: (_) @recv method: (identifier) @fn arguments: (argument_list) @args)
(program (identifier) @bare)

(superclass [(constant) (scope_resolution)] @ref)
(exceptions [(constant) (scope_resolution)] @ref)
(call receiver: [(constant) (scope_resolution)] @ref)
(argument_list [(constant) (scope_resolution)] @ref)
(assignment right: [(constant) (scope_resolution)] @ref)

(module name: (_) @def.module)
(class name: (_) @def.class)
(method name: (_) @def.method)
(singleton_method object: (_) @obj name: (_) @def.smethod)
(assignment left: (constant) @def.const)
(call !receiver method: (identifier) @attr arguments: (argument_list (simple_symbol) @asym))
`

var grammar = treesitter.MustGrammar("ruby", ruby.Language(), query)

// Implements: REQ-RUBY-001
type Plugin struct{}

func (Plugin) Name() string { return "ruby" }
func (Plugin) Version() int { return 1 }

// Implements: REQ-RUBY-001
func (Plugin) Claims(f *scan.File) bool {
	return !f.Binary && (exts[strings.ToLower(path.Ext(f.Path))] || names[path.Base(f.Path)])
}

func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecoGems, Name: "RubyGems"},
		{ID: ecoStd, Name: "Ruby standard library", Std: true},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all), nil
}

// Import kinds, carried in RawImport.Name.
const (
	kindRequire  = "require"  // require, require_dependency, autoload: the load path, then gems
	kindRelative = "relative" // a path relative to the requiring file ("__DIR__/...")
	kindLoad     = "load"     // load: a file name, extension included
	kindGem      = "gem"      // a gem named in a Gemfile, a gemspec or by Kernel#gem
	kindGemPath  = "gempath"  // a gem the Gemfile takes from a directory (path:)
	kindGemspec  = "gemspec"  // the Gemfile's gemspec directive: the gemspec in a directory
	kindConst    = "const"    // a constant, looked up the way Zeitwerk would; Name is "const:<nesting>"
)

// ref is a constant used in code, kept with the modules around it.
type ref struct {
	text, nesting string
	line          int
}

// Implements: REQ-RUBY-002, REQ-RUBY-003, REQ-RUBY-010
func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	ex := &lang.Extraction{}
	var symbols lang.SymbolSet
	var refs []ref
	err := grammar.Matches(src, func(m treesitter.Match) {
		if fn, ok := m.Get("fn"); ok {
			args, _ := m.Get("args")
			recv, hasRecv := m.Get("recv")
			if imp, ok := callImport(fn, recv, hasRecv, args); ok {
				imp.Line = m[0].Line
				imp.Spec = callSpec(fn, recv, hasRecv, args)
				ex.Imports = append(ex.Imports, imp)
			}
			return
		}
		if fn, ok := m.Get("attr"); ok {
			if strings.HasPrefix(fn, "attr_") || fn == "attr" {
				c := m[len(m)-1]
				if owner, inBody := owner(c.Scopes(), false); !inBody && owner != "" {
					symbols.Add(owner+"."+strings.TrimPrefix(c.Text, ":"), "attr", c.Line)
				}
			}
			return
		}
		for _, c := range m {
			switch c.Name {
			case "bare":
				if c.Text == "gemspec" {
					ex.Imports = append(ex.Imports, lang.RawImport{Spec: "gemspec", Module: "__DIR__", Name: kindGemspec, Line: c.Line})
				}
			case "ref":
				refs = append(refs, ref{c.Text, nesting(c.Scopes()), c.Line})
			case "def.module", "def.class":
				outer, inBody := owner(c.Scopes(), true)
				if !inBody {
					symbols.Add(qualify(outer, c.Text), strings.TrimPrefix(c.Name, "def."), c.Line)
				}
			case "def.method":
				switch outer, inBody := owner(c.Scopes(), true); {
				case inBody:
				case outer != "":
					symbols.Add(outer+"."+c.Text, "method", c.Line)
				default:
					symbols.Add(c.Text, "func", c.Line)
				}
			case "def.smethod":
				obj, _ := m.Get("obj")
				outer, inBody := owner(c.Scopes(), true)
				switch {
				case inBody:
				case obj == "self" && outer != "":
					symbols.Add(outer+"."+c.Text, "method", c.Line)
				case obj != "self" && isConstant(obj):
					symbols.Add(obj+"."+c.Text, "method", c.Line)
				}
			case "def.const":
				if outer, inBody := owner(c.Scopes(), false); !inBody {
					symbols.Add(qualify(outer, c.Text), "const", c.Line)
				}
			}
		}
	})
	seen := map[string]bool{}
	for _, r := range refs {
		key := r.nesting + " " + r.text
		if seen[key] {
			continue
		}
		seen[key] = true
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: r.text, Module: r.text, Name: kindConst + ":" + r.nesting, Line: r.line})
	}
	slices.SortStableFunc(ex.Imports, func(a, b lang.RawImport) int {
		return cmp.Or(cmp.Compare(a.Line, b.Line), cmp.Compare(a.Spec, b.Spec))
	})
	ex.Symbols = symbols.List()
	return ex, err
}

// callSpec is how a call is shown: as written, its whitespace collapsed.
func callSpec(fn, recv string, hasRecv bool, args string) string {
	s := fn + " " + strings.Join(strings.Fields(args), " ")
	if strings.HasPrefix(args, "(") {
		s = fn + strings.Join(strings.Fields(args), " ")
	}
	if hasRecv {
		s = recv + "." + s
	}
	return s
}

// callImport reads what a call imports: require and its kin, a Gemfile's gem and
// gemspec lines, a gemspec's dependencies. Anything whose path or name is not
// spelled out (a variable, a computed string) imports nothing that can be known.
//
// Implements: REQ-RUBY-002, REQ-RUBY-007
func callImport(fn, recv string, hasRecv bool, args string) (lang.RawImport, bool) {
	list := splitArgs(args)
	kernel := !hasRecv || recv == "Kernel"
	switch {
	case kernel && (fn == "require" || fn == "require_dependency") && len(list) == 1:
		if p, ok := evalPath(list[0]); ok {
			if strings.HasPrefix(p, "__DIR__") {
				return lang.RawImport{Module: p, Name: kindRelative}, true
			}
			return lang.RawImport{Module: p, Name: kindRequire}, true
		}
	case kernel && fn == "require_relative" && len(list) == 1:
		if p, ok := evalPath(list[0]); ok {
			if !strings.HasPrefix(p, "__DIR__") {
				p = "__DIR__/" + p
			}
			return lang.RawImport{Module: p, Name: kindRelative}, true
		}
	case kernel && fn == "load" && len(list) >= 1:
		if p, ok := evalPath(list[0]); ok {
			return lang.RawImport{Module: p, Name: kindLoad}, true
		}
	case fn == "autoload" && len(list) == 2:
		if p, ok := evalPath(list[1]); ok {
			if strings.HasPrefix(p, "__DIR__") {
				return lang.RawImport{Module: p, Name: kindRelative}, true
			}
			return lang.RawImport{Module: p, Name: kindRequire}, true
		}
	case !hasRecv && fn == "gem" && len(list) >= 1:
		name, ok := literal(list[0])
		if !ok || name == "" {
			return lang.RawImport{}, false
		}
		if p, ok := option(list[1:], "path"); ok {
			return lang.RawImport{Module: "__DIR__/" + p, Name: kindGemPath}, true
		}
		return lang.RawImport{Module: name, Name: kindGem}, true
	case !hasRecv && fn == "gemspec":
		p, _ := option(list, "path")
		return lang.RawImport{Module: strings.TrimSuffix("__DIR__/"+p, "/"), Name: kindGemspec}, true
	case hasRecv && dependencyCall[fn] && len(list) >= 1:
		if name, ok := literal(list[0]); ok && name != "" {
			return lang.RawImport{Module: name, Name: kindGem}, true
		}
	}
	return lang.RawImport{}, false
}

// dependencyCall are the Gem::Specification methods declaring a dependency.
var dependencyCall = map[string]bool{
	"add_dependency": true, "add_runtime_dependency": true, "add_development_dependency": true,
}

// owner reads the class or module a definition belongs to from its ancestors: the
// names of the classes and modules around it joined with "::", and whether it sits in
// a method body, where it is nobody's to name. A definition's own node is its first
// ancestor and is skipped when self is set. Blocks and `class << self` are
// transparent: a Concern's `class_methods do ... end` still defines on the module.
func owner(scopes []treesitter.Scope, self bool) (string, bool) {
	var outer []string
	for i, s := range scopes {
		if i == 0 && self {
			continue
		}
		switch s.Type {
		case "class", "module":
			outer = append(outer, s.Name)
		case "method", "singleton_method", "lambda":
			if len(outer) == 0 {
				return "", true
			}
		}
	}
	slices.Reverse(outer)
	name := ""
	for _, o := range outer {
		name = qualify(name, o)
	}
	return name, false
}

// nesting is the modules and classes around a constant reference, outermost first,
// joined with "::": Ruby looks a constant up in each of them, innermost first.
func nesting(scopes []treesitter.Scope) string {
	name := ""
	for i := len(scopes) - 1; i >= 0; i-- {
		if s := scopes[i]; s.Type == "class" || s.Type == "module" {
			name = qualify(name, s.Name)
		}
	}
	return name
}

// qualify names a class or module inside another: a name starting with "::" is
// already absolute.
func qualify(outer, name string) string {
	if strings.HasPrefix(name, "::") {
		return strings.TrimPrefix(name, "::")
	}
	if outer == "" {
		return name
	}
	return outer + "::" + name
}

func isConstant(s string) bool {
	return s != "" && s[0] >= 'A' && s[0] <= 'Z' && !strings.ContainsAny(s, " .()")
}

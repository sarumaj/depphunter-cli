// Package golang analyzes Go sources with the standard library parser and resolves
// imports against every go.mod found in the project.
package golang

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

type Plugin struct{}

func (Plugin) Name() string { return "go" }

func (Plugin) Claims(f *scan.File) bool { return strings.HasSuffix(f.Path, ".go") && !f.Binary }

// Implements: REQ-LANG-005, REQ-GO-004
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: lang.EcosystemGo, Name: "Go modules"},
		{ID: lang.EcosystemGoStd, Name: "Go standard library", Std: true},
	}
}

type module struct {
	directory string // relative, "." for the root
	path      string
	requires  map[string]string      // module path -> version
	replaces  map[string]replacement // module path -> what the build uses instead
}

// replacement is the right-hand side of a replace directive: a directory (relative to
// the project root, and possibly outside it) or another module at a version.
type replacement struct {
	directory string
	module    string
	version   string
}

func (Plugin) Version() int { return 1 }

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	modules, err := loadModules(all)
	if err != nil {
		return nil, err
	}
	r := &resolver{modules: modules, packageDirectories: map[string]bool{}}
	for _, f := range lang.Claimed(Plugin{}, all) {
		r.packageDirectories[path.Dir(f.Path)] = true
	}
	return r, nil
}

type resolver struct {
	modules            []*module
	packageDirectories map[string]bool // directories holding Go files, i.e. local packages
}

func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	return resolve(rawImport.Module, owner(r.modules, file), r.modules, r.packageDirectories)
}

// Implements: REQ-GO-003
func loadModules(all []*scan.File) ([]*module, error) {
	var modules []*module
	for _, f := range all {
		if path.Base(f.Path) != "go.mod" {
			continue
		}
		data, err := os.ReadFile(f.AbsolutePath)
		if err != nil {
			// Gone since the scan - a branch switch, an editor saving by rename -
			// and no more reason to abort the analysis than a broken one below.
			continue
		}
		// Parse, not ParseLax: the lax parser, meant for the go.mod of a dependency,
		// drops replace directives, which are exactly what says where a module comes
		// from. It stays the fallback for a go.mod the strict parser refuses.
		parsed, err := modfile.Parse(f.Path, data, nil)
		if err != nil {
			parsed, err = modfile.ParseLax(f.Path, data, nil)
		}
		if err != nil || parsed.Module == nil {
			continue // a broken go.mod should not abort the whole analysis
		}
		m := &module{directory: path.Dir(f.Path), path: parsed.Module.Mod.Path, requires: map[string]string{}, replaces: map[string]replacement{}}
		for _, r := range parsed.Require {
			m.requires[r.Mod.Path] = r.Mod.Version
		}
		for _, r := range parsed.Replace {
			// "old v1.2.3 => new" replaces that one version, and only applies when it
			// is the one required.
			if r.Old.Version != "" && m.requires[r.Old.Path] != r.Old.Version {
				continue
			}
			if modfile.IsDirectoryPath(r.New.Path) {
				m.replaces[r.Old.Path] = replacement{directory: path.Clean(path.Join(m.directory, r.New.Path))}
			} else {
				m.replaces[r.Old.Path] = replacement{module: r.New.Path, version: r.New.Version}
			}
		}
		modules = append(modules, m)
	}
	// Deepest first, so owner() and local resolution pick the most specific module.
	sort.Slice(modules, func(i, j int) bool { return len(modules[i].directory) > len(modules[j].directory) })
	return modules, nil
}

func owner(modules []*module, file string) *module {
	for _, m := range modules {
		if m.directory == "." || strings.HasPrefix(file, m.directory+"/") {
			return m
		}
	}
	return nil
}

// Implements: REQ-LANG-009, REQ-GO-001
func (Plugin) Extract(f *scan.File, source []byte) (*lang.Extraction, error) {
	fSet := token.NewFileSet()
	// On syntax errors the parser still returns a partial AST, which is good enough for a map.
	af, _ := parser.ParseFile(fSet, f.Path, source, parser.SkipObjectResolution)
	if af == nil {
		return &lang.Extraction{}, nil
	}
	extraction := &lang.Extraction{Symbols: symbols(fSet, af)}
	for _, spec := range af.Imports {
		importPath := strings.Trim(spec.Path.Value, "`\"")
		if importPath == "C" {
			continue // cgo pseudo-package
		}
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: importPath, Module: importPath, Line: fSet.Position(spec.Pos()).Line})
	}
	return extraction, nil
}

// Implements: REQ-GO-003, REQ-GO-004, REQ-GO-005, REQ-GO-006
func resolve(importPath string, own *module, modules []*module, packageDirectories map[string]bool) lang.Target {
	if own != nil {
		// The longest match, as the go command picks it: ranging over the map and
		// taking the first would answer differently from run to run.
		old := ""
		for modulePath := range own.replaces {
			if _, ok := within(importPath, modulePath); ok && len(modulePath) > len(old) {
				old = modulePath
			}
		}
		if old != "" {
			rest, _ := within(importPath, old)
			switch r := own.replaces[old]; {
			case r.module != "":
				// The build fetches the replacement, so that is the package - and the
				// version a vulnerability database has to be asked about.
				return lang.Target{Ecosystem: lang.EcosystemGo, Package: r.module, Version: r.version,
					Requested: own.requires[old], Pinned: lang.Pinned(r.version)}
			case packageDirectories[path.Join(r.directory, rest)]:
				return lang.Target{Local: path.Join(r.directory, rest)}
			default:
				// A directory outside the project has no published version. The
				// require line's version, often the v0.0.0-00010101000000-000000000000
				// placeholder, is not what is built and must not be looked up.
				return lang.Target{Ecosystem: lang.EcosystemGo, Package: old}
			}
		}
	}
	for _, m := range modules {
		if rest, ok := within(importPath, m.path); ok {
			if d := path.Join(m.directory, rest); packageDirectories[d] {
				return lang.Target{Local: d}
			}
		}
	}
	if first, _, _ := strings.Cut(importPath, "/"); !strings.Contains(first, ".") {
		return lang.Target{Ecosystem: lang.EcosystemGoStd, Package: importPath}
	}
	if own != nil {
		best := ""
		for modulePath := range own.requires {
			if _, ok := within(importPath, modulePath); ok && len(modulePath) > len(best) {
				best = modulePath
			}
		}
		if best != "" {
			// A require line carries the version the build selects, so a module is
			// pinned unless go.mod was written without one.
			v := own.requires[best]
			return lang.Target{Ecosystem: lang.EcosystemGo, Package: best, Version: v, Pinned: lang.Pinned(v)}
		}
	}
	return lang.Target{Ecosystem: lang.EcosystemGo, Package: importPath, Unresolved: true}
}

// within reports whether import path importPath is prefix or a sub-package of module, returning the remainder.
func within(importPath, module string) (string, bool) {
	if importPath == module {
		return "", true
	}
	if strings.HasPrefix(importPath, module+"/") {
		return importPath[len(module)+1:], true
	}
	return "", false
}

// Implements: REQ-GO-002, REQ-LANG-024
func symbols(fSet *token.FileSet, af *ast.File) []lang.Symbol {
	var set lang.SymbolSet
	add := func(name, kind string, position token.Pos) { set.Add(name, kind, fSet.Position(position).Line) }
	for _, declaration := range af.Decls {
		switch d := declaration.(type) {
		case *ast.FuncDecl:
			if d.Recv != nil && len(d.Recv.List) > 0 {
				add(receiverName(d.Recv.List[0].Type)+"."+d.Name.Name, "method", d.Pos())
			} else {
				add(d.Name.Name, "func", d.Pos())
			}
		case *ast.GenDecl:
			for _, s := range d.Specs {
				switch s := s.(type) {
				case *ast.TypeSpec:
					add(s.Name.Name, "type", s.Pos())
				case *ast.ValueSpec:
					kind := "var"
					if d.Tok == token.CONST {
						kind = "const"
					}
					for _, n := range s.Names {
						add(n.Name, kind, n.Pos())
					}
				}
			}
		}
	}
	return set.List()
}

func receiverName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return receiverName(t.X)
	case *ast.IndexExpr: // generic receiver T[P]
		return receiverName(t.X)
	case *ast.IndexListExpr:
		return receiverName(t.X)
	case *ast.Ident:
		return t.Name
	}
	return "?"
}

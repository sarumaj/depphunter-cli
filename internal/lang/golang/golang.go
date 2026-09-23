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

const (
	ecoModules = "go"
	ecoStd     = "go-std"
)

type Plugin struct{}

func (Plugin) Name() string { return "go" }

func (Plugin) Claims(f *scan.File) bool { return strings.HasSuffix(f.Path, ".go") && !f.Binary }

func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecoModules, Name: "Go modules"},
		{ID: ecoStd, Name: "Go standard library", Std: true},
	}
}

type module struct {
	dir      string // relative, "." for the root
	path     string
	requires map[string]string      // module path -> version
	replaces map[string]replacement // module path -> what the build uses instead
}

// replacement is the right-hand side of a replace directive: a directory (relative to
// the project root, and possibly outside it) or another module at a version.
type replacement struct {
	dir     string
	module  string
	version string
}

func (Plugin) Version() int { return 1 }

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	mods, err := loadModules(all)
	if err != nil {
		return nil, err
	}
	r := &resolver{mods: mods, pkgDirs: map[string]bool{}}
	for _, f := range lang.Claimed(Plugin{}, all) {
		r.pkgDirs[path.Dir(f.Path)] = true
	}
	return r, nil
}

type resolver struct {
	mods    []*module
	pkgDirs map[string]bool // directories holding Go files, i.e. local packages
}

func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	return resolve(imp.Module, owner(r.mods, file), r.mods, r.pkgDirs)
}

func loadModules(all []*scan.File) ([]*module, error) {
	var mods []*module
	for _, f := range all {
		if path.Base(f.Path) != "go.mod" {
			continue
		}
		data, err := os.ReadFile(f.Abs)
		if err != nil {
			// Gone since the scan - a branch switch, an editor saving by rename -
			// and no more reason to abort the analysis than a broken one below.
			continue
		}
		// Parse, not ParseLax: the lax parser, meant for the go.mod of a dependency,
		// drops replace directives, which are exactly what says where a module comes
		// from. It stays the fallback for a go.mod the strict parser refuses.
		mf, err := modfile.Parse(f.Path, data, nil)
		if err != nil {
			mf, err = modfile.ParseLax(f.Path, data, nil)
		}
		if err != nil || mf.Module == nil {
			continue // a broken go.mod should not abort the whole analysis
		}
		m := &module{dir: path.Dir(f.Path), path: mf.Module.Mod.Path, requires: map[string]string{}, replaces: map[string]replacement{}}
		for _, r := range mf.Require {
			m.requires[r.Mod.Path] = r.Mod.Version
		}
		for _, r := range mf.Replace {
			// "old v1.2.3 => new" replaces that one version, and only applies when it
			// is the one required.
			if r.Old.Version != "" && m.requires[r.Old.Path] != r.Old.Version {
				continue
			}
			if modfile.IsDirectoryPath(r.New.Path) {
				m.replaces[r.Old.Path] = replacement{dir: path.Clean(path.Join(m.dir, r.New.Path))}
			} else {
				m.replaces[r.Old.Path] = replacement{module: r.New.Path, version: r.New.Version}
			}
		}
		mods = append(mods, m)
	}
	// Deepest first, so owner() and local resolution pick the most specific module.
	sort.Slice(mods, func(i, j int) bool { return len(mods[i].dir) > len(mods[j].dir) })
	return mods, nil
}

func owner(mods []*module, file string) *module {
	for _, m := range mods {
		if m.dir == "." || strings.HasPrefix(file, m.dir+"/") {
			return m
		}
	}
	return nil
}

func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	fSet := token.NewFileSet()
	// On syntax errors the parser still returns a partial AST, which is good enough for a map.
	af, _ := parser.ParseFile(fSet, f.Path, src, parser.SkipObjectResolution)
	if af == nil {
		return &lang.Extraction{}, nil
	}
	ex := &lang.Extraction{Symbols: symbols(fSet, af)}
	for _, spec := range af.Imports {
		ip := strings.Trim(spec.Path.Value, "`\"")
		if ip == "C" {
			continue // cgo pseudo-package
		}
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: ip, Module: ip, Line: fSet.Position(spec.Pos()).Line})
	}
	return ex, nil
}

func resolve(ip string, own *module, mods []*module, pkgDirs map[string]bool) lang.Target {
	if own != nil {
		// The longest match, as the go command picks it: ranging over the map and
		// taking the first would answer differently from run to run.
		old := ""
		for mp := range own.replaces {
			if _, ok := within(ip, mp); ok && len(mp) > len(old) {
				old = mp
			}
		}
		if old != "" {
			rest, _ := within(ip, old)
			switch r := own.replaces[old]; {
			case r.module != "":
				// The build fetches the replacement, so that is the package - and the
				// version a vulnerability database has to be asked about.
				return lang.Target{Ecosystem: ecoModules, Package: r.module, Version: r.version,
					Requested: own.requires[old], Pinned: lang.Pinned(r.version)}
			case pkgDirs[path.Join(r.dir, rest)]:
				return lang.Target{Local: path.Join(r.dir, rest)}
			default:
				// A directory outside the project: source on this machine, with no
				// published version. The require line's version - often the
				// v0.0.0-00010101000000-000000000000 placeholder - is not what is
				// built and must not be looked up as though it were.
				return lang.Target{Ecosystem: ecoModules, Package: old}
			}
		}
	}
	for _, m := range mods {
		if rest, ok := within(ip, m.path); ok {
			if d := path.Join(m.dir, rest); pkgDirs[d] {
				return lang.Target{Local: d}
			}
		}
	}
	if first, _, _ := strings.Cut(ip, "/"); !strings.Contains(first, ".") {
		return lang.Target{Ecosystem: ecoStd, Package: ip}
	}
	if own != nil {
		best := ""
		for mp := range own.requires {
			if _, ok := within(ip, mp); ok && len(mp) > len(best) {
				best = mp
			}
		}
		if best != "" {
			// A require line carries the version the build selects, so a module is
			// pinned unless go.mod was written without one.
			v := own.requires[best]
			return lang.Target{Ecosystem: ecoModules, Package: best, Version: v, Pinned: lang.Pinned(v)}
		}
	}
	return lang.Target{Ecosystem: ecoModules, Package: ip, Unresolved: true}
}

// within reports whether import path ip is prefix or a sub-package of mod, returning the remainder.
func within(ip, mod string) (string, bool) {
	if ip == mod {
		return "", true
	}
	if strings.HasPrefix(ip, mod+"/") {
		return ip[len(mod)+1:], true
	}
	return "", false
}

func symbols(fSet *token.FileSet, af *ast.File) []lang.Symbol {
	var set lang.SymbolSet
	add := func(name, kind string, pos token.Pos) { set.Add(name, kind, fSet.Position(pos).Line) }
	for _, decl := range af.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv != nil && len(d.Recv.List) > 0 {
				add(recvName(d.Recv.List[0].Type)+"."+d.Name.Name, "method", d.Pos())
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

func recvName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return recvName(t.X)
	case *ast.IndexExpr: // generic receiver T[P]
		return recvName(t.X)
	case *ast.IndexListExpr:
		return recvName(t.X)
	case *ast.Ident:
		return t.Name
	}
	return "?"
}

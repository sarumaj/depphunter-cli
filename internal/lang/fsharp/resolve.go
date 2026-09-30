package fsharp

import (
	"os"
	"path"
	"runtime"
	"strings"
	"sync"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/nuget"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// place is a source file's position in a project's compile order.
type place struct {
	project string // the .fsproj
	index   int
}

// declared is a file declaring a name, and what kind of name.
type declared struct {
	file string
	kind byte
}

type resolver struct {
	*nuget.Store
	files    map[string]bool
	index    map[string][]declared      // full name -> declaring files, in file order
	prefixes map[string]bool            // every proper dotted prefix of a declared name
	places   map[string][]place         // source -> its places in compile orders
	reach    map[string]map[string]bool // project -> projects it references, transitively
}

// maxHops bounds the project reference walk.
const maxHops = 64

// newResolver reads the NuGet store, every F# source's declarations (the lexer is
// cheap) and every .fsproj's compile order and project references. Resolve runs
// concurrently, so all of it is built here and only read later.
//
// Implements: REQ-FSHARP-004, REQ-FSHARP-005
func newResolver(all []*scan.File) *resolver {
	r := &resolver{
		Store: nuget.Read(all), files: lang.PathSet(all), index: map[string][]declared{},
		prefixes: map[string]bool{}, places: map[string][]place{}, reach: map[string]map[string]bool{},
	}
	references := map[string][]string{}
	var sources []*scan.File
	for _, f := range all {
		if f.Binary || f.TooLarge || f.Size > lang.MaxParseSize {
			continue
		}
		p := (Plugin{})
		if !p.Claims(f) {
			continue
		}
		switch class(f.Path) {
		case classProject:
			data, err := os.ReadFile(f.AbsolutePath)
			if err != nil {
				continue
			}
			directory := path.Dir(f.Path)
			n := 0
			for _, it := range nuget.ReadProject(data).Items {
				if it.Update {
					continue
				}
				for _, include := range strings.Split(it.Include, ";") {
					relative, ok := nuget.ProjectPath(include)
					if !ok {
						continue
					}
					target := path.Join(directory, relative)
					switch it.Kind {
					case "Compile":
						r.places[target] = append(r.places[target], place{project: f.Path, index: n})
						n++
					case "ProjectReference":
						references[f.Path] = append(references[f.Path], target)
					}
				}
			}
		case "":
			sources = append(sources, f)
		}
	}
	// Declarations are read concurrently (it is a whole lexing pass) and indexed in
	// file order.
	declarations := make([][]declaration, len(sources))
	var wg sync.WaitGroup
	work := make(chan int)
	for w := 0; w < runtime.GOMAXPROCS(0); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				declarations[i] = fileDeclarations(sources[i])
			}
		}()
	}
	for i := range sources {
		work <- i
	}
	close(work)
	wg.Wait()
	for i, f := range sources {
		for _, d := range declarations[i] {
			r.index[d.name] = append(r.index[d.name], declared{file: f.Path, kind: d.kind})
			for n := d.name; ; {
				i := strings.LastIndexByte(n, '.')
				if i < 0 {
					break
				}
				n = n[:i]
				r.prefixes[n] = true
			}
		}
	}
	for p := range references {
		seen := map[string]bool{}
		queue := []string{p}
		for hop := 0; len(queue) > 0 && hop < maxHops; hop++ {
			var next []string
			for _, q := range queue {
				for _, t := range references[q] {
					if !seen[t] {
						seen[t] = true
						next = append(next, t)
					}
				}
			}
			queue = next
		}
		r.reach[p] = seen
	}
	return r
}

// fileDeclarations is what a source declares; a file without a namespace or module
// header (a script, a program's last file) is a module named after the file.
func fileDeclarations(f *scan.File) []declaration {
	data, err := os.ReadFile(f.AbsolutePath)
	if err != nil {
		return nil
	}
	in := scanSource(string(data))
	declarations := in.declarations
	if !in.header {
		stem := strings.TrimSuffix(path.Base(f.Path), path.Ext(f.Path))
		if stem != "" {
			declarations = append(declarations, declaration{name: strings.ToUpper(stem[:1]) + stem[1:], kind: 'm'})
		}
	}
	return declarations
}

// allowed reports whether importer may use what candidate declares: an earlier file of a
// project both are compiled in, or a file of a project the importer's project
// references. A file compiled in no project (a script) may use anything.
//
// Implements: REQ-FSHARP-004
func (r *resolver) allowed(importer, candidate string) bool {
	importerPlaces := r.places[importer]
	if len(importerPlaces) == 0 {
		return true
	}
	candidatePlaces := r.places[candidate]
	common := false
	for _, a := range importerPlaces {
		for _, b := range candidatePlaces {
			if a.project == b.project {
				common = true
				if b.index < a.index {
					return true
				}
			}
		}
	}
	if common {
		return false
	}
	for _, a := range importerPlaces {
		for _, b := range candidatePlaces {
			if r.reach[a.project][b.project] {
				return true
			}
		}
	}
	return false
}

// local finds the project files declaring name, tried as written and qualified by
// each context, first match wins. blocked says a declaration exists that the
// compile order or the project references keep from the importer.
func (r *resolver) local(file, name string, contexts []string, kinds string) (files []string, blocked bool) {
	candidates := append([]string{name}, nil...)
	for _, c := range contexts {
		if !strings.HasPrefix(c, "nuget:") {
			candidates = append(candidates, c+"."+name)
		}
	}
	for _, n := range candidates {
		seen := map[string]bool{}
		for _, d := range r.index[n] {
			if !strings.ContainsRune(kinds, rune(d.kind)) || seen[d.file] {
				continue
			}
			seen[d.file] = true
			if d.file == file {
				continue
			}
			if !r.allowed(file, d.file) {
				blocked = true
				continue
			}
			files = append(files, d.file)
		}
		if len(files) > 0 {
			return files, blocked
		}
	}
	return nil, blocked
}

func split(name string) (kind string, lines []string) {
	parts := strings.Split(name, "\n")
	return parts[0], parts[1:]
}

// Expand gives an open or a qualified name one import per file when several files
// declare the name (a namespace spread over files).
//
// Implements: REQ-FSHARP-004
func (r *resolver) Expand(file string, rawImport lang.RawImport) ([]lang.Import, bool) {
	kind, ctx := split(rawImport.Name)
	var files []string
	switch kind {
	case kindOpen:
		files, _ = r.local(file, rawImport.Module, ctx, openKinds(rawImport.Spec))
	case kindReference:
		files = r.referenceFiles(file, rawImport.Module, ctx)
	default:
		return nil, false
	}
	if len(files) < 2 {
		return nil, false
	}
	out := make([]lang.Import, 0, len(files))
	for _, f := range files {
		out = append(out, lang.Import{Spec: rawImport.Spec + " (" + f + ")", Line: rawImport.Line, Target: lang.Target{Local: f}})
	}
	return out, true
}

func openKinds(spec string) string {
	if strings.HasPrefix(spec, "open type ") {
		return "t"
	}
	return "nm"
}

// referenceFiles looks a qualified name up longest prefix first: Shop.Cart.Item may be a
// type Item in module Shop.Cart, or module Cart in namespace Shop.
func (r *resolver) referenceFiles(file, name string, ctx []string) []string {
	segments := strings.Split(name, ".")
	for n := len(segments); n >= 1; n-- {
		if files, _ := r.local(file, strings.Join(segments[:n], "."), ctx, "mt"); len(files) > 0 {
			return files
		}
	}
	return nil
}

// Implements: REQ-FSHARP-004, REQ-FSHARP-005, REQ-FSHARP-006, REQ-FSHARP-007, REQ-FSHARP-008
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	kind, lines := split(rawImport.Name)
	switch kind {
	case kindOpen:
		return r.open(file, rawImport, lines)
	case kindReference:
		if files := r.referenceFiles(file, rawImport.Module, lines); len(files) > 0 {
			return lang.Target{Local: files[0]}
		}
		return lang.Target{}
	case kindLoad:
		return r.load(file, rawImport.Module, lines)
	case kindNuGet:
		version := ""
		if len(lines) > 0 {
			version = lines[0]
		}
		return r.Version(file, rawImport.Module, version)
	case kindDLL:
		return r.dll(file, rawImport.Module, lines)
	case kindCompile:
		return r.compile(file, rawImport.Module)
	case kindProject:
		if relative, ok := nuget.ProjectPath(rawImport.Module); ok {
			if p := path.Join(path.Dir(file), relative); r.files[p] {
				return lang.Target{Local: p}
			}
		}
		return lang.Target{}
	case kindPackage:
		group := ""
		if len(lines) > 0 {
			group = lines[0]
		}
		if t, ok := r.Package(file, group, rawImport.Module); ok {
			return t
		}
		return lang.Target{Ecosystem: nuget.Ecosystem, Package: rawImport.Module, Unresolved: true}
	case kindImplicit:
		return r.fsharpCore(file)
	case kindRemote:
		if len(lines) < 1 {
			return lang.Target{}
		}
		reference := ""
		if len(lines) > 1 {
			reference = lines[1]
		}
		t, _ := r.Remote(lines[0], rawImport.Module, reference)
		return t
	case kindPaketFile:
		t, _ := r.RemoteFile(rawImport.Module)
		return t
	}
	return lang.Target{}
}

// open resolves `open X.Y`: the project files declaring it, else a package a
// script's #r names, else a declared NuGet package (an id prefixing the namespace,
// or the shortest declared id under it: Fake.Core -> Fake.Core.Target), else
// FSharp.Core's namespaces, else - when the project declares the name but the
// compile order or references keep it away - nothing, else the .NET base library or
// an unresolved package.
//
// Implements: REQ-FSHARP-004, REQ-FSHARP-008
func (r *resolver) open(file string, rawImport lang.RawImport, ctx []string) lang.Target {
	namespace := rawImport.Module
	files, blocked := r.local(file, namespace, ctx, openKinds(rawImport.Spec))
	if len(files) > 0 {
		return lang.Target{Local: files[0]}
	}
	if strings.HasPrefix(rawImport.Spec, "open type ") {
		if i := strings.LastIndexByte(namespace, '.'); i > 0 {
			if f, b := r.local(file, namespace[:i], ctx, "nm"); len(f) > 0 {
				return lang.Target{Local: f[0]}
			} else if b {
				blocked = true
			}
		}
	}
	if t, ok := r.scriptPackage(file, namespace, ctx); ok {
		return t
	}
	for _, id := range knownNamespaces(namespace) {
		if t, ok := r.Package(file, "", id); ok {
			return t
		}
	}
	declared, isDeclared := r.Declared(file, namespace)
	// A namespace (or a module in it) that declared packages' ids extend, when it
	// shares more segments with that id than a declared id prefixing it does:
	// Fake.Core.TargetOperators is Fake.Core.Target's, not FAKE's.
	for n := namespace; !baseLibrary(namespace); n = n[:strings.LastIndexByte(n, '.')] {
		if isDeclared && strings.Count(n, ".") <= strings.Count(declared.Package, ".") {
			break
		}
		if t, ok := r.Under(file, n); ok {
			return t
		}
		if strings.Count(n, ".") < 2 {
			break
		}
	}
	if isDeclared {
		return declared
	}
	if fsharpCoreNamespace(namespace) {
		return r.fsharpCore(file)
	}
	t := nuget.Undeclared(namespace)
	if t.Ecosystem == nuget.Dotnet {
		return t
	}
	if ids := knownNamespaces(namespace); len(ids) > 0 && !blocked {
		return lang.Target{Ecosystem: nuget.Ecosystem, Package: r.ID(ids[0]), Unresolved: true}
	}
	if blocked || r.prefixes[namespace] {
		return lang.Target{}
	}
	for _, c := range ctx {
		if r.prefixes[c+"."+namespace] || len(r.index[c+"."+namespace]) > 0 {
			return lang.Target{}
		}
	}
	return t
}

// scriptPackage is the package of a script's #r "nuget: ..." lines that provides
// namespace by the rules of Declared and Under.
func (r *resolver) scriptPackage(file, namespace string, ctx []string) (lang.Target, bool) {
	type nugetPackage struct{ id, version string }
	var packages []nugetPackage
	for _, c := range ctx {
		if p, ok := strings.CutPrefix(c, "nuget:"); ok {
			id, version, _ := strings.Cut(p, ",")
			packages = append(packages, nugetPackage{id, version})
		}
	}
	if len(packages) == 0 {
		return lang.Target{}, false
	}
	lower := strings.ToLower(namespace)
	best := -1
	for i, p := range packages {
		if l := strings.ToLower(p.id); (lower == l || strings.HasPrefix(lower, l+".")) && (best < 0 || len(l) > len(packages[best].id)) {
			best = i
		}
	}
	for n := lower; best < 0 && !baseLibrary(namespace); n = n[:strings.LastIndexByte(n, '.')] {
		for i, p := range packages {
			if l := strings.ToLower(p.id); strings.HasPrefix(l, n+".") && (best < 0 || len(l) < len(packages[best].id)) {
				best = i
			}
		}
		if strings.Count(n, ".") < 2 {
			break
		}
	}
	if best < 0 {
		return lang.Target{}, false
	}
	return r.Version(file, packages[best].id, packages[best].version), true
}

// baseLibrary reports whether namespace lies in the namespaces of the .NET base library,
// which many packages extend (System.Text.Json, Microsoft.AspNetCore.TestHost): a
// package id below such a namespace does not say it provides the namespace.
func baseLibrary(namespace string) bool {
	first, _, _ := strings.Cut(namespace, ".")
	return first == "System" || first == "Microsoft" || first == "Windows"
}

// knownNamespaces lists the packages providing a namespace their ids do not spell.
func knownNamespaces(namespace string) []string {
	for _, k := range []struct {
		prefix string
		ids    []string
	}{
		{"FSharp.Control.Tasks", []string{"Ply", "TaskBuilder.fs"}},
		{"Swensen.Unquote", []string{"Unquote"}},
		{"Elmish", []string{"Fable.Elmish", "Elmish"}},
		{"Thoth.Json", []string{"Thoth.Json", "Thoth.Json.Net"}},
		{"Fable.Import", []string{"Fable.Core"}},
		{"Browser", []string{"Fable.Browser.Dom"}},
		{"FSharp.Compiler", []string{"FSharp.Compiler.Service"}},
	} {
		if namespace == k.prefix || strings.HasPrefix(namespace, k.prefix+".") {
			return k.ids
		}
	}
	return nil
}

// coreModules are FSharp.Core modules code opens by their short name, since F#
// opens Microsoft.FSharp.Core, .Collections and .Control by default.
var coreModules = map[string]bool{
	"Checked": true, "Operators": true, "Printf": true, "Unchecked": true, "LanguagePrimitives": true,
	"ExtraTopLevelOperators": true, "NativePtr": true, "OptimizedClosures": true, "NonStructuralComparison": true,
	"List": true, "Seq": true, "Array": true, "Array2D": true, "Array3D": true, "Array4D": true, "Map": true,
	"Set": true, "Option": true, "ValueOption": true, "Result": true, "String": true, "Async": true,
	"Event": true, "Observable": true, "Lazy": true, "Patterns": true, "DerivedPatterns": true, "ExprShape": true,
}

// fsharpCoreNamespace reports whether namespace is one FSharp.Core provides:
// Microsoft.FSharp.*, the FSharp.* short forms F# accepts for them, and the
// modules of the namespaces F# opens by default.
func fsharpCoreNamespace(namespace string) bool {
	if namespace == "Microsoft.FSharp" || strings.HasPrefix(namespace, "Microsoft.FSharp.") {
		return true
	}
	if first, _, _ := strings.Cut(namespace, "."); coreModules[first] {
		return true
	}
	for _, p := range []string{"FSharp.Core", "FSharp.Collections", "FSharp.Control", "FSharp.Reflection", "FSharp.Quotations", "FSharp.Linq", "FSharp.NativeInterop", "FSharp.Text"} {
		if namespace == p || strings.HasPrefix(namespace, p+".") {
			return true
		}
	}
	return false
}

// fsharpCore is the FSharp.Core package: at the version the repository declares or
// locks, else floating with the SDK that references it implicitly.
//
// Implements: REQ-FSHARP-008
func (r *resolver) fsharpCore(file string) lang.Target {
	if t, ok := r.Package(file, "", "FSharp.Core"); ok {
		return t
	}
	return lang.Target{Ecosystem: nuget.Ecosystem, Package: "FSharp.Core", Floating: true}
}

// compile resolves a Compile item: a project file, or a file Paket links from
// paket-files/.
func (r *resolver) compile(project, include string) lang.Target {
	relative, ok := nuget.ProjectPath(include)
	if !ok {
		return lang.Target{}
	}
	p := path.Join(path.Dir(project), relative)
	if r.files[p] {
		return lang.Target{Local: p}
	}
	if i := strings.Index(p, "paket-files/"); i >= 0 {
		if t, ok := r.RemoteRepository(p[i+len("paket-files/"):]); ok {
			return t
		}
		if t, ok := r.RemoteFile(p); ok {
			return t
		}
	}
	return lang.Target{}
}

// load resolves #load: relative to the script, then to each #I directory. A script
// Paket installed (packages/<id>/...) is that package.
func (r *resolver) load(file, argument string, idirs []string) lang.Target {
	for _, p := range r.candidates(file, argument, idirs) {
		if r.files[p] {
			return lang.Target{Local: p}
		}
		if t, ok := r.installed(file, p); ok {
			return t
		}
	}
	return lang.Target{}
}

func (r *resolver) candidates(file, argument string, idirs []string) []string {
	a := strings.ReplaceAll(strings.TrimSpace(argument), `\`, "/")
	if a == "" {
		return nil
	}
	directory := path.Dir(file)
	out := []string{path.Join(directory, a)}
	for _, d := range idirs {
		d = strings.ReplaceAll(d, `\`, "/")
		out = append(out, path.Join(directory, d, a))
	}
	return out
}

// installed is the package a path under packages/ belongs to (Paket installs
// packages/<id>/ and packages/<group>/<id>/).
func (r *resolver) installed(file, p string) (lang.Target, bool) {
	segments := strings.Split(p, "/")
	for i, s := range segments {
		if s != "packages" {
			continue
		}
		for _, k := range []int{i + 1, i + 2} {
			if k < len(segments)-1 {
				if t, ok := r.Package(file, "", segments[k]); ok {
					return t, true
				}
			}
		}
	}
	return lang.Target{}, false
}

// dll resolves #r of an assembly: a file of the repository, a package's assembly
// under packages/, or a bare framework assembly name (System.Xml.Linq). Anything
// else - an assembly outside the repository - is dropped.
func (r *resolver) dll(file, argument string, idirs []string) lang.Target {
	for _, p := range r.candidates(file, argument, idirs) {
		if r.files[p] {
			return lang.Target{Local: p}
		}
		if t, ok := r.installed(file, p); ok {
			return t
		}
	}
	name := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(argument), ".dll"), ".DLL")
	if name == "" || strings.ContainsAny(name, `/\:`) {
		return lang.Target{}
	}
	if t, ok := r.Package(file, "", name); ok {
		return t
	}
	if t := nuget.Undeclared(name); t.Ecosystem == nuget.Dotnet {
		return t
	}
	return lang.Target{}
}

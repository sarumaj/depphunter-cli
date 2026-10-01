package lsp

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"golang.org/x/sync/errgroup"

	"github.com/sarumaj/depphunter-cli/internal/graph"
)

// Server describes a language server: the files it answers for and the commands that
// start it, tried in order.
type Server struct {
	Name       string
	Extensions map[string]string // file extension -> LSP languageId
	// Names are the files the server answers for by name rather than extension
	// (CMakeLists.txt), with their languageId.
	Names    map[string]string
	Commands [][]string
	// Open sends every file with didOpen first; some servers only know opened files.
	Open bool
}

// Implements: REQ-LSP-002
var Servers = []Server{
	{Name: "gopls", Extensions: map[string]string{".go": "go"}, Commands: [][]string{{"gopls"}}},
	{Name: "typescript-language-server", Open: true, Extensions: map[string]string{
		".ts": "typescript", ".mts": "typescript", ".cts": "typescript", ".tsx": "typescriptreact",
		".js": "javascript", ".mjs": "javascript", ".cjs": "javascript", ".jsx": "javascriptreact",
	}, Commands: [][]string{{"typescript-language-server", "--stdio"}}},
	{Name: "python", Open: true, Extensions: map[string]string{".py": "python", ".pyi": "python"},
		Commands: [][]string{{"pyright-langserver", "--stdio"}, {"basedpyright-langserver", "--stdio"}, {"pylsp"}}},
	{Name: "rust-analyzer", Extensions: map[string]string{".rs": "rust"}, Commands: [][]string{{"rust-analyzer"}}},
	{Name: "jdtls", Open: true, Extensions: map[string]string{".java": "java"}, Commands: [][]string{{"jdtls"}}},
	{Name: "kotlin-language-server", Open: true, Extensions: map[string]string{".kt": "kotlin", ".kts": "kotlin"},
		Commands: [][]string{{"kotlin-language-server"}}},
	{Name: "metals", Open: true, Extensions: map[string]string{".scala": "scala", ".sc": "scala"}, Commands: [][]string{{"metals"}}},
	{Name: "csharp-ls", Open: true, Extensions: map[string]string{".cs": "csharp"}, Commands: [][]string{{"csharp-ls"}}},
	// FsAutoComplete answers references for F# sources and scripts; it loads the
	// .fsproj projects (their compile order) first, which takes a while.
	{Name: "fsharp", Open: true, Extensions: map[string]string{".fs": "fsharp", ".fsi": "fsharp", ".fsx": "fsharp"},
		Commands: [][]string{{"fsautocomplete", "--adaptive-lsp-server-enabled"}}},
	{Name: "clangd", Open: true, Extensions: map[string]string{
		".c": "c", ".h": "cpp", ".cc": "cpp", ".cpp": "cpp", ".cxx": "cpp", ".c++": "cpp",
		".hpp": "cpp", ".hh": "cpp", ".hxx": "cpp", ".h++": "cpp", ".ipp": "cpp", ".inl": "cpp",
		".m": "objective-c", ".mm": "objective-cpp",
		".cu": "cuda-cpp", ".cuh": "cuda-cpp", ".metal": "metal", ".clh": "opencl",
	}, Commands: [][]string{{"clangd"}}},
	{Name: "php", Open: true, Extensions: map[string]string{".php": "php", ".phtml": "php", ".inc": "php"},
		Commands: [][]string{{"intelephense", "--stdio"}, {"phpactor", "language-server"}}},
	{Name: "ruby", Open: true, Extensions: map[string]string{".rb": "ruby", ".rake": "ruby", ".gemspec": "ruby", ".ru": "ruby"},
		Commands: [][]string{{"ruby-lsp"}, {"solargraph", "stdio"}}},
	{Name: "sourcekit-lsp", Open: true, Extensions: map[string]string{".swift": "swift"}, Commands: [][]string{{"sourcekit-lsp"}}},
	{Name: "dart", Open: true, Extensions: map[string]string{".dart": "dart"}, Commands: [][]string{{"dart", "language-server", "--protocol=lsp"}}},
	{Name: "elixir", Open: true, Extensions: map[string]string{".ex": "elixir", ".exs": "elixir"},
		Commands: [][]string{{"elixir-ls"}, {"language_server.sh"}, {"lexical"}, {"nextls", "--stdio"}}},
	{Name: "erlang", Open: true, Extensions: map[string]string{".erl": "erlang", ".hrl": "erlang"},
		Commands: [][]string{{"elp", "server"}, {"erlang_ls"}}},
	// The languageserver package runs inside R. R on PATH does not mean the package
	// is installed; when it is not, R exits, initialize fails, and the failure is
	// logged and the server passed over.
	{Name: "r", Open: true, Extensions: map[string]string{".r": "r", ".rmd": "rmd"},
		Commands: [][]string{{"R", "--slave", "-e", "languageserver::run()"}}},
	// The wrapper picks the server binary built for the project's GHC version.
	{Name: "haskell", Open: true, Extensions: map[string]string{".hs": "haskell", ".lhs": "lhaskell"},
		Commands: [][]string{{"haskell-language-server-wrapper", "--lsp"}, {"haskell-language-server", "--lsp"}}},
	// terraform-ls serves OpenTofu files as well; tofu-ls is OpenTofu's fork of it.
	{Name: "terraform", Open: true, Extensions: map[string]string{".tf": "terraform", ".tofu": "opentofu", ".tfvars": "terraform-vars"},
		Commands: [][]string{{"terraform-ls", "serve"}, {"tofu-ls", "serve"}}},
	// buf lsp serve is Buf's own server (buf 1.43 and later); bufls is its former
	// standalone release, protols a community server that needs no Buf configuration.
	{Name: "proto", Open: true, Extensions: map[string]string{".proto": "proto"},
		Commands: [][]string{{"buf", "lsp", "serve"}, {"bufls", "serve"}, {"protols"}}},
	// bash-language-server serves sh and Bash (and bats, which it reads as Bash); it
	// has no zsh support, so zsh files are left out.
	{Name: "bash", Open: true, Extensions: map[string]string{".sh": "shellscript", ".bash": "shellscript", ".ksh": "shellscript", ".bats": "shellscript", ".envrc": "shellscript"},
		Commands: [][]string{{"bash-language-server", "start"}}},
	// lua-language-server (LuaLS) reads Lua and LuaJIT; Luau needs luau-lsp, whose
	// server is its lsp subcommand. Teal's teal-language-server is left out: it
	// answers no references.
	{Name: "lua", Open: true, Extensions: map[string]string{".lua": "lua"}, Commands: [][]string{{"lua-language-server"}}},
	{Name: "luau", Open: true, Extensions: map[string]string{".luau": "luau"}, Commands: [][]string{{"luau-lsp", "lsp"}}},
	// Perl Navigator and PLS are standalone servers; Perl::LanguageServer runs inside
	// perl, which is on PATH wherever Perl is, so without the module installed its
	// initialize fails, is logged, and the server is passed over.
	{Name: "perl", Open: true, Extensions: map[string]string{".pl": "perl", ".pm": "perl", ".t": "perl", ".psgi": "perl"},
		Commands: [][]string{{"perlnavigator", "--stdio"}, {"pls"}, {"perl", "-MPerl::LanguageServer", "-e", "Perl::LanguageServer::run"}}},
	// LanguageServer.jl runs inside julia; julia on PATH without the package
	// installed fails initialize, which is logged, and the server is passed over.
	// It indexes the environment's packages first, which takes a while.
	{Name: "julia", Open: true, Extensions: map[string]string{".jl": "julia"},
		Commands: [][]string{{"julia", "--startup-file=no", "--history-file=no", "-e", "using LanguageServer; runserver()"}}},
	// ocaml-lsp-server answers for implementations, interfaces and the ocamllex and
	// Menhir sources merlin reads; it needs the project built once for dune's
	// metadata.
	{Name: "ocaml", Open: true, Extensions: map[string]string{".ml": "ocaml", ".mli": "ocaml.interface", ".mll": "ocaml.ocamllex", ".mly": "ocaml.menhir"},
		Commands: [][]string{{"ocamllsp"}}},
	// clojure-lsp serves Clojure, ClojureScript and babashka alike (it analyzes the
	// classpath tools.deps or Leiningen computes, so it indexes a while first).
	{Name: "clojure", Open: true, Extensions: map[string]string{".clj": "clojure", ".cljs": "clojure", ".cljc": "clojure", ".bb": "clojure"},
		Commands: [][]string{{"clojure-lsp"}}},
	// zls answers references for Zig sources; it reads build.zig for the modules
	// and packages the build wires.
	{Name: "zig", Open: true, Extensions: map[string]string{".zig": "zig"}, Commands: [][]string{{"zls"}}},
	// nil and nixd both answer references for Nix expressions; nixd evaluates the
	// flake or NIX_PATH it is configured for.
	{Name: "nix", Open: true, Extensions: map[string]string{".nix": "nix"}, Commands: [][]string{{"nil"}, {"nixd"}}},
	// The Gleam compiler serves the language server itself (gleam lsp), from the
	// package's gleam.toml.
	{Name: "gleam", Open: true, Extensions: map[string]string{".gleam": "gleam"}, Commands: [][]string{{"gleam", "lsp"}}},
	// elm-language-server answers references for Elm modules from the nearest
	// elm.json.
	{Name: "elm", Open: true, Extensions: map[string]string{".elm": "elm"}, Commands: [][]string{{"elm-language-server", "--stdio"}}},
	// purescript-language-server answers references for PureScript modules from
	// the project's spago build output.
	{Name: "purescript", Open: true, Extensions: map[string]string{".purs": "purescript"}, Commands: [][]string{{"purescript-language-server", "--stdio"}}},
	// crystalline answers references for Crystal from the shard's entry point
	// (shard.yml targets), over stdio.
	{Name: "crystal", Open: true, Extensions: map[string]string{".cr": "crystal"}, Commands: [][]string{{"crystalline"}}},
	// serve-d answers references for D modules from the dub package around them,
	// over stdio.
	{Name: "d", Open: true, Extensions: map[string]string{".d": "d", ".di": "d"}, Commands: [][]string{{"serve-d"}}},
	// The Haxe language server (vshaxe's, run as haxe-language-server) answers
	// references for Haxe modules; it compiles with the first .hxml it finds
	// (build.hxml), so a project with none may get no answers.
	{Name: "haxe", Open: true, Extensions: map[string]string{".hx": "haxe"}, Commands: [][]string{{"haxe-language-server"}}},
	// The Ada Language Server answers references for Ada units and their
	// declarations over stdio; it loads the project file it finds (a single
	// .gpr at the root, or alire.toml's), so a repository with several may get
	// fewer answers.
	{Name: "ada", Open: true, Extensions: map[string]string{".ads": "ada", ".adb": "ada", ".ada": "ada"}, Commands: [][]string{{"ada_language_server"}}},
	// racket-langserver answers references for Racket modules over stdio,
	// started as a module of an installed Racket; it expands each opened
	// file, so a first answer may take a while.
	{Name: "racket", Open: true, Extensions: map[string]string{".rkt": "racket", ".rktl": "racket", ".scrbl": "racket"}, Commands: [][]string{{"racket", "-l", "racket-langserver"}}},
	// Nomic Foundation's server (Hardhat's) reads Foundry and Hardhat projects
	// alike; solidity-ls is Juan Blanco's server of the VS Code extension. Both
	// compile the project before answering, so a first answer may take a while.
	{Name: "solidity", Open: true, Extensions: map[string]string{".sol": "solidity"},
		Commands: [][]string{{"nomicfoundation-solidity-language-server", "--stdio"}, {"solidity-ls", "--stdio"}}},
	// cl-lsp answers references for Common Lisp over stdio; it loads each
	// opened file's system in its own Lisp image, so a first answer may take a
	// while.
	{Name: "commonlisp", Open: true, Extensions: map[string]string{".lisp": "lisp", ".lsp": "lisp", ".cl": "lisp", ".asd": "lisp"}, Commands: [][]string{{"cl-lsp"}}},
	// nimlangserver (the Nim team's) and nimlsp both answer references for Nim
	// modules over stdio; nimlangserver starts nimsuggest per project, so a
	// first answer may take a while.
	{Name: "nim", Open: true, Extensions: map[string]string{".nim": "nim", ".nims": "nim", ".nimble": "nim"},
		Commands: [][]string{{"nimlangserver"}, {"nimlsp"}}},
	// jsonnet-language-server (Grafana's) answers references for Jsonnet and
	// libsonnet files; `cue lsp` is CUE's own server, part of the cue command.
	{Name: "jsonnet", Open: true, Extensions: map[string]string{".jsonnet": "jsonnet", ".libsonnet": "jsonnet"},
		Commands: [][]string{{"jsonnet-language-server"}}},
	{Name: "cue", Open: true, Extensions: map[string]string{".cue": "cue"}, Commands: [][]string{{"cue", "lsp"}}},
	// dhall-lsp-server (from dhall-haskell), Puppet Editor Services'
	// puppet-languageserver and Regal's language server (Styra's Rego linter,
	// on OPA's parser) answer references for their languages.
	{Name: "dhall", Open: true, Extensions: map[string]string{".dhall": "dhall"}, Commands: [][]string{{"dhall-lsp-server"}}},
	{Name: "puppet", Open: true, Extensions: map[string]string{".pp": "puppet"}, Commands: [][]string{{"puppet-languageserver", "--stdio"}}},
	{Name: "rego", Open: true, Extensions: map[string]string{".rego": "rego"}, Commands: [][]string{{"regal", "language-server"}}},
	// fortls answers references for Fortran modules and procedures across the
	// project's sources (free and fixed form), over stdio.
	{Name: "fortran", Open: true, Extensions: map[string]string{".f90": "fortran", ".f95": "fortran", ".f03": "fortran", ".f08": "fortran", ".f18": "fortran", ".f": "fortran", ".for": "fortran", ".ftn": "fortran", ".f77": "fortran", ".fpp": "fortran"}, Commands: [][]string{{"fortls"}}},
	// neocmakelsp and cmake-language-server both answer references for CMake's
	// functions, macros and variables; a CMakeLists.txt is known by its name.
	{Name: "cmake", Open: true, Extensions: map[string]string{".cmake": "cmake"}, Names: map[string]string{"CMakeLists.txt": "cmake"},
		Commands: [][]string{{"neocmakelsp", "--stdio"}, {"cmake-language-server"}}},
	// starpls, bazel-lsp and bzl answer references for Bazel's Starlark: BUILD and
	// WORKSPACE files are known by their names, the rest by .bzl and .bazel.
	{Name: "starlark", Open: true, Extensions: map[string]string{".bzl": "starlark", ".bazel": "starlark"},
		Names:    map[string]string{"BUILD": "starlark", "WORKSPACE": "starlark", "WORKSPACE.bzlmod": "starlark"},
		Commands: [][]string{{"starpls", "server"}, {"bazel-lsp"}, {"bzl", "lsp", "serve"}}},
	{Name: "glsl_analyzer", Open: true, Extensions: map[string]string{
		".glsl": "glsl", ".vert": "glsl", ".frag": "glsl", ".geom": "glsl", ".tesc": "glsl", ".tese": "glsl",
		".comp": "glsl", ".rgen": "glsl", ".rchit": "glsl", ".rahit": "glsl", ".rmiss": "glsl", ".rint": "glsl",
		".rcall": "glsl", ".vsh": "glsl", ".fsh": "glsl",
	}, Commands: [][]string{{"glsl_analyzer"}}},
	{Name: "shader-language-server", Open: true, Extensions: map[string]string{
		".hlsl": "hlsl", ".hlsli": "hlsl", ".fx": "hlsl", ".fxh": "hlsl", ".usf": "hlsl", ".ush": "hlsl",
	}, Commands: [][]string{{"shader-language-server"}}},
	{Name: "wgsl-analyzer", Open: true, Extensions: map[string]string{".wgsl": "wgsl", ".wesl": "wesl"},
		Commands: [][]string{{"wgsl-analyzer"}, {"wgsl_analyzer"}}},
}

// languageID is the languageId the server gives the file at p, and whether it
// answers for it at all.
func (s Server) languageID(p string) (string, bool) {
	if id, ok := s.Names[path.Base(p)]; ok {
		return id, true
	}
	id, ok := s.Extensions[strings.ToLower(path.Ext(p))]
	return id, ok
}

type Options struct {
	Root     string
	Timeout  time.Duration // overall budget; on expiry the references found so far are kept
	Parallel int           // requests in flight per server
	LookPath func(string) (string, error)
	Logf     func(format string, arguments ...any)
}

type Result struct {
	Edges   []*graph.Edge `json:"edges"`   // kind "reference": enclosing symbol (or file) -> definition
	Servers []string      `json:"servers"` // servers that answered
	Queried int           `json:"queried"` // definitions asked about
	Partial bool          `json:"partial"` // the time budget ran out
}

type symbol struct {
	id   string
	name string // as written in source: the method part of "Type.method"
	line int    // 1-based
}

type fileInfo struct {
	id      string
	symbols []symbol // by line
	spans   []span   // full extents of symbols, from textDocument/documentSymbol
}

// span is the line range (1-based, inclusive) a symbol's definition covers.
type span struct {
	start, end int
	id         string
}

// References asks every applicable, installed language server where each symbol of g
// is referenced.
//
// Implements: REQ-LSP-002, REQ-LSP-009, REQ-LSP-010
func References(ctx context.Context, g *graph.Graph, options Options) (*Result, error) {
	if options.Parallel <= 0 {
		options.Parallel = max(2, runtime.NumCPU()/2)
	}
	if options.LookPath == nil {
		options.LookPath = exec.LookPath
	}
	if options.Logf == nil {
		options.Logf = func(string, ...any) {}
	}
	if options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}

	files := map[string]*fileInfo{} // path -> symbols
	for n := range g.Of(graph.KindFile) {
		files[n.Path] = &fileInfo{id: n.ID}
	}
	for n := range g.Of(graph.KindSymbol) {
		if f := files[graph.FileOf(n.Parent)]; f != nil {
			f.symbols = append(f.symbols, symbol{id: n.ID, name: symbolWord(n.Name), line: n.Line})
		}
	}
	for _, f := range files {
		sort.Slice(f.symbols, func(i, j int) bool { return f.symbols[i].line < f.symbols[j].line })
	}

	result := &Result{}
	seen := map[[2]string]bool{}
	var mu sync.Mutex
	add := func(from, to string) {
		mu.Lock()
		defer mu.Unlock()
		if from != to && !seen[[2]string{from, to}] {
			seen[[2]string{from, to}] = true
			result.Edges = append(result.Edges, &graph.Edge{From: from, To: to, Kind: graph.EdgeReference})
		}
	}

	for _, server := range Servers {
		var paths []string
		for p := range files {
			if _, ok := server.languageID(p); ok {
				paths = append(paths, p)
			}
		}
		if len(paths) == 0 {
			continue
		}
		argv := server.command(options.LookPath)
		if argv == nil {
			options.Logf("references: no %s on PATH, skipping %d files", server.Name, len(paths))
			continue
		}
		sort.Strings(paths)
		start := time.Now()
		n, err := runServer(ctx, server, argv, options, paths, files, add)
		result.Queried += n
		if errors.Is(err, context.DeadlineExceeded) {
			result.Partial = true
			options.Logf("references: time budget used up during %s; results are partial", server.Name)
			break
		}
		if err != nil {
			options.Logf("references: %s: %v", server.Name, err)
			continue
		}
		result.Servers = append(result.Servers, server.Name)
		options.Logf("references: %s answered %d definitions in %s", server.Name, n, time.Since(start).Round(time.Millisecond))
	}
	sort.Slice(result.Edges, func(i, j int) bool {
		if result.Edges[i].From != result.Edges[j].From {
			return result.Edges[i].From < result.Edges[j].From
		}
		return result.Edges[i].To < result.Edges[j].To
	})
	return result, ctx.Err()
}

func (s Server) command(lookPath func(string) (string, error)) []string {
	for _, c := range s.Commands {
		if p, err := lookPath(c[0]); err == nil {
			return append([]string{p}, c[1:]...)
		}
		if p := goBin(c[0]); p != "" {
			return append([]string{p}, c[1:]...)
		}
	}
	return nil
}

// goBin finds a tool installed with `go install` (gopls) when its directory is not on
// PATH: $GOBIN, then $GOPATH/bin, then ~/go/bin.
func goBin(name string) string {
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	var directories []string
	if d := os.Getenv("GOBIN"); d != "" {
		directories = append(directories, d)
	}
	for _, d := range filepath.SplitList(os.Getenv("GOPATH")) {
		directories = append(directories, filepath.Join(d, "bin"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		directories = append(directories, filepath.Join(home, "go", "bin"))
	}
	for _, d := range directories {
		if fileInfo, err := os.Stat(filepath.Join(d, name)); err == nil && !fileInfo.IsDir() {
			return filepath.Join(d, name)
		}
	}
	return ""
}

// runServer starts one language server over the project, asks it where the symbols of
// its files are referenced, and hands each reference to add; it returns how many
// definitions it asked about.
//
// Implements: REQ-LSP-003
func runServer(ctx context.Context, server Server, argv []string, options Options, paths []string,
	files map[string]*fileInfo, add func(from, to string)) (int, error) {
	c, err := start(ctx, options.Root, argv)
	if err != nil {
		return 0, err
	}
	defer func() {
		stop, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		c.shutdown(stop)
	}()
	if err := initialize(ctx, c, options.Root); err != nil {
		return 0, err
	}
	queries := prepareQueries(ctx, c, server, options, paths, files)
	sent, failed := queryReferences(ctx, c, options, queries, files, add)
	if ctx.Err() != nil {
		return sent, ctx.Err()
	}
	if sent > 0 && failed != nil {
		options.Logf("references: %s: some requests failed, first: %v", server.Name, failed)
	}
	return sent, nil
}

// initialize opens the session with the capabilities depphunter uses: references,
// document symbols and synchronization, over one workspace folder, root.
func initialize(ctx context.Context, c *client, root string) error {
	rootURI := fileURI(root)
	var reply struct{}
	if err := c.call(ctx, "initialize", map[string]any{
		"processId":        os.Getpid(),
		"rootUri":          rootURI,
		"rootPath":         root,
		"workspaceFolders": []map[string]string{{"uri": rootURI, "name": filepath.Base(root)}},
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"references":      map[string]any{},
				"synchronization": map[string]any{},
				"documentSymbol":  map[string]any{"hierarchicalDocumentSymbolSupport": true},
			},
			"workspace": map[string]any{"configuration": true, "workspaceFolders": true},
		},
	}, &reply); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	return c.notify("initialized", map[string]any{})
}

// A query is a symbol's definition to ask references of: its file and the column
// its name starts at on its line.
type query struct {
	file   string
	symbol symbol
	column int
}

// prepareQueries opens each file with a server that wants them opened, records the
// extents of its symbols (documentSpans), and finds where on its line each symbol's
// name is. A file the server could not be told about is logged, once.
func prepareQueries(ctx context.Context, c *client, server Server, options Options, paths []string, files map[string]*fileInfo) []query {
	root := options.Root
	var queries []query
	reported := false
	for _, p := range paths {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
		if err != nil {
			continue
		}
		uri := fileURI(filepath.Join(root, p))
		if server.Open {
			id, _ := server.languageID(p)
			if err := c.notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{
				"uri": uri, "languageId": id, "version": 1, "text": string(data),
			}}); err != nil && !reported {
				reported = true
				options.Logf("references: %s: could not open %s: %v", server.Name, p, err)
			}
		}
		lines := strings.Split(string(data), "\n")
		files[p].spans = documentSpans(ctx, c, uri, files[p].symbols)
		for _, s := range files[p].symbols {
			if s.line < 1 || s.line > len(lines) {
				continue
			}
			if column, ok := nameColumn(lines[s.line-1], s.name); ok {
				queries = append(queries, query{p, s, column})
			}
		}
	}
	return queries
}

// queryReferences asks for the references of each query, options.Parallel at a time,
// and hands add each reference from the symbol (or file) that encloses it; it returns
// how many it sent and the first that failed while ctx was live. The first request
// also waits for the server to load the workspace.
func queryReferences(ctx context.Context, c *client, options Options, queries []query, files map[string]*fileInfo,
	add func(from, to string)) (int, error) {
	var g errgroup.Group
	g.SetLimit(options.Parallel)
	var failed error
	var once sync.Once
	sent := 0
	for _, q := range queries {
		if ctx.Err() != nil {
			break
		}
		sent++
		g.Go(func() error {
			var locations []struct {
				URI   string `json:"uri"`
				Range struct {
					Start struct{ Line int } `json:"start"`
				} `json:"range"`
			}
			err := c.call(ctx, "textDocument/references", map[string]any{
				"textDocument": map[string]string{"uri": fileURI(filepath.Join(options.Root, q.file))},
				"position":     map[string]int{"line": q.symbol.line - 1, "character": q.column},
				"context":      map[string]bool{"includeDeclaration": false},
			}, &locations)
			if err != nil {
				if ctx.Err() == nil {
					once.Do(func() { failed = err })
				}
				return nil
			}
			for _, l := range locations {
				if relative, ok := relativePath(options.Root, l.URI); ok {
					if f := files[relative]; f != nil {
						add(f.enclosing(l.Range.Start.Line+1), q.symbol.id)
					}
				}
			}
			return nil
		})
	}
	g.Wait()
	return sent, failed
}

// enclosing returns the innermost symbol whose definition contains line, or the file
// for top-level code. Without extents from the server it falls back to the closest
// preceding definition.
//
// Implements: REQ-LSP-004
func (f *fileInfo) enclosing(line int) string {
	if f.spans != nil {
		best := span{id: f.id, start: 0, end: 1 << 30}
		for _, s := range f.spans {
			if s.start <= line && line <= s.end && s.end-s.start < best.end-best.start {
				best = s
			}
		}
		return best.id
	}
	i := sort.Search(len(f.symbols), func(i int) bool { return f.symbols[i].line > line })
	if i == 0 {
		return f.id
	}
	return f.symbols[i-1].id
}

type lspRange struct {
	Start struct{ Line int } `json:"start"`
	End   struct{ Line int } `json:"end"`
}

// documentSymbol covers both reply shapes: DocumentSymbol (range, selectionRange,
// children) and SymbolInformation (location.range).
type documentSymbol struct {
	Name           string                    `json:"name"`
	Range          *lspRange                 `json:"range"`
	SelectionRange *lspRange                 `json:"selectionRange"`
	Location       *struct{ Range lspRange } `json:"location"`
	Children       []documentSymbol          `json:"children"`
}

// documentSpans matches the file's symbols to the server's document symbols by
// definition line and returns their extents; nil when the server gives none.
//
// Implements: REQ-LSP-004
func documentSpans(ctx context.Context, c *client, uri string, symbols []symbol) []span {
	var reply []documentSymbol
	if err := c.call(ctx, "textDocument/documentSymbol", map[string]any{
		"textDocument": map[string]string{"uri": uri},
	}, &reply); err != nil || len(reply) == 0 {
		return nil
	}
	extent := map[int]int{} // definition line (1-based) -> last line
	var walk func([]documentSymbol)
	walk = func(symbols []documentSymbol) {
		for _, d := range symbols {
			full, selection := d.Range, d.SelectionRange
			if full == nil && d.Location != nil {
				full = &d.Location.Range
			}
			if full != nil {
				if selection == nil {
					selection = full
				}
				for _, l := range []int{selection.Start.Line + 1, full.Start.Line + 1} {
					if full.End.Line+1 > extent[l] {
						extent[l] = full.End.Line + 1
					}
				}
			}
			walk(d.Children)
		}
	}
	walk(reply)
	var spans []span
	for _, s := range symbols {
		if end, ok := extent[s.line]; ok {
			spans = append(spans, span{start: s.line, end: end, id: s.id})
		}
	}
	return spans
}

var wordCache sync.Map // name -> *regexp.Regexp

// symbolWord is the word a symbol's name is written as on its line:
// Type.method -> method, init@12 -> init, an Elixir or Erlang function's
// Mod.fun/2 -> fun, and an Objective-C method's Class.a:b: -> a.
//
// Implements: REQ-LSP-003
func symbolWord(name string) string {
	// A Clojure defmethod is the multimethod's name and its dispatch value
	// ("area :circle"): the name is the word.
	if i := strings.IndexByte(name, ' '); i > 0 {
		name = name[:i]
	}
	if i := strings.LastIndex(name, "@"); i > 0 && allDigits(name[i+1:]) {
		name = name[:i]
	}
	// A Bazel target is //pkg:name; its name is the word (name = "shop").
	if strings.HasPrefix(name, "//") {
		return name[strings.LastIndex(name, ":")+1:]
	}
	if i := strings.LastIndex(name, "/"); i > 0 && allDigits(name[i+1:]) {
		name = name[:i]
	}
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	if i := strings.Index(name, "@"); i >= 0 {
		name = name[:i]
	}
	// An Objective-C selector is written in parts; its first is on the line
	// (initWithFrame:style: -> initWithFrame), and a category is its class's name
	// with the category's after it (NSString(Shop) -> NSString).
	if i := strings.IndexAny(name, ":("); i > 0 {
		name = name[:i]
	}
	return name
}

func allDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s != ""
}

// nameColumn finds name as a whole word on the definition line and returns its
// column in UTF-16 code units, the unit LSP positions use by default.
//
// Implements: REQ-LSP-008
func nameColumn(line, name string) (int, bool) {
	re, ok := wordCache.Load(name)
	if !ok {
		re, _ = wordCache.LoadOrStore(name, regexp.MustCompile(`(^|[^\w$])(`+regexp.QuoteMeta(name)+`)($|[^\w$])`))
	}
	m := re.(*regexp.Regexp).FindStringSubmatchIndex(line)
	if m == nil {
		return 0, false
	}
	return len(utf16.Encode([]rune(line[:m[4]]))), true
}

func fileURI(p string) string {
	p = filepath.ToSlash(p)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p // Windows: C:/x -> /C:/x
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}

func relativePath(root, uri string) (string, bool) {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return "", false
	}
	p := u.Path
	if len(p) > 2 && p[0] == '/' && p[2] == ':' {
		p = p[1:] // /C:/x -> C:/x
	}
	relative, err := filepath.Rel(root, filepath.FromSlash(p))
	if err != nil || strings.HasPrefix(relative, "..") {
		return "", false
	}
	return filepath.ToSlash(relative), true
}

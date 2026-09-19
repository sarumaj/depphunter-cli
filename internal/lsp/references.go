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
	Name     string
	Exts     map[string]string // file extension -> LSP languageId
	Commands [][]string
	// Open sends every file with didOpen first; some servers only know opened files.
	Open bool
}

var Servers = []Server{
	{Name: "gopls", Exts: map[string]string{".go": "go"}, Commands: [][]string{{"gopls"}}},
	{Name: "typescript-language-server", Open: true, Exts: map[string]string{
		".ts": "typescript", ".mts": "typescript", ".cts": "typescript", ".tsx": "typescriptreact",
		".js": "javascript", ".mjs": "javascript", ".cjs": "javascript", ".jsx": "javascriptreact",
	}, Commands: [][]string{{"typescript-language-server", "--stdio"}}},
	{Name: "python", Open: true, Exts: map[string]string{".py": "python", ".pyi": "python"},
		Commands: [][]string{{"pyright-langserver", "--stdio"}, {"basedpyright-langserver", "--stdio"}, {"pylsp"}}},
	{Name: "rust-analyzer", Exts: map[string]string{".rs": "rust"}, Commands: [][]string{{"rust-analyzer"}}},
	{Name: "jdtls", Open: true, Exts: map[string]string{".java": "java"}, Commands: [][]string{{"jdtls"}}},
	{Name: "csharp-ls", Open: true, Exts: map[string]string{".cs": "csharp"}, Commands: [][]string{{"csharp-ls"}}},
}

type Options struct {
	Root     string
	Timeout  time.Duration // overall budget; on expiry the references found so far are kept
	Parallel int           // requests in flight per server
	LookPath func(string) (string, error)
	Logf     func(format string, args ...any)
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
func References(ctx context.Context, g *graph.Graph, opts Options) (*Result, error) {
	if opts.Parallel <= 0 {
		opts.Parallel = max(2, runtime.NumCPU()/2)
	}
	if opts.LookPath == nil {
		opts.LookPath = exec.LookPath
	}
	if opts.Logf == nil {
		opts.Logf = func(string, ...any) {}
	}
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	files := map[string]*fileInfo{} // path -> symbols
	for _, n := range g.Nodes {
		if n.Kind == graph.KindFile {
			files[n.Path] = &fileInfo{id: n.ID}
		}
	}
	for _, n := range g.Nodes {
		if n.Kind != graph.KindSymbol {
			continue
		}
		p := strings.TrimPrefix(n.Parent, "f:")
		if f := files[p]; f != nil {
			name := n.Name
			if i := strings.LastIndex(name, "."); i >= 0 {
				name = name[i+1:] // Type.method -> method
			}
			if i := strings.Index(name, "@"); i >= 0 {
				name = name[:i] // init@12 -> init
			}
			f.symbols = append(f.symbols, symbol{id: n.ID, name: name, line: n.Line})
		}
	}
	for _, f := range files {
		sort.Slice(f.symbols, func(i, j int) bool { return f.symbols[i].line < f.symbols[j].line })
	}

	res := &Result{}
	seen := map[[2]string]bool{}
	var mu sync.Mutex
	add := func(from, to string) {
		mu.Lock()
		defer mu.Unlock()
		if from != to && !seen[[2]string{from, to}] {
			seen[[2]string{from, to}] = true
			res.Edges = append(res.Edges, &graph.Edge{From: from, To: to, Kind: graph.EdgeReference})
		}
	}

	for _, srv := range Servers {
		var paths []string
		for p := range files {
			if _, ok := srv.Exts[strings.ToLower(path.Ext(p))]; ok {
				paths = append(paths, p)
			}
		}
		if len(paths) == 0 {
			continue
		}
		argv := srv.command(opts.LookPath)
		if argv == nil {
			opts.Logf("references: no %s on PATH, skipping %d files", srv.Name, len(paths))
			continue
		}
		sort.Strings(paths)
		start := time.Now()
		n, err := runServer(ctx, srv, argv, opts, paths, files, add)
		res.Queried += n
		if errors.Is(err, context.DeadlineExceeded) {
			res.Partial = true
			opts.Logf("references: time budget used up during %s; results are partial", srv.Name)
			break
		}
		if err != nil {
			opts.Logf("references: %s: %v", srv.Name, err)
			continue
		}
		res.Servers = append(res.Servers, srv.Name)
		opts.Logf("references: %s answered %d definitions in %s", srv.Name, n, time.Since(start).Round(time.Millisecond))
	}
	sort.Slice(res.Edges, func(i, j int) bool {
		if res.Edges[i].From != res.Edges[j].From {
			return res.Edges[i].From < res.Edges[j].From
		}
		return res.Edges[i].To < res.Edges[j].To
	})
	return res, ctx.Err()
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
	var dirs []string
	if d := os.Getenv("GOBIN"); d != "" {
		dirs = append(dirs, d)
	}
	for _, d := range filepath.SplitList(os.Getenv("GOPATH")) {
		dirs = append(dirs, filepath.Join(d, "bin"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, "go", "bin"))
	}
	for _, d := range dirs {
		if st, err := os.Stat(filepath.Join(d, name)); err == nil && !st.IsDir() {
			return filepath.Join(d, name)
		}
	}
	return ""
}

func runServer(ctx context.Context, srv Server, argv []string, opts Options, paths []string,
	files map[string]*fileInfo, add func(from, to string)) (int, error) {
	c, err := start(ctx, opts.Root, argv)
	if err != nil {
		return 0, err
	}
	defer func() {
		stop, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		c.shutdown(stop)
	}()

	rootURI := fileURI(opts.Root)
	var init struct{}
	if err := c.call(ctx, "initialize", map[string]any{
		"processId":        os.Getpid(),
		"rootUri":          rootURI,
		"rootPath":         opts.Root,
		"workspaceFolders": []map[string]string{{"uri": rootURI, "name": filepath.Base(opts.Root)}},
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"references":      map[string]any{},
				"synchronization": map[string]any{},
				"documentSymbol":  map[string]any{"hierarchicalDocumentSymbolSupport": true},
			},
			"workspace": map[string]any{"configuration": true, "workspaceFolders": true},
		},
	}, &init); err != nil {
		return 0, fmt.Errorf("initialize: %w", err)
	}
	if err := c.notify("initialized", map[string]any{}); err != nil {
		return 0, err
	}

	type query struct {
		file string
		sym  symbol
		col  int
	}
	var queries []query
	for _, p := range paths {
		data, err := os.ReadFile(filepath.Join(opts.Root, filepath.FromSlash(p)))
		if err != nil {
			continue
		}
		if srv.Open {
			c.notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{
				"uri": fileURI(filepath.Join(opts.Root, p)), "languageId": srv.Exts[strings.ToLower(path.Ext(p))],
				"version": 1, "text": string(data),
			}})
		}
		lines := strings.Split(string(data), "\n")
		files[p].spans = documentSpans(ctx, c, fileURI(filepath.Join(opts.Root, p)), files[p].symbols)
		for _, s := range files[p].symbols {
			if s.line < 1 || s.line > len(lines) {
				continue
			}
			if col, ok := nameColumn(lines[s.line-1], s.name); ok {
				queries = append(queries, query{p, s, col})
			}
		}
	}

	// The first request also waits for the server to load the workspace.
	var g errgroup.Group
	g.SetLimit(opts.Parallel)
	var firstErr error
	var errOnce sync.Once
	sent := 0
	for _, q := range queries {
		if ctx.Err() != nil {
			break
		}
		sent++
		g.Go(func() error {
			var locs []struct {
				URI   string `json:"uri"`
				Range struct {
					Start struct{ Line int } `json:"start"`
				} `json:"range"`
			}
			err := c.call(ctx, "textDocument/references", map[string]any{
				"textDocument": map[string]string{"uri": fileURI(filepath.Join(opts.Root, q.file))},
				"position":     map[string]int{"line": q.sym.line - 1, "character": q.col},
				"context":      map[string]bool{"includeDeclaration": false},
			}, &locs)
			if err != nil {
				if ctx.Err() == nil {
					errOnce.Do(func() { firstErr = err })
				}
				return nil
			}
			for _, l := range locs {
				if rel, ok := relPath(opts.Root, l.URI); ok {
					if f := files[rel]; f != nil {
						add(f.enclosing(l.Range.Start.Line+1), q.sym.id)
					}
				}
			}
			return nil
		})
	}
	g.Wait()
	if ctx.Err() != nil {
		return sent, ctx.Err()
	}
	if sent > 0 && firstErr != nil && len(queries) > 0 {
		opts.Logf("references: %s: some requests failed, first: %v", srv.Name, firstErr)
	}
	return sent, nil
}

// enclosing returns the innermost symbol whose definition contains line, or the file
// for top-level code. Without extents from the server it falls back to the closest
// preceding definition.
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
func documentSpans(ctx context.Context, c *client, uri string, symbols []symbol) []span {
	var reply []documentSymbol
	if err := c.call(ctx, "textDocument/documentSymbol", map[string]any{
		"textDocument": map[string]string{"uri": uri},
	}, &reply); err != nil || len(reply) == 0 {
		return nil
	}
	extent := map[int]int{} // definition line (1-based) -> last line
	var walk func([]documentSymbol)
	walk = func(ds []documentSymbol) {
		for _, d := range ds {
			full, sel := d.Range, d.SelectionRange
			if full == nil && d.Location != nil {
				full = &d.Location.Range
			}
			if full != nil {
				if sel == nil {
					sel = full
				}
				for _, l := range []int{sel.Start.Line + 1, full.Start.Line + 1} {
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

// nameColumn finds name as a whole word on the definition line and returns its
// column in UTF-16 code units, the unit LSP positions use by default.
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

func relPath(root, uri string) (string, bool) {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return "", false
	}
	p := u.Path
	if len(p) > 2 && p[0] == '/' && p[2] == ':' {
		p = p[1:] // /C:/x -> C:/x
	}
	rel, err := filepath.Rel(root, filepath.FromSlash(p))
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

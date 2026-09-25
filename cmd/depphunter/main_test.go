package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/config"
	"github.com/sarumaj/depphunter-cli/internal/graph"
)

// execute runs the command with args and returns what it printed.
func execute(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := newCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(context.Background())
	return out.String(), err
}

// Verifies: REQ-CLI-002, REQ-CLI-003, REQ-CLI-006, REQ-CLI-007
func TestVersionAndHelp(t *testing.T) {
	out, err := execute(t, "--version")
	if err != nil || out != "depphunter "+version+"\n" {
		t.Errorf("--version: %q, %v", out, err)
	}
	out, err = execute(t, "--help")
	if err != nil || !strings.Contains(out, "depphunter [path]") || !strings.Contains(out, "--history-commits") {
		t.Errorf("--help: %v\n%s", err, out)
	}
	if _, err := execute(t, "a", "b"); err == nil {
		t.Error("two paths accepted")
	}
	if _, err := execute(t, "--no-such-flag"); err == nil {
		t.Error("unknown flag accepted")
	}
}

// Verifies: REQ-CLI-001, REQ-CLI-004, REQ-EXP-004
func TestExportFromCommand(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DEPPHUNTER_CACHE", "false")
	out := filepath.Join(t.TempDir(), "g.json")
	if _, err := execute(t, "--no-history", "--export", "json", "-o", out, root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var g struct{ Edges []struct{ To string } }
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Edges) == 0 || !strings.Contains(g.Edges[0].To, "fmt") {
		t.Errorf("edges: %+v", g.Edges)
	}
}

func TestLatestCoalesces(t *testing.T) {
	var mu sync.Mutex
	var seen []int
	release := make(chan struct{})
	l := newLatest(func(v int) {
		if v == 1 {
			<-release // hold the first run while more values arrive
		}
		mu.Lock()
		seen = append(seen, v)
		mu.Unlock()
	})
	done := make(chan struct{})
	go func() { l.Run(1); close(done) }()
	time.Sleep(20 * time.Millisecond)
	for v := 2; v <= 5; v++ {
		l.Run(v) // returns at once: a run is in progress
	}
	close(release)
	<-done
	if len(seen) != 2 || seen[0] != 1 || seen[1] != 5 {
		t.Errorf("runs %v, want [1 5]", seen)
	}
}

// TestLogOutput checks where the log goes. It is stdout, so that what depphunter
// says can be piped and read like any other output - except where stdout is already
// carrying the export, which would otherwise have "analyzed …" written into the
// middle of it.
//
// Verifies: REQ-CLI-009, REQ-CLI-010
func TestLogOutput(t *testing.T) {
	for _, c := range []struct {
		name           string
		export, output string
		want           *os.File
	}{
		{"serving", "", "", os.Stdout},
		{"export to a file", "json", "g.json", os.Stdout},
		{"export to stdout", "json", "", os.Stderr},
		// -o without --export is refused by the configuration, but the rule here
		// reads off Export either way.
		{"an output with no export", "", "g.json", os.Stdout},
	} {
		if got := logOutput(config.Config{Export: c.export, Output: c.output}); got != c.want {
			t.Errorf("%s: the log goes to %v, want %v", c.name, got, c.want)
		}
	}
}

// TestExportToStdoutIsOnlyTheExport is the reason for the exception above: a caller
// that redirects the export has to get a document and nothing else.
//
// Verifies: REQ-CLI-010, REQ-EXP-004
func TestExportToStdoutIsOnlyTheExport(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"),
		[]byte("package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DEPPHUNTER_CACHE", "false")

	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	real := os.Stdout
	os.Stdout = write
	done := make(chan []byte)
	go func() {
		out, _ := io.ReadAll(read)
		done <- out
	}()
	// --explain as well, so the longest thing depphunter writes is in the run.
	_, runErr := execute(t, "--no-history", "--explain", "--export", "json", root)
	os.Stdout = real
	write.Close()
	out := <-done
	read.Close()
	if runErr != nil {
		t.Fatal(runErr)
	}

	var g struct{ Nodes []struct{ ID string } }
	if err := json.Unmarshal(out, &g); err != nil {
		t.Fatalf("stdout is not the export alone: %v\n%s", err, out)
	}
	if len(g.Nodes) == 0 {
		t.Error("the export carried no nodes")
	}
}

// A link's answer is kept for a day; an advisory, which changes faster, for six
// hours. The two caches have their own lifetimes.
//
// Verifies: REQ-MD-014, REQ-FND-012
func TestLinkAnswersOutliveAdvisories(t *testing.T) {
	if linkCacheTTL != 24*time.Hour {
		t.Errorf("link answers kept for %v, want a day", linkCacheTTL)
	}
	if findingsCacheTTL != 6*time.Hour {
		t.Errorf("advisories kept for %v, want six hours", findingsCacheTTL)
	}
}

// Markdown that is not this repository's own writing - vendored dependencies'
// READMEs and fixtures under testdata - is never handed to the link check, at
// whatever depth it sits.
//
// Verifies: REQ-MD-015
func TestVendoredDocsAndTestdataAreNotLinkChecked(t *testing.T) {
	g := &graph.Graph{}
	for _, p := range []string{
		"README.md", "docs/guide.md", "internal/x/notes.md",
		"vendor/github.com/a/b/README.md", "web/node_modules/pkg/README.md",
		"third_party/lib/README.md", "thirdparty/lib/README.md",
		"lib/site-packages/pkg/README.md", ".venv/lib/README.md", "venv/README.md",
		"internal/x/testdata/README.md",
	} {
		g.Nodes = append(g.Nodes, &graph.Node{ID: graph.FileID(p), Kind: graph.KindFile, Path: p, Lang: "Markdown"})
	}
	// Not Markdown, so not a document either way.
	g.Nodes = append(g.Nodes, &graph.Node{ID: graph.FileID("main.go"), Kind: graph.KindFile, Path: "main.go", Lang: "Go"})

	got := documents(g)
	want := []string{"README.md", "docs/guide.md", "internal/x/notes.md"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("documents %v, want %v", got, want)
	}
}

// TestPrivatePackagesAreNotAskedAbout runs an analysis with --private and reads the
// packages the vulnerability database would be asked about off the map it drew:
// the organization's own package is on the map, pinned like the public one, and is
// still not among them.
//
// Verifies: REQ-SUP-040
func TestPrivatePackagesAreNotAskedAbout(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":  "module app\n\ngo 1.22\n\nrequire (\n\tcorp.example/lib v1.0.0\n\tgithub.com/pub/lib v1.2.0\n)\n",
		"main.go": "package main\n\nimport (\n\t_ \"corp.example/lib\"\n\t_ \"github.com/pub/lib\"\n)\n\nfunc main() {}\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("DEPPHUNTER_CACHE", "false")
	t.Setenv("GOPRIVATE", "")
	t.Setenv("GONOPROXY", "")
	out := filepath.Join(t.TempDir(), "g.json")
	if _, err := execute(t, "--no-history", "--no-links", "--private", "corp.example/*",
		"--export", "json", "-o", out, root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var g graph.Graph
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	onMap := false
	for _, n := range g.Nodes {
		if n.Name == "corp.example/lib" && n.Private && n.Version == "v1.0.0" {
			onMap = true
		}
	}
	if !onMap {
		t.Fatal("the private package is not on the map as a pinned private package")
	}
	var asked []string
	for _, p := range pinned(&g) {
		asked = append(asked, p.Name)
		if strings.HasPrefix(p.Name, "corp.example/") {
			t.Errorf("%s would be sent to the vulnerability database", p.Name)
		}
	}
	if len(asked) != 1 || asked[0] != "github.com/pub/lib" {
		t.Errorf("asked about %v, want the public package only", asked)
	}
}

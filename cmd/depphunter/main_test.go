package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/config"
	"github.com/sarumaj/depphunter-cli/internal/graph"
	"github.com/sarumaj/depphunter-cli/internal/scope"
)

// execute runs the command with arguments and returns what it printed.
func execute(t *testing.T, arguments ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	command := newCommand()
	command.SetOut(&out)
	command.SetErr(&out)
	command.SetArgs(arguments)
	err := command.ExecuteContext(context.Background())
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
		g.Nodes = append(g.Nodes, &graph.Node{ID: graph.FileID(p), Kind: graph.KindFile, Path: p, Language: "Markdown"})
	}
	// Not Markdown, so not a document either way.
	g.Nodes = append(g.Nodes, &graph.Node{ID: graph.FileID("main.go"), Kind: graph.KindFile, Path: "main.go", Language: "Go"})

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
	for _, p := range pinned(&g, nil) {
		asked = append(asked, p.Name)
		if strings.HasPrefix(p.Name, "corp.example/") {
			t.Errorf("%s would be sent to the vulnerability database", p.Name)
		}
	}
	if len(asked) != 1 || asked[0] != "github.com/pub/lib" {
		t.Errorf("asked about %v, want the public package only", asked)
	}
}

// TestCommitPinnedPackagesAreAskedByCommit reads what the vulnerability database
// would be asked about the git dependencies of a map: a full commit on a public
// forge is asked about; a commit on a company host, of a repository or package a
// private pattern names, or of a package private by name is not, nor is a
// shortened one; a package private only for having been installed from GitHub is
// asked about by its commit alone; and a version that is itself a git reference is
// not sent as a version.
//
// Verifies: REQ-FND-026, REQ-SUP-040
func TestCommitPinnedPackagesAreAskedByCommit(t *testing.T) {
	const sha = "0123456789abcdef0123456789abcdef01234567"
	packageName := func(ecosystem, name, version string, set func(*graph.Node)) *graph.Node {
		n := &graph.Node{ID: graph.PackageID(ecosystem, name), Kind: graph.KindPackage, Name: name,
			Parent: graph.EcosystemID(ecosystem), Version: version}
		if set != nil {
			set(n)
		}
		return n
	}
	g := &graph.Graph{Nodes: []*graph.Node{
		packageName("zig", "github.com/ziglibs/known-folders", sha, nil),
		packageName("carthage", "git.example.com/ios/Kit", sha, nil),
		packageName("jsonnet-bundler", "git.acme.internal/ops/libs", sha, func(n *graph.Node) {
			n.Origin, n.Private = "git@git.acme.internal:ops/libs.git", true
		}),
		packageName("hex", "phoenix", sha, func(n *graph.Node) {
			n.Origin, n.Private = "https://github.com/phoenixframework/phoenix.git", true
		}),
		packageName("swiftpm", "github.com/acme/private-kit", sha, nil),
		packageName("shards", "internal-shard", sha, func(n *graph.Node) { n.Private = true }),
		packageName("shards", "markd", sha, nil),
		packageName("paket", "github.com/fsharp/FAKE", sha, func(n *graph.Node) { n.Private = true }),
		packageName("nix", "github.com/numtide/flake-utils", "b1d9ab7", func(n *graph.Node) {
			n.Git = "https://github.com/numtide/flake-utils#" + sha
		}),
		packageName("npm", "forge-std", "github:foundry-rs/forge-std#1eea5ba", nil),
		packageName("npm", "left-pad", "github:stevemao/left-pad#"+sha, nil),
		packageName("pypi", "foo", "@ git+https://git.corp.example/o/foo@"+sha, func(n *graph.Node) {
			n.Git = "https://git.corp.example/o/foo#" + sha
		}),
		packageName("pypi", "bar", "@ git+https://github.com/o/bar@"+sha, func(n *graph.Node) {
			n.Git = "https://github.com/o/bar#" + sha
		}),
		packageName("fpm", "floating", sha, func(n *graph.Node) { n.Floating = true }),
	}}
	private := scope.New([]string{"github.com/acme/*", "shards:internal-*"})
	var got []string
	for _, p := range pinned(g, private.Match) {
		s := p.Ecosystem + " " + p.Name
		if p.Commit != "" {
			s += " commit=" + p.Repository
		}
		if p.CommitOnly {
			s += " only"
		}
		got = append(got, s)
	}
	sort.Strings(got)
	want := []string{
		"carthage git.example.com/ios/Kit", // named, as before; its commit stays here
		"hex phoenix commit=github.com/phoenixframework/phoenix only",
		"nix github.com/numtide/flake-utils commit=github.com/numtide/flake-utils",
		"npm forge-std", // Bun's shortened commit: the version is sent, as before
		"npm left-pad commit=github.com/stevemao/left-pad only",
		"pypi bar commit=github.com/o/bar only",
		"shards markd commit=",
		"swiftpm github.com/acme/private-kit",
		"zig github.com/ziglibs/known-folders commit=github.com/ziglibs/known-folders",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("asked\n%q\nwant\n%q", got, want)
	}
}

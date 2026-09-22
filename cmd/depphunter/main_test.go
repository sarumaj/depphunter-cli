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

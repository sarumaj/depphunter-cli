package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
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

package server

import (
	"bufio"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/config"
	"github.com/sarumaj/depphunter-cli/internal/graph"
)

func start(t *testing.T, mods ...func(*config.Config)) (*Server, string, string) {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o644)
	os.WriteFile(filepath.Join(root, "secret.txt"), []byte("not in graph"), 0o644)

	cfg := config.Default()
	cfg.Root = root
	for _, m := range mods {
		m(&cfg)
	}
	g := &graph.Graph{Nodes: []*graph.Node{{ID: graph.FileID("a.go"), Kind: graph.KindFile, Path: "a.go"}}}
	s, err := New(cfg, g, fstest.MapFS{"index.html": {Data: []byte("<!doctype html>ui")}})
	if err != nil {
		t.Fatal(err)
	}
	ln, url, err := s.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &httptest.Server{Listener: ln, Config: &http.Server{Handler: s.Handler()}}
	srv.Start()
	t.Cleanup(srv.Close)
	t.Cleanup(s.Close) // runs first: ends event streams so srv.Close does not wait on them
	return s, url, strings.TrimSuffix(url[:strings.Index(url, "?")], "/")
}

func get(t *testing.T, c *http.Client, url string, mod func(*http.Request)) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if mod != nil {
		mod(req)
	}
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b)
}

func TestTokenExchangeAndAccess(t *testing.T) {
	_, url, base := start(t)
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar}

	if code, _ := get(t, http.DefaultClient, base+"/api/graph", nil); code != http.StatusUnauthorized {
		t.Errorf("without token: got %d", code)
	}
	if code, _ := get(t, http.DefaultClient, base+"/?token=wrong", nil); code != http.StatusUnauthorized {
		t.Errorf("wrong token: got %d", code)
	}
	if code, body := get(t, c, url, nil); code != http.StatusOK || !strings.Contains(body, "ui") {
		t.Errorf("token URL should redirect to the UI: %d %q", code, body)
	}
	if code, body := get(t, c, base+"/api/graph", nil); code != http.StatusOK || !strings.Contains(body, `"a.go"`) {
		t.Errorf("graph with cookie: %d %q", code, body)
	}
}

func TestFileAllowList(t *testing.T) {
	_, url, base := start(t)
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar}
	get(t, c, url, nil)

	if code, body := get(t, c, base+"/api/file?path=a.go", nil); code != http.StatusOK || body != "package a\n" {
		t.Errorf("graph file: %d %q", code, body)
	}
	for _, p := range []string{"secret.txt", "../a.go", "/etc/passwd", "./a.go"} {
		if code, _ := get(t, c, base+"/api/file?path="+p, nil); code != http.StatusNotFound {
			t.Errorf("%s: got %d, want 404", p, code)
		}
	}
}

func TestForeignHostRejected(t *testing.T) {
	_, url, _ := start(t)
	code, _ := get(t, http.DefaultClient, url, func(r *http.Request) { r.Host = "attacker.example:80" })
	if code != http.StatusForbidden {
		t.Errorf("foreign host: got %d, want 403", code)
	}
}

func login(t *testing.T, url string) *http.Client {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar}
	get(t, c, url, nil)
	return c
}

func TestUpdatePushesEvents(t *testing.T) {
	s, url, base := start(t)
	c := login(t, url)
	res, err := c.Get(base + "/api/events")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	events := make(chan string, 4)
	go func() {
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			if line := sc.Text(); strings.HasPrefix(line, "data: ") {
				events <- line
			}
		}
	}()
	next := func() string {
		select {
		case e := <-events:
			return e
		case <-time.After(3 * time.Second):
			t.Fatal("no event")
			return ""
		}
	}
	if e := next(); e != `data: {"version":1}` {
		t.Fatalf("hello: %s", e)
	}

	same := &graph.Graph{Nodes: []*graph.Node{{ID: graph.FileID("a.go"), Kind: graph.KindFile, Path: "a.go"}}}
	if changed, _ := s.Update(same, []string{"a.go"}); changed {
		t.Error("identical graph reported as a change")
	}
	g := &graph.Graph{Nodes: []*graph.Node{
		{ID: graph.FileID("a.go"), Kind: graph.KindFile, Path: "a.go", LOC: 2},
		{ID: graph.FileID("b.go"), Kind: graph.KindFile, Path: "b.go"},
	}}
	if changed, err := s.Update(g, nil); !changed || err != nil {
		t.Fatalf("update: %v %v", changed, err)
	}
	if e := next(); e != `data: {"version":2,"changed":["a.go","b.go"]}` {
		t.Fatalf("update event: %s", e)
	}
	if code, body := get(t, c, base+"/api/graph", nil); code != 200 || !strings.Contains(body, "b.go") {
		t.Errorf("graph after update: %d %s", code, body)
	}
}

func TestExportDownload(t *testing.T) {
	_, url, base := start(t)
	c := login(t, url)
	for format, want := range map[string]string{"json": `"a.go"`, "graphml": "<graphml", "dot": "digraph"} {
		if code, body := get(t, c, base+"/api/export?format="+format, nil); code != 200 || !strings.Contains(body, want) {
			t.Errorf("%s: %d %.80s", format, code, body)
		}
	}
	if code, _ := get(t, c, base+"/api/export?format=svg", nil); code != http.StatusBadRequest {
		t.Errorf("svg: got %d", code)
	}
}

func TestOpenInEditor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh")
	}
	marker := filepath.Join(t.TempDir(), "opened")
	_, url, base := start(t, func(c *config.Config) {
		c.Editor = `/bin/sh -c "echo {file}:{line} > ` + marker + `"`
	})
	c := login(t, url)
	post := func(body string, header bool) int {
		req, _ := http.NewRequest(http.MethodPost, base+"/api/open", strings.NewReader(body))
		if header {
			req.Header.Set(requestHeader, "1")
		}
		res, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	if code := post(`{"path":"a.go","line":3}`, false); code != http.StatusForbidden {
		t.Errorf("without CSRF header: %d", code)
	}
	if code := post(`{"path":"secret.txt","line":1}`, true); code != http.StatusNotFound {
		t.Errorf("file outside graph: %d", code)
	}
	if code := post(`{"path":"a.go","line":3}`, true); code != http.StatusNoContent {
		t.Fatalf("open: %d", code)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(marker); err == nil && strings.HasSuffix(strings.TrimSpace(string(b)), "a.go:3") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Error("editor command did not run")
}

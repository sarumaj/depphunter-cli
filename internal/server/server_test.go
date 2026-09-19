package server

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sarumaj/depphunter-cli/internal/config"
	"github.com/sarumaj/depphunter-cli/internal/graph"
)

func start(t *testing.T) (*Server, string, string) {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o644)
	os.WriteFile(filepath.Join(root, "secret.txt"), []byte("not in graph"), 0o644)

	cfg := config.Default()
	cfg.Root = root
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

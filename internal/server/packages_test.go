package server

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/graph"
)

// withPackages puts lodash on the map, imported by a.go and installed beside it, and
// left-pad, which links nowhere and is installed nowhere.
func withPackages(t *testing.T, s *Server) string {
	t.Helper()
	installed := filepath.Join(s.root, "node_modules", "lodash")
	if err := os.MkdirAll(installed, 0o755); err != nil {
		t.Fatal(err)
	}
	g := &graph.Graph{
		Nodes: []*graph.Node{
			{ID: graph.FileID("a.go"), Kind: graph.KindFile, Path: "a.go"},
			{ID: graph.EcosystemID("npm"), Kind: graph.KindEcosystem, Name: "npm"},
			{ID: graph.PackageID("npm", "lodash"), Kind: graph.KindPackage, Name: "lodash", Parent: graph.EcosystemID("npm"),
				Page: "https://www.npmjs.com/package/lodash", Repository: "https://github.com/lodash/lodash"},
			{ID: graph.PackageID("npm", "left-pad"), Kind: graph.KindPackage, Name: "left-pad", Parent: graph.EcosystemID("npm")},
		},
		Edges: []*graph.Edge{{From: graph.FileID("a.go"), To: graph.PackageID("npm", "lodash"), Kind: graph.EdgeImport}},
	}
	if _, err := s.Update(g, nil); err != nil {
		t.Fatal(err)
	}
	return installed
}

// Verifies: REQ-SRV-018
func TestLocate(t *testing.T) {
	s, url, base := start(t)
	installed := withPackages(t, s)
	c := login(t, url)
	code, body := get(t, c, base+"/api/locate?id=p:npm:lodash", nil)
	var located struct{ Folder string }
	if err := json.Unmarshal([]byte(body), &located); code != http.StatusOK || err != nil || located.Folder != installed {
		t.Errorf("lodash: %d %s", code, body)
	}
	for _, id := range []string{"p:npm:left-pad", "p:npm:absent", "f:a.go"} {
		if code, _ := get(t, c, base+"/api/locate?id="+id, nil); code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", id, code)
		}
	}
}

// Verifies: REQ-SRV-019
func TestBrowse(t *testing.T) {
	s, url, base := start(t)
	installed := withPackages(t, s)
	c := login(t, url)
	var revealed string
	defer func(original func(string, string) *exec.Cmd) { reveal = original }(reveal)
	reveal = func(_, directory string) *exec.Cmd {
		revealed = directory
		return exec.Command(os.Args[0], "-test.run=^$")
	}
	post := func(body string) int {
		request, _ := http.NewRequest(http.MethodPost, base+"/api/browse", strings.NewReader(body))
		request.Header.Set(requestHeader, "1")
		response, err := c.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		return response.StatusCode
	}

	// Nothing that opens links is listening: the page opens a link itself, and the
	// server shows a folder in the file manager.
	if code := post(`{"id":"p:npm:lodash","to":"page"}`); code != http.StatusNotImplemented {
		t.Errorf("page, nobody listening: %d", code)
	}
	if code := post(`{"id":"p:npm:lodash","to":"folder"}`); code != http.StatusNoContent || revealed != installed {
		t.Errorf("folder, nobody listening: %d, revealed %q", code, revealed)
	}
	for body, want := range map[string]int{
		`{"id":"p:npm:left-pad","to":"repository"}`: http.StatusNotFound,
		`{"id":"p:npm:left-pad","to":"folder"}`:     http.StatusNotFound,
		`{"id":"p:npm:absent","to":"page"}`:         http.StatusNotFound,
		`{"id":"p:npm:lodash","to":"anywhere"}`:     http.StatusBadRequest,
	} {
		if code := post(body); code != want {
			t.Errorf("%s: %d, want %d", body, code, want)
		}
	}

	// The extension's stream opens links; a page's does not.
	page, err := c.Get(base + "/api/events")
	if err != nil {
		t.Fatal(err)
	}
	defer page.Body.Close()
	extension, err := c.Get(base + "/api/events?opens=hex,links")
	if err != nil {
		t.Fatal(err)
	}
	defer extension.Body.Close()
	fromPage, fromExtension := streamLines(page.Body), streamLines(extension.Body)
	for _, from := range []<-chan string{fromPage, fromExtension} {
		if !waitForLine(from, "event: hello", 2*time.Second) {
			t.Fatal("a stream never said hello")
		}
	}
	revealed = ""
	if code := post(`{"id":"p:npm:lodash","to":"repository"}`); code != http.StatusAccepted {
		t.Errorf("repository, the extension listening: %d", code)
	}
	if !waitForLine(fromExtension, `"url":"https://github.com/lodash/lodash"`, 2*time.Second) {
		t.Error("the extension was not handed the repository")
	}
	if code := post(`{"id":"p:npm:lodash","to":"folder"}`); code != http.StatusAccepted || revealed != "" {
		t.Errorf("folder, the extension listening: %d, revealed here %q", code, revealed)
	}
	if !waitForLine(fromExtension, `"folder":`, 2*time.Second) {
		t.Error("the extension was not handed the folder")
	}
	if waitForLine(fromPage, "event: browse", 300*time.Millisecond) {
		t.Error("a page was handed what to open as well")
	}
}

func streamLines(r io.Reader) <-chan string {
	out := make(chan string, 16)
	go func() {
		s := bufio.NewScanner(r)
		for s.Scan() {
			out <- s.Text()
		}
		close(out)
	}()
	return out
}

func waitForLine(from <-chan string, want string, d time.Duration) bool {
	deadline := time.After(d)
	for {
		select {
		case l, ok := <-from:
			if !ok {
				return false
			}
			if strings.Contains(l, want) {
				return true
			}
		case <-deadline:
			return false
		}
	}
}

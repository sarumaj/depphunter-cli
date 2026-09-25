package server

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	neturl "net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/config"
	"github.com/sarumaj/depphunter-cli/internal/findings"
	"github.com/sarumaj/depphunter-cli/internal/graph"
	"github.com/sarumaj/depphunter-cli/internal/history"
	"github.com/sarumaj/depphunter-cli/internal/trace"
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

// Verifies: REQ-SEC-002, REQ-SEC-003, REQ-SEC-004
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

// Verifies: REQ-SEC-003
func TestTwoServersOneBrowser(t *testing.T) {
	// Browsers keep cookies per host, not per port: one jar for both servers is what a
	// browser with two maps open looks like.
	_, url1, base1 := start(t)
	_, url2, base2 := start(t)
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar}
	get(t, c, url1, nil)
	get(t, c, url2, nil)
	for _, base := range []string{base1, base2} {
		if code, _ := get(t, c, base+"/api/graph", nil); code != http.StatusOK {
			t.Errorf("%s after logging in to both: got %d", base, code)
		}
	}
}

// Verifies: REQ-SEC-006, REQ-SRV-003
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

// Verifies: REQ-SEC-005
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

// Verifies: REQ-WATCH-004, REQ-SRV-016
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
	var hello struct {
		Version int    `json:"version"`
		ETag    string `json:"etag"`
		Resumed bool   `json:"resumed"`
	}
	if e := next(); json.Unmarshal([]byte(strings.TrimPrefix(e, "data: ")), &hello) != nil || hello.Version != 1 {
		t.Fatalf("hello: %s", e)
	}
	// A first connection has nothing to resume, and the greeting carries the
	// validator so a client can ask whether what it already holds is still current.
	if hello.Resumed || hello.ETag == "" {
		t.Errorf("hello: resumed %v etag %q", hello.Resumed, hello.ETag)
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

// Verifies: REQ-EXP-005
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

// Verifies: REQ-SEC-007, REQ-SEC-008, REQ-SRV-005
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

// Verifies: REQ-CFG-012, REQ-CFG-014, REQ-CFG-015, REQ-SEC-007, REQ-SRV-004
func TestSaveSettings(t *testing.T) {
	file := filepath.Join(t.TempDir(), config.ProjectFile)
	_, url, base := start(t, func(c *config.Config) { c.ConfigFile = file })
	c := login(t, url)
	post := func(body string, header bool) int {
		req, _ := http.NewRequest(http.MethodPost, base+"/api/settings", strings.NewReader(body))
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
	good := `{"theme":"dark","style":"galaxy","colorBy":"size","heightScale":"log","expandDepth":2,"hideLanguages":["Go"]}`
	if code := post(good, false); code != http.StatusForbidden {
		t.Errorf("without CSRF header: %d", code)
	}
	if code := post(`{"theme":"neon","colorBy":"size","heightScale":"log"}`, true); code != http.StatusBadRequest {
		t.Errorf("invalid theme: %d", code)
	}
	if code := post(`{"theme":"dark","style":"swamp","colorBy":"size","heightScale":"log"}`, true); code != http.StatusBadRequest {
		t.Errorf("invalid style: %d", code)
	}
	if code := post(good, true); code != http.StatusNoContent {
		t.Fatalf("save: %d", code)
	}
	if data, err := os.ReadFile(file); err != nil || !strings.Contains(string(data), "height_scale: log") ||
		!strings.Contains(string(data), "style: galaxy") {
		t.Errorf("config file: %v\n%s", err, data)
	}
	if code, body := get(t, c, base+"/api/config", nil); code != 200 || !strings.Contains(body, `"heightScale":"log"`) {
		t.Errorf("config after save: %d %s", code, body)
	}
}

// Verifies: REQ-EXP-005
func TestStaticExportDownload(t *testing.T) {
	_, url, base := start(t)
	c := login(t, url)
	if code, body := get(t, c, base+"/api/export?format=html", nil); code != 200 || !strings.Contains(body, `id="depphunter-data"`) {
		t.Errorf("html export: %d %.100s", code, body)
	}
}

// Verifies: REQ-HIST-007, REQ-HIST-008
func TestHistoryLifecycle(t *testing.T) {
	s, url, base := start(t, func(c *config.Config) { c.History = true })
	c := login(t, url)
	if code, _ := get(t, c, base+"/api/history", nil); code != http.StatusAccepted {
		t.Errorf("while reading: %d, want 202", code)
	}

	res, err := c.Get(base + "/api/events")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	events := make(chan string, 4)
	go func() {
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			if line := sc.Text(); strings.HasPrefix(line, "event: ") {
				events <- strings.TrimPrefix(line, "event: ")
			}
		}
	}()
	<-events // hello

	h := &history.History{Head: "abc", Commits: 1, Authors: []string{"Ann"},
		Files: map[string][]history.Change{"a.go": {{1000, 0, 2, 0, 0}}}}
	if err := s.SetHistory(h); err != nil {
		t.Fatal(err)
	}
	select {
	case e := <-events:
		if e != "history" {
			t.Errorf("event %q, want history", e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no history event")
	}
	if code, body := get(t, c, base+"/api/history", nil); code != 200 || !strings.Contains(body, `"a.go":[[1000,0,2,0,0]]`) {
		t.Errorf("history: %d %s", code, body)
	}
	// A page loaded again revalidates rather than downloading it all again.
	res, err = c.Get(base + "/api/history")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	etag := res.Header.Get("ETag")
	if etag == "" {
		t.Fatal("the history carries no entity tag")
	}
	if code, _ := get(t, c, base+"/api/history", func(r *http.Request) { r.Header.Set("If-None-Match", etag) }); code != http.StatusNotModified {
		t.Errorf("unchanged history: %d, want 304", code)
	}
	s.SetHistory(nil)
	if code, _ := get(t, c, base+"/api/history", nil); code != http.StatusNoContent {
		t.Errorf("without history: %d, want 204", code)
	}
}

// Embed mode is what an editor's built-in browser needs: the map has to be allowed
// into a frame, and the cookie the server would normally hand out is no use there,
// so the token comes back on every call instead. None of it may leak into the
// ordinary mode, which is what the second half of each of these checks.

func embedded(t *testing.T) (string, string, string) {
	t.Helper()
	s, url, base := start(t, func(c *config.Config) { c.Embed = []string{"vscode-webview:"} })
	return s.token, url, base
}

func headers(t *testing.T, url string) http.Header {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res.Header
}

func TestEmbedAllowsTheFrameItWasTold(t *testing.T) {
	_, url, _ := embedded(t)
	h := headers(t, url)
	if got := h.Get("Content-Security-Policy"); !strings.Contains(got, "frame-ancestors vscode-webview:") {
		t.Errorf("embed CSP: %q", got)
	}
	if got := h.Get("X-Frame-Options"); got != "" {
		// It has no syntax for naming an origin that browsers still honour, so in
		// embed mode it can only be absent; the CSP above is the whole of the answer.
		t.Errorf("X-Frame-Options in embed mode: %q", got)
	}

	_, plain, _ := start(t)
	h = headers(t, plain)
	if got := h.Get("Content-Security-Policy"); !strings.Contains(got, "frame-ancestors 'none'") {
		t.Errorf("default CSP: %q", got)
	}
	if got := h.Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("default X-Frame-Options: %q", got)
	}
}

func TestEmbedKeepsTheTokenInTheAddress(t *testing.T) {
	token, url, base := embedded(t)
	// No redirect and no cookie: the page needs the token where it can read it.
	res, err := (&http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}).Get(url)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Errorf("document: got %d, want 200", res.StatusCode)
	}
	if len(res.Cookies()) != 0 {
		t.Errorf("embed mode set a cookie: %v", res.Cookies())
	}

	// The token is taken from a header, and from the query for what cannot set one.
	if code, body := get(t, http.DefaultClient, base+"/api/graph", func(r *http.Request) {
		r.Header.Set(tokenHeader, token)
	}); code != http.StatusOK || !strings.Contains(body, `"a.go"`) {
		t.Errorf("graph with the token header: %d %q", code, body)
	}
	if code, _ := get(t, http.DefaultClient, base+"/api/graph?token="+token, nil); code != http.StatusOK {
		t.Errorf("graph with the token in the query: got %d", code)
	}
	if code, _ := get(t, http.DefaultClient, base+"/api/graph", func(r *http.Request) {
		r.Header.Set(tokenHeader, "wrong")
	}); code != http.StatusUnauthorized {
		t.Errorf("graph with a wrong token: got %d", code)
	}
}

func TestEmbedServesTheInterfaceButNotTheProject(t *testing.T) {
	_, _, base := embedded(t)
	// The UI's own files are the same bytes in every release and say nothing about
	// the project, so a frame can load them without the token it cannot attach.
	if code, body := get(t, http.DefaultClient, base+"/index.html", nil); code != http.StatusOK || !strings.Contains(body, "ui") {
		t.Errorf("asset without a token: %d %q", code, body)
	}
	// Everything that knows anything still needs it, and so does the document that
	// carries the token to the page.
	for _, path := range []string{"/", "/api/graph", "/api/config", "/api/file?path=a.go"} {
		if code, _ := get(t, http.DefaultClient, base+path, nil); code != http.StatusUnauthorized {
			t.Errorf("%s without a token: got %d, want 401", path, code)
		}
	}

	// And none of that applies without --embed: there, an asset needs the cookie.
	_, _, plain := start(t)
	if code, _ := get(t, http.DefaultClient, plain+"/index.html", nil); code != http.StatusUnauthorized {
		t.Errorf("asset without --embed: got %d, want 401", code)
	}
}

// Verifies: REQ-SEC-007
func TestEmbedStillRefusesAWriteWithoutTheRequestHeader(t *testing.T) {
	token, _, base := embedded(t)
	req, _ := http.NewRequest(http.MethodPost, base+"/api/settings", strings.NewReader("{}"))
	req.Header.Set(tokenHeader, token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("POST without %s: got %d, want 403", requestHeader, res.StatusCode)
	}
}

// The document the server hands out is assembled rather than marshalled from the
// graph, so that the nodes and the edges are encoded once instead of twice (see
// newSnapshot). That makes it possible to add a field to graph.Graph and quietly stop
// serving it, which is what this is here to catch.
//
// Verifies: REQ-SRV-002
func TestServedGraphMatchesTheDocument(t *testing.T) {
	g := &graph.Graph{
		Root:        "repo",
		GeneratedAt: time.Date(2024, 5, 4, 3, 2, 1, 0, time.UTC),
		Nodes: []*graph.Node{
			{ID: "f:a.go", Kind: graph.KindFile, Name: "a.go", Path: "a.go", Lang: "Go", LOC: 12},
			{ID: "p:go:example.com/x", Kind: graph.KindPackage, Name: "example.com/x", Version: "v1.2.3",
				Requested: "v1.2", Floating: true, Transitive: true, Index: "https://proxy.golang.org", IndexUnknown: true},
		},
		Edges: []*graph.Edge{{From: "f:a.go", To: "p:go:example.com/x", Kind: graph.EdgeImport, Line: 3}},
	}
	sn, err := newSnapshot(g, 1)
	if err != nil {
		t.Fatal(err)
	}
	want, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	if string(sn.json) != string(want) {
		t.Errorf("the served document is not the graph:\n got %s\nwant %s", sn.json, want)
	}

	// And the fingerprint is of what the map is of, not of when it was made: the same
	// analysis a minute later is the same analysis.
	later := *g
	later.GeneratedAt = g.GeneratedAt.Add(time.Minute)
	again, err := newSnapshot(&later, 2)
	if err != nil {
		t.Fatal(err)
	}
	if again.fingerprint != sn.fingerprint {
		t.Error("a later run of the same analysis fingerprinted differently")
	}
}

// Verifies: REQ-SRV-013, REQ-SRV-014
func TestGraphAnswersNotModifiedForWhatTheClientHolds(t *testing.T) {
	s, url, base := start(t)
	c := login(t, url)

	res, err := c.Get(base + "/api/graph")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	etag := res.Header.Get("ETag")
	if etag == "" {
		t.Fatal("no ETag on the graph")
	}
	// A validator is no use to a client that was told not to keep the document.
	if cc := res.Header.Get("Cache-Control"); cc == "no-store" {
		t.Errorf("Cache-Control %q forbids the store an ETag is for", cc)
	}

	ask := func(match string) int {
		req, _ := http.NewRequest(http.MethodGet, base+"/api/graph", nil)
		req.Header.Set("If-None-Match", match)
		res, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	if code := ask(etag); code != http.StatusNotModified {
		t.Errorf("holding the current graph: %d", code)
	}
	// The forms a cache is allowed to send.
	if code := ask(`"stale", ` + etag); code != http.StatusNotModified {
		t.Errorf("a list holding it: %d", code)
	}
	if code := ask("W/" + etag); code != http.StatusNotModified {
		t.Errorf("weakened: %d", code)
	}
	if code := ask("*"); code != http.StatusNotModified {
		t.Errorf("*: %d", code)
	}
	if code := ask(`"something-else"`); code != http.StatusOK {
		t.Errorf("holding something else: %d", code)
	}

	// A re-analysis that found the same project is the same entity, however many
	// times it is read: the fingerprint is of the graph, not of the reading.
	same := &graph.Graph{Nodes: []*graph.Node{{ID: graph.FileID("a.go"), Kind: graph.KindFile, Path: "a.go"}}}
	if changed, _ := s.Update(same, nil); changed {
		t.Fatal("an identical graph was reported as a change")
	}
	if code := ask(etag); code != http.StatusNotModified {
		t.Errorf("after an identical re-analysis: %d", code)
	}

	// A real change is a new entity.
	g := &graph.Graph{Nodes: []*graph.Node{{ID: graph.FileID("b.go"), Kind: graph.KindFile, Path: "b.go"}}}
	if changed, err := s.Update(g, nil); !changed || err != nil {
		t.Fatalf("update: %v %v", changed, err)
	}
	if code := ask(etag); code != http.StatusOK {
		t.Errorf("after the graph changed: %d", code)
	}
}

// Verifies: REQ-SRV-016
func TestAReconnectingStreamIsToldWhetherItMissedAnything(t *testing.T) {
	s, url, base := start(t)
	c := login(t, url)

	hello := func(lastID string) (seq uint64, wasResumed bool) {
		req, _ := http.NewRequest(http.MethodGet, base+"/api/events", nil)
		if lastID != "" {
			req.Header.Set("Last-Event-ID", lastID)
		}
		res, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			data, ok := strings.CutPrefix(sc.Text(), "data: ")
			if !ok {
				continue
			}
			var h struct {
				Seq     uint64 `json:"seq"`
				Resumed bool   `json:"resumed"`
			}
			if err := json.Unmarshal([]byte(data), &h); err != nil {
				t.Fatal(err)
			}
			return h.Seq, h.Resumed
		}
		t.Fatal("no greeting")
		return 0, false
	}

	// Nothing has been announced yet, and a client with no id to hand back has
	// nothing to resume.
	seq, wasResumed := hello("")
	if seq != 0 || wasResumed {
		t.Fatalf("first connection: seq %d resumed %v", seq, wasResumed)
	}
	// Handing back the id it last saw says it is still current.
	if _, wasResumed = hello("0"); !wasResumed {
		t.Error("a client holding the latest id was told it had missed something")
	}

	// Something happens while it is away.
	g := &graph.Graph{Nodes: []*graph.Node{{ID: graph.FileID("b.go"), Kind: graph.KindFile, Path: "b.go"}}}
	if _, err := s.Update(g, nil); err != nil {
		t.Fatal(err)
	}
	seq, wasResumed = hello("0")
	if wasResumed {
		t.Error("a client that slept through an announcement was told it was up to date")
	}
	if seq == 0 {
		t.Error("the sequence did not move when something was announced")
	}
	// ... and once it has caught up it is current again.
	if _, wasResumed = hello(strconv.FormatUint(seq, 10)); !wasResumed {
		t.Error("a caught-up client was not resumed")
	}
	// Nonsense is not a resume.
	if _, wasResumed = hello("not-a-number"); wasResumed {
		t.Error("an unreadable Last-Event-ID resumed the stream")
	}
}

// Verifies: REQ-SRV-015
func TestEventsCarryTheirIdSoAClientCanResume(t *testing.T) {
	s, url, base := start(t)
	c := login(t, url)
	res, err := c.Get(base + "/api/events")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	lines := make(chan string, 16)
	go func() {
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
	}()
	next := func() string {
		select {
		case l := <-lines:
			return l
		case <-time.After(3 * time.Second):
			t.Fatal("no line")
			return ""
		}
	}
	for next() != "" { // the greeting, up to its blank line
	}
	g := &graph.Graph{Nodes: []*graph.Node{{ID: graph.FileID("b.go"), Kind: graph.KindFile, Path: "b.go"}}}
	if _, err := s.Update(g, nil); err != nil {
		t.Fatal(err)
	}
	// An announcement leads with its id, which is what EventSource remembers and
	// hands back on the next connection.
	if line := next(); line != "id: 1" {
		t.Errorf("first announcement: %q, want an id", line)
	}
	if line := next(); line != "event: graph" {
		t.Errorf("after the id: %q", line)
	}
}

// TestResolutionReportIsServedInThreeShapes checks the endpoint the editor's
// Resolution Report reads: the same account of one analysis, as data, as a document,
// and as the text the command line prints.
//
// Verifies: REQ-TRC-011, REQ-TRC-012, REQ-TRC-013
func TestResolutionReportIsServedInThreeShapes(t *testing.T) {
	s, url, base := start(t)
	c := login(t, url)

	// Before an analysis has handed one over there is nothing to serve, and saying
	// so beats serving an empty report that reads like "nothing resolved".
	if code, _ := get(t, c, base+"/api/resolution", nil); code != http.StatusNotFound {
		t.Errorf("with no report: %d, want 404", code)
	}

	rep := trace.New(2, true, []string{"corp.example/*"}, nil)
	rep.Enter("go", 0)
	rep.Add(trace.Lookup{Ecosystem: "go", Package: "corp.example/billing", Version: "v1.0.0",
		Answer: trace.NoAnswer, Reason: trace.ReasonPrivate, Index: "https://proxy.golang.org"})
	rep.Done(1, 0, 0, 0, time.Millisecond)
	rep.Finish()
	s.SetResolution(rep)

	code, body := get(t, c, base+"/api/resolution", nil)
	if code != http.StatusOK {
		t.Fatalf("json: %d", code)
	}
	var doc struct {
		ResolveDepth int      `json:"resolveDepth"`
		Private      []string `json:"private"`
		Lookups      []struct {
			Package string `json:"package"`
			Reason  string `json:"reason"`
		} `json:"lookups"`
	}
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatalf("json: %v: %s", err, body)
	}
	if doc.ResolveDepth != 2 || len(doc.Private) != 1 || len(doc.Lookups) != 1 ||
		doc.Lookups[0].Reason != trace.ReasonPrivate {
		t.Errorf("the report came back as %+v", doc)
	}

	for _, c2 := range []struct{ format, want string }{
		{"md", "# Resolution report"},
		{"text", "resolution report for"},
	} {
		code, body := get(t, c, base+"/api/resolution?format="+c2.format, nil)
		if code != http.StatusOK || !strings.Contains(body, c2.want) {
			t.Errorf("format %s: %d, %q", c2.format, code, body)
		}
		if !strings.Contains(body, "corp.example/billing") {
			t.Errorf("format %s does not name the package nothing was asked about", c2.format)
		}
	}
	if code, _ := get(t, c, base+"/api/resolution?format=csv", nil); code != http.StatusBadRequest {
		t.Errorf("an unknown format: %d, want 400", code)
	}
}

// TestHTMLExportCarriesTheViewOnScreen checks the ui parameter of an HTML export:
// the page written opens with the settings the browser sent, not those of the
// configuration, and a value that is not valid settings is refused outright.
//
// Verifies: REQ-EXP-009
func TestHTMLExportCarriesTheViewOnScreen(t *testing.T) {
	_, url, base := start(t) // configured theme: auto
	c := login(t, url)
	data := func(body string) (cfg struct {
		Theme  string `json:"theme"`
		Style  string `json:"style"`
		Static bool   `json:"static"`
	}) {
		t.Helper()
		const open = `<script type="application/json" id="depphunter-data">`
		i := strings.Index(body, open)
		if i < 0 {
			t.Fatalf("no data element in %.200s", body)
		}
		rest := body[i+len(open):]
		var doc struct {
			Config json.RawMessage `json:"config"`
		}
		if err := json.Unmarshal([]byte(rest[:strings.Index(rest, "</script>")]), &doc); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(doc.Config, &cfg); err != nil {
			t.Fatal(err)
		}
		return cfg
	}

	code, body := get(t, c, base+"/api/export?format=html", nil)
	if code != 200 || data(body).Theme != "auto" {
		t.Fatalf("without ui: %d, theme %q", code, data(body).Theme)
	}
	ui := `{"theme":"dark","colorBy":"size","heightScale":"log","style":"galaxy"}`
	code, body = get(t, c, base+"/api/export?format=html&ui="+neturl.QueryEscape(ui), nil)
	if got := data(body); code != 200 || got.Theme != "dark" || got.Style != "galaxy" || !got.Static {
		t.Errorf("with ui: %d, %+v", code, got)
	}
	for _, bad := range []string{`{"theme":"neon","colorBy":"size","heightScale":"log"}`, `not json`} {
		code, body := get(t, c, base+"/api/export?format=html&ui="+neturl.QueryEscape(bad), nil)
		if code != http.StatusBadRequest || strings.TrimSpace(body) != "invalid ui settings" {
			t.Errorf("ui %s: %d %q, want 400 invalid ui settings", bad, code, body)
		}
	}
}

// lazyEvents reads the names of the events on an open /api/events stream, after
// the greeting.
func lazyEvents(t *testing.T, c *http.Client, base string) <-chan string {
	t.Helper()
	res, err := c.Get(base + "/api/events")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	events := make(chan string, 8)
	go func() {
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			if line := sc.Text(); strings.HasPrefix(line, "event: ") {
				events <- strings.TrimPrefix(line, "event: ")
			}
		}
	}()
	<-events // hello
	return events
}

func expectEvent(t *testing.T, events <-chan string, want string) {
	t.Helper()
	select {
	case e := <-events:
		if e != want {
			t.Errorf("event %q, want %q", e, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("no %s event", want)
	}
}

func expectNoEvent(t *testing.T, events <-chan string) {
	t.Helper()
	select {
	case e := <-events:
		t.Errorf("unexpected event %q", e)
	case <-time.After(200 * time.Millisecond):
	}
}

// TestFindingsLifecycle checks the findings as a background dataset: pending while
// the reports are read, then the set with its sources, announced once - a re-read
// that found the same is not news - and 204 when nothing is left.
//
// Verifies: REQ-FND-022
func TestFindingsLifecycle(t *testing.T) {
	s, url, base := start(t, func(c *config.Config) { c.Links = true })
	c := login(t, url)
	if code, _ := get(t, c, base+"/api/findings", nil); code != http.StatusAccepted {
		t.Errorf("while reading: %d, want 202", code)
	}
	events := lazyEvents(t, c, base)

	set := func() *findings.Set {
		return &findings.Set{Sources: []string{"govulncheck"}, Partial: true, Findings: []*findings.Finding{
			{ID: "f1", Kind: "vuln", Source: "govulncheck", Ref: "GO-2024-0001", Title: "bad", Path: "a.go"}}}
	}
	if err := s.SetFindings(set()); err != nil {
		t.Fatal(err)
	}
	expectEvent(t, events, "findings")
	code, body := get(t, c, base+"/api/findings", nil)
	var doc struct {
		Findings []struct{ Ref string } `json:"findings"`
		Sources  []string               `json:"sources"`
		Partial  bool                   `json:"partial"`
	}
	if code != 200 || json.Unmarshal([]byte(body), &doc) != nil || len(doc.Findings) != 1 ||
		doc.Findings[0].Ref != "GO-2024-0001" || len(doc.Sources) != 1 || !doc.Partial {
		t.Errorf("findings: %d %s", code, body)
	}

	// The same answer again, as a --watch re-read produces it.
	if err := s.SetFindings(set()); err != nil {
		t.Fatal(err)
	}
	expectNoEvent(t, events)

	s.SetFindings(nil)
	expectEvent(t, events, "findings")
	if code, _ := get(t, c, base+"/api/findings", nil); code != http.StatusNoContent {
		t.Errorf("without findings: %d, want 204", code)
	}
}

// TestReferencesLifecycle checks the references as a background dataset: with
// --lsp they are pending until the servers finish, then served as {edges, servers,
// partial} with a references event, and 204 when no server answered.
//
// Verifies: REQ-LSP-005
func TestReferencesLifecycle(t *testing.T) {
	s, url, base := start(t, func(c *config.Config) { c.LSP = true })
	c := login(t, url)
	if code, _ := get(t, c, base+"/api/references", nil); code != http.StatusAccepted {
		t.Errorf("while the servers run: %d, want 202", code)
	}
	events := lazyEvents(t, c, base)

	refs := &References{Servers: []string{"gopls"}, Edges: []*graph.Edge{
		{From: "s:a.go#A", To: graph.FileID("a.go"), Kind: graph.EdgeReference}}}
	if err := s.SetReferences(refs); err != nil {
		t.Fatal(err)
	}
	expectEvent(t, events, "references")
	code, body := get(t, c, base+"/api/references", nil)
	var doc struct {
		Edges   []struct{ From, To, Kind string } `json:"edges"`
		Servers []string                          `json:"servers"`
		Partial *bool                             `json:"partial"`
	}
	if code != 200 || json.Unmarshal([]byte(body), &doc) != nil || len(doc.Edges) != 1 ||
		doc.Edges[0].From != "s:a.go#A" || len(doc.Servers) != 1 || doc.Partial == nil {
		t.Errorf("references: %d %s", code, body)
	}

	s.SetReferences(nil)
	expectEvent(t, events, "references")
	if code, _ := get(t, c, base+"/api/references", nil); code != http.StatusNoContent {
		t.Errorf("without references: %d, want 204", code)
	}
}

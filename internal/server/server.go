// Package server exposes the analyzed graph and the embedded UI on a local HTTP port.
package server

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/config"
	"github.com/sarumaj/depphunter-cli/internal/editor"
	"github.com/sarumaj/depphunter-cli/internal/export"
	"github.com/sarumaj/depphunter-cli/internal/graph"
	"github.com/sarumaj/depphunter-cli/internal/history"
	"github.com/sarumaj/depphunter-cli/web"
)

const (
	cookieName    = "depphunter_token"
	maxServedFile = 4 << 20
	// requestHeader must accompany state-changing requests. Cross-site pages cannot
	// set it without a CORS preflight, which this server never grants.
	requestHeader = "X-Depphunter-Request"
	heartbeat     = 25 * time.Second
)

type Server struct {
	token  string
	root   string
	assets fs.FS
	editor string // command template; "" disables /api/open
	cfg    config.Config
	// allowedHosts is nil when listening on a non-loopback address.
	allowedHosts map[string]bool

	mu   sync.RWMutex
	snap *snapshot
	lazy map[string]*lazyData // "history", "references": computed after startup
	subs map[chan event]struct{}
	done chan struct{}
}

type event struct {
	name string
	data []byte
}

// lazyData is a dataset computed in the background after the map is served.
type lazyData struct {
	pending bool
	value   any // nil when unavailable
	gz      []byte
}

// snapshot is one immutable analysis result with its encodings.
type snapshot struct {
	g           *graph.Graph
	version     int
	json, gz    []byte
	fingerprint [32]byte
	files       map[string]int // path -> lines, also the allow-list for /api/file
}

func New(cfg config.Config, g *graph.Graph, assets fs.FS) (*Server, error) {
	tok := make([]byte, 24)
	if _, err := rand.Read(tok); err != nil {
		return nil, err
	}
	s := &Server{
		token: hex.EncodeToString(tok), root: cfg.Root, assets: assets, editor: cfg.Editor, cfg: cfg,
		subs: map[chan event]struct{}{}, done: make(chan struct{}),
		lazy: map[string]*lazyData{
			"history":    {pending: cfg.History},
			"references": {pending: cfg.LSP},
		},
	}
	var err error
	if s.snap, err = newSnapshot(g, 1); err != nil {
		return nil, err
	}
	return s, nil
}

func newSnapshot(g *graph.Graph, version int) (*snapshot, error) {
	sn := &snapshot{g: g, version: version, files: map[string]int{}}
	for _, n := range g.Nodes {
		if n.Kind == graph.KindFile {
			sn.files[n.Path] = n.LOC
		}
	}
	content, err := json.Marshal([]any{g.Nodes, g.Edges})
	if err != nil {
		return nil, err
	}
	sn.fingerprint = sha256.Sum256(content)
	if sn.json, err = json.Marshal(g); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	zw.Write(sn.json)
	if err := zw.Close(); err != nil {
		return nil, err
	}
	sn.gz = buf.Bytes()
	return sn, nil
}

// Update publishes a new analysis and notifies connected browsers. touched lists files
// whose contents were re-read; together with added, removed and resized files they are
// reported as changed. It returns false when the graph did not change.
func (s *Server) Update(g *graph.Graph, touched []string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sn, err := newSnapshot(g, s.snap.version+1)
	if err != nil {
		return false, err
	}
	if sn.fingerprint == s.snap.fingerprint {
		return false, nil
	}
	changed := map[string]bool{}
	for p, loc := range sn.files {
		if old, ok := s.snap.files[p]; !ok || old != loc {
			changed[p] = true
		}
	}
	for p := range s.snap.files {
		if _, ok := sn.files[p]; !ok {
			changed[p] = true
		}
	}
	for _, p := range touched {
		if _, ok := sn.files[p]; ok {
			changed[p] = true
		}
	}
	paths := make([]string, 0, len(changed))
	for p := range changed {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	msg, _ := json.Marshal(struct {
		Version int      `json:"version"`
		Changed []string `json:"changed"`
	}{sn.version, paths})

	s.snap = sn
	s.broadcast(event{"graph", msg})
	return true, nil
}

// broadcast sends ev to every event stream; callers hold s.mu.
func (s *Server) broadcast(ev event) {
	for ch := range s.subs {
		select {
		case ch <- ev:
		default: // a slow client still refetches the latest state on its next event
		}
	}
}

// SetHistory publishes the git history (nil: none available) and notifies browsers.
func (s *Server) SetHistory(h *history.History) error {
	if h == nil {
		return s.setLazy("history", nil)
	}
	return s.setLazy("history", h)
}

// SetReferences publishes symbol references (nil: none available).
func (s *Server) SetReferences(r *References) error {
	if r == nil {
		return s.setLazy("references", nil)
	}
	return s.setLazy("references", r)
}

// References is the /api/references document.
type References struct {
	Edges   []*graph.Edge `json:"edges"`
	Servers []string      `json:"servers"`
	Partial bool          `json:"partial"`
}

func (s *Server) setLazy(name string, v any) error {
	d := &lazyData{value: v}
	if v != nil {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		if err := json.NewEncoder(zw).Encode(v); err != nil {
			return err
		}
		if err := zw.Close(); err != nil {
			return err
		}
		d.gz = buf.Bytes()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lazy[name] = d
	s.broadcast(event{name, []byte(fmt.Sprintf(`{"available":%t}`, v != nil))})
	return nil
}

// Lazy returns a background dataset for exports; nil while pending or unavailable.
func (s *Server) Lazy(name string) any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if d := s.lazy[name]; d != nil {
		return d.value
	}
	return nil
}

// handleLazy serves a background dataset: 202 while it is computed, 204 without one.
func (s *Server) handleLazy(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.mu.RLock()
		d := *s.lazy[name]
		s.mu.RUnlock()
		w.Header().Set("Cache-Control", "no-store")
		switch {
		case d.pending:
			w.WriteHeader(http.StatusAccepted)
		case d.value == nil:
			w.WriteHeader(http.StatusNoContent)
		case !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip"):
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(d.value)
		default:
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Vary", "Accept-Encoding")
			w.Write(d.gz)
		}
	}
}

// Close ends event streams so an HTTP server shutdown does not wait for them.
func (s *Server) Close() { close(s.done) }

func (s *Server) current() *snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snap
}

// Listen binds the address and returns the listener and the URL to open (including the token).
func (s *Server) Listen(addr string) (net.Listener, string, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, "", err
	}
	tcp := ln.Addr().(*net.TCPAddr)
	port := tcp.Port
	host := tcp.IP.String()
	if tcp.IP.IsLoopback() {
		s.allowedHosts = map[string]bool{}
		for _, h := range []string{"127.0.0.1", "localhost", "::1"} {
			s.allowedHosts[net.JoinHostPort(h, strconv.Itoa(port))] = true
		}
	} else if tcp.IP.IsUnspecified() {
		host = "127.0.0.1"
	}
	return ln, "http://" + net.JoinHostPort(host, strconv.Itoa(port)) + "/?token=" + s.token, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/graph", s.handleGraph)
	mux.HandleFunc("GET /api/config", s.handleConfig)
	mux.HandleFunc("GET /api/file", s.handleFile)
	mux.HandleFunc("GET /api/events", s.handleEvents)
	mux.HandleFunc("GET /api/history", s.handleLazy("history"))
	mux.HandleFunc("GET /api/references", s.handleLazy("references"))
	mux.HandleFunc("GET /api/export", s.handleExport)
	mux.HandleFunc("POST /api/open", s.handleOpen)
	mux.HandleFunc("POST /api/settings", s.handleSettings)
	mux.Handle("GET /", http.FileServerFS(s.assets))
	return s.guard(mux)
}

func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A foreign Host header means a DNS-rebinding page is talking to us.
		if s.allowedHosts != nil && !s.allowedHosts[r.Host] {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		h := w.Header()
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; frame-ancestors 'none'")

		if t := r.URL.Query().Get("token"); t != "" && s.equalToken(t) {
			http.SetCookie(w, &http.Cookie{Name: cookieName, Value: s.token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
			u := *r.URL
			q := u.Query()
			q.Del("token")
			u.RawQuery = q.Encode()
			http.Redirect(w, r, u.String(), http.StatusSeeOther)
			return
		}
		if c, err := r.Cookie(cookieName); err != nil || !s.equalToken(c.Value) {
			http.Error(w, "unauthorized: open the URL printed by depphunter", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodGet && r.Header.Get(requestHeader) != "1" {
			http.Error(w, "missing "+requestHeader+" header", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) equalToken(t string) bool {
	return subtle.ConstantTimeCompare([]byte(t), []byte(s.token)) == 1
}

func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	sn := s.current()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Vary", "Accept-Encoding")
	w.Header().Set("X-Graph-Version", strconv.Itoa(sn.version))
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		w.Write(sn.gz)
		return
	}
	w.Write(sn.json)
}

func (s *Server) handleConfig(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		config.UI
		Editor     bool   `json:"editor"`
		Root       string `json:"root"`
		Watch      bool   `json:"watch"`
		ConfigFile string `json:"configFile"`
		LSP        bool   `json:"lsp"`
	}{cfg.UI, s.editor != "", cfg.Root, cfg.Watch, filepath.Base(cfg.ConfigFile), cfg.LSP})
}

// handleSettings saves the browser's view settings into the project config file.
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	var ui config.UI
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&ui); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := ui.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := config.SaveUI(s.cfg.ConfigFile, ui); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.cfg.UI = ui
	w.WriteHeader(http.StatusNoContent)
}

// handleFile serves the source of a file that is part of the graph, and nothing else.
func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	if _, ok := s.current().files[rel]; !ok {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(filepath.Join(s.root, filepath.FromSlash(rel)))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if st.Size() > maxServedFile {
		http.Error(w, "file too large to display", http.StatusRequestEntityTooLarge)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "", time.Time{}, f)
}

// handleEvents streams graph updates as Server-Sent Events.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	ch := make(chan event, 8)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	version := s.snap.version
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.subs, ch)
		s.mu.Unlock()
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprintf(w, "retry: 2000\nevent: hello\ndata: {\"version\":%d}\n\n", version)
	flusher.Flush()

	tick := time.NewTicker(heartbeat)
	defer tick.Stop()
	for {
		select {
		case ev := <-ch:
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.name, ev.data)
		case <-tick.C:
			fmt.Fprint(w, ": ping\n\n")
		case <-r.Context().Done():
			return
		case <-s.done:
			return
		}
		flusher.Flush()
	}
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format == "html" {
		sn := s.current()
		s.mu.RLock()
		ui := s.cfg.UI
		s.mu.RUnlock()
		var buf bytes.Buffer
		if err := web.WriteStatic(&buf, sn.g, ui, s.root, map[string]any{
			"history": s.Lazy("history"), "references": s.Lazy("references"),
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", sn.g.Root+".html"))
		w.Write(buf.Bytes())
		return
	}
	ct, ok := export.ContentTypes[format]
	if !ok {
		http.Error(w, "format must be one of "+strings.Join(export.Formats, ", "), http.StatusBadRequest)
		return
	}
	sn := s.current()
	g := sn.g
	if refs, ok := s.Lazy("references").(*References); ok && refs != nil {
		g = export.WithEdges(g, refs.Edges)
	}
	var buf bytes.Buffer
	if err := export.Write(&buf, g, format); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", sn.g.Root+export.Extensions[format]))
	w.Write(buf.Bytes())
}

// handleOpen opens a graph file in the configured editor: {"path": "...", "line": 12}.
func (s *Server) handleOpen(w http.ResponseWriter, r *http.Request) {
	if s.editor == "" {
		http.Error(w, "no editor configured (set --editor or DEPPHUNTER_EDITOR)", http.StatusNotImplemented)
		return
	}
	var req struct {
		Path string `json:"path"`
		Line int    `json:"line"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, ok := s.current().files[req.Path]; !ok {
		http.NotFound(w, r)
		return
	}
	cmd, err := editor.Command(s.editor, filepath.Join(s.root, filepath.FromSlash(req.Path)), req.Line)
	if err == nil {
		err = cmd.Start()
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	go cmd.Wait() // reap the editor launcher
	w.WriteHeader(http.StatusNoContent)
}

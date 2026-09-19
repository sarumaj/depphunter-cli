// Package server exposes the analysed graph and the embedded UI on a local HTTP port.
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
	ui     []byte // /api/config body
	// allowedHosts is nil when listening on a non-loopback address.
	allowedHosts map[string]bool

	mu   sync.RWMutex
	snap *snapshot
	subs map[chan []byte]struct{}
	done chan struct{}
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
		token: hex.EncodeToString(tok), root: cfg.Root, assets: assets, editor: cfg.Editor,
		subs: map[chan []byte]struct{}{}, done: make(chan struct{}),
	}
	var err error
	if s.ui, err = json.Marshal(struct {
		config.UI
		Editor bool   `json:"editor"`
		Root   string `json:"root"`
		Watch  bool   `json:"watch"`
	}{cfg.UI, cfg.Editor != "", cfg.Root, cfg.Watch}); err != nil {
		return nil, err
	}
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
	for ch := range s.subs {
		select {
		case ch <- msg:
		default: // a slow client still refetches the latest graph on its next event
		}
	}
	return true, nil
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
	mux.HandleFunc("GET /api/export", s.handleExport)
	mux.HandleFunc("POST /api/open", s.handleOpen)
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
	w.Header().Set("Content-Type", "application/json")
	w.Write(s.ui)
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
	ch := make(chan []byte, 8)
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
		case msg := <-ch:
			fmt.Fprintf(w, "event: graph\ndata: %s\n\n", msg)
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
	ct, ok := export.ContentTypes[format]
	if !ok {
		http.Error(w, "format must be one of "+strings.Join(export.Formats, ", "), http.StatusBadRequest)
		return
	}
	sn := s.current()
	var buf bytes.Buffer
	if err := export.Write(&buf, sn.g, format); err != nil {
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

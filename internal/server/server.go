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
	"github.com/sarumaj/depphunter-cli/internal/findings"
	"github.com/sarumaj/depphunter-cli/internal/graph"
	"github.com/sarumaj/depphunter-cli/internal/history"
	"github.com/sarumaj/depphunter-cli/internal/trace"
	"github.com/sarumaj/depphunter-cli/web"
)

const (
	cookieName    = "depphunter_token"
	maxServedFile = 4 << 20
	// requestHeader must accompany state-changing requests. Cross-site pages cannot
	// set it without a CORS preflight, which this server never grants.
	requestHeader = "X-Depphunter-Request"
	// tokenHeader carries the token in embed mode, where there is no cookie to carry
	// it: the page is shown inside another origin's frame, and a cookie set here is
	// a third-party cookie there, which browsers do not send back.
	tokenHeader = "X-Depphunter-Token"
	heartbeat   = 25 * time.Second
)

type Server struct {
	token  string
	root   string
	assets fs.FS
	editor string // command template; "" disables /api/open
	cfg    config.Config
	// allowedHosts is nil when listening on a non-loopback address.
	allowedHosts map[string]bool
	// embed lists the origins allowed to show the map in a frame of their own; empty
	// - the default - means nobody may.
	embed []string

	mu   sync.RWMutex
	snap *snapshot
	lazy map[string]*lazyData // "history", "references", "findings": computed after startup
	subs map[chan event]struct{}
	done chan struct{}
	// What the clients share while the map is open (session.go): the selected node
	// and the catch, so the page and the editor's side panel are one interface.
	selected string
	pack     []PackItem
	// seq counts the announcements made on the event stream; see event.seq.
	seq uint64
	// resolution is the account the last analysis gave of itself (internal/trace).
	// It is served rather than announced: nothing on the map is drawn from it, and
	// the editor asks for it when somebody opens the report.
	resolution *trace.Report

	closeOnce sync.Once
}

type event struct {
	name string
	data []byte
	// seq numbers every announcement this server has made, and goes out as the
	// stream's event id. A client that reconnects hands its last one back
	// (Last-Event-ID) and is told whether it is still current; without it a dropped
	// connection is indistinguishable from a quiet one, and whatever was announced
	// while it was down is simply lost.
	seq uint64
}

// lazyData is a dataset computed in the background after the map is served.
type lazyData struct {
	pending bool
	value   any // nil when unavailable
	gz      []byte
	// sum fingerprints the encoded value, so a re-read that produced the same answer
	// can be recognized and not announced again.
	sum [32]byte
}

// snapshot is one immutable analysis result with its encodings.
type snapshot struct {
	g           *graph.Graph
	version     int
	json, gz    []byte
	fingerprint [32]byte
	// etag is the fingerprint as an HTTP entity tag, quoted and ready to compare.
	etag  string
	files map[string]int // path -> lines, also the allow-list for /api/file
}

func New(cfg config.Config, g *graph.Graph, assets fs.FS) (*Server, error) {
	tok := make([]byte, 24)
	if _, err := rand.Read(tok); err != nil {
		return nil, err
	}
	s := &Server{
		token: hex.EncodeToString(tok), root: cfg.Root, assets: assets, editor: cfg.Editor, cfg: cfg,
		embed: cfg.Embed,
		subs:  map[chan event]struct{}{}, done: make(chan struct{}),
		lazy: map[string]*lazyData{
			"history":    {pending: cfg.History},
			"references": {pending: cfg.LSP},
			"findings":   {pending: cfg.FindingsEnabled()},
		},
	}
	var err error
	if s.snap, err = newSnapshot(g, 1); err != nil {
		return nil, err
	}
	return s, nil
}

// docHead is everything the served graph document says about itself, which is
// everything in graph.Graph but its two big lists. Those are encoded on their own and
// the document is built around them (see newSnapshot), so this has to keep saying
// what graph.Graph says: a field added there and forgotten here would simply stop
// being served. TestServedGraphMatchesTheDocument holds it to that.
type docHead struct {
	Root        string    `json:"root"`
	GeneratedAt time.Time `json:"generatedAt"`
}

func newSnapshot(g *graph.Graph, version int) (*snapshot, error) {
	sn := &snapshot{g: g, version: version, files: make(map[string]int, len(g.Nodes))}
	for _, n := range g.Nodes {
		if n.Kind == graph.KindFile {
			sn.files[n.Path] = n.LOC
		}
	}
	nodes, err := json.Marshal(g.Nodes)
	if err != nil {
		return nil, err
	}
	edges, err := json.Marshal(g.Edges)
	if err != nil {
		return nil, err
	}
	// What the map is of, and not when it was made: a re-analysis that produced the
	// same graph has to fingerprint the same, and generatedAt never would.
	sum := sha256.New()
	sum.Write(nodes)
	sum.Write(edges)
	sum.Sum(sn.fingerprint[:0])
	sn.etag = `"` + hex.EncodeToString(sn.fingerprint[:16]) + `"`

	// The nodes and the edges are written straight into the buffer and fingerprinted
	// from it. Encoding them a second time to fingerprint - or handing them back to
	// the encoder as raw messages, which revalidates and copies every byte - is most
	// of what a --watch update costs on a hundred-thousand-node repository.
	head, err := json.Marshal(docHead{g.Root, g.GeneratedAt})
	if err != nil {
		return nil, err
	}
	var doc bytes.Buffer
	doc.Grow(len(head) + len(nodes) + len(edges) + 32)
	doc.Write(head[:len(head)-1]) // everything but the closing brace
	doc.WriteString(`,"nodes":`)
	doc.Write(nodes)
	doc.WriteString(`,"edges":`)
	doc.Write(edges)
	doc.WriteByte('}')
	sn.json = doc.Bytes()
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
	s.broadcast(event{name: "graph", data: msg})
	return true, nil
}

// broadcast sends ev to every event stream; callers hold s.mu.
func (s *Server) broadcast(ev event) {
	s.seq++
	ev.seq = s.seq
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

// SetResolution publishes the report of the analysis now being served. --watch
// analyzes again on every change, and each re-analysis brings its own.
func (s *Server) SetResolution(r *trace.Report) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resolution = r
}

// SetFindings publishes what the scanners said (nil: nothing was found or asked).
func (s *Server) SetFindings(f *findings.Set) error {
	if f.Empty() {
		return s.setLazy("findings", nil)
	}
	return s.setLazy("findings", f)
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
		var raw bytes.Buffer
		if err := json.NewEncoder(&raw).Encode(v); err != nil {
			return err
		}
		d.sum = sha256.Sum256(raw.Bytes())
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		if _, err := zw.Write(raw.Bytes()); err != nil {
			return err
		}
		if err := zw.Close(); err != nil {
			return err
		}
		d.gz = buf.Bytes()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// A re-read that produced the same answer is not news. Saying so anyway has
	// every browser reload the dataset and redraw for nothing - and re-reads are
	// far more common than changes: with --watch, the scanner reports are re-read
	// after any re-analysis, because a linter complains about the text of a file
	// and fixing one changes neither its imports nor its size.
	if old := s.lazy[name]; old != nil && !old.pending && old.sum == d.sum && (old.value == nil) == (v == nil) {
		return nil
	}
	s.lazy[name] = d
	s.broadcast(event{name: name, data: []byte(fmt.Sprintf(`{"available":%t}`, v != nil))})
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

// Close ends event streams so an HTTP server shutdown does not wait for them. It is
// safe to call more than once, so a shutdown path may close the server without
// checking whether the signal handler got there first.
func (s *Server) Close() { s.closeOnce.Do(func() { close(s.done) }) }

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
	mux.HandleFunc("GET /api/findings", s.handleLazy("findings"))
	mux.HandleFunc("GET /api/resolution", s.handleResolution)
	mux.HandleFunc("GET /api/export", s.handleExport)
	mux.HandleFunc("GET /api/session", s.handleSession)
	mux.HandleFunc("POST /api/selection", s.handleSelection)
	mux.HandleFunc("GET /api/backpack", s.handlePackExport)
	mux.HandleFunc("PUT /api/backpack", s.handlePack)
	mux.HandleFunc("POST /api/open", s.handleOpen)
	mux.HandleFunc("POST /api/settings", s.handleSettings)
	mux.Handle("GET /", newAssets(s.assets))
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
		ancestors := "'none'"
		if len(s.embed) > 0 {
			ancestors = strings.Join(s.embed, " ")
		} else {
			// Only where nothing may frame us at all. X-Frame-Options has no way to
			// name an origin that browsers still honour, so in embed mode the
			// Content-Security-Policy below is the whole of the answer.
			h.Set("X-Frame-Options", "DENY")
		}
		h.Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; frame-ancestors "+ancestors)

		if !s.allowed(w, r) {
			return
		}
		if r.Method != http.MethodGet && r.Header.Get(requestHeader) != "1" {
			http.Error(w, "missing "+requestHeader+" header", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// allowed decides whether a request carries the token, and answers the request
// itself when it does not: it writes to w only when one is refused or redirected.
func (s *Server) allowed(w http.ResponseWriter, r *http.Request) bool {
	if len(s.embed) > 0 {
		return s.embedAllowed(w, r)
	}
	// The ordinary way in: the token arrives once in the URL and is exchanged for a
	// cookie, so it leaves the address bar and never reaches a link or a log.
	if t := r.URL.Query().Get("token"); t != "" && s.equalToken(t) {
		http.SetCookie(w, &http.Cookie{Name: cookieName, Value: s.token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
		u := *r.URL
		q := u.Query()
		q.Del("token")
		u.RawQuery = q.Encode()
		http.Redirect(w, r, u.String(), http.StatusSeeOther)
		return false
	}
	if c, err := r.Cookie(cookieName); err != nil || !s.equalToken(c.Value) {
		http.Error(w, "unauthorized: open the URL printed by depphunter", http.StatusUnauthorized)
		return false
	}
	return true
}

// The same question inside somebody else's frame, where a cookie is no help: one set
// here is a third-party cookie there and is not sent back, so the token stays in the
// address the frame was given and the page hands it back on every call - in a header,
// or in the query string for the event stream, which cannot set headers.
//
// The interface's own files are served without it: they are the same bytes in every
// release and say nothing about the project. What the token guards is /api, where the
// repository's contents are, and the document that carries the token to the page.
func (s *Server) embedAllowed(w http.ResponseWriter, r *http.Request) bool {
	if t := r.Header.Get(tokenHeader); t != "" && s.equalToken(t) {
		return true
	}
	if t := r.URL.Query().Get("token"); t != "" && s.equalToken(t) {
		return true
	}
	if r.Method == http.MethodGet && r.URL.Path != "/" && !strings.HasPrefix(r.URL.Path, "/api/") {
		return true
	}
	http.Error(w, "unauthorized: open the URL printed by depphunter", http.StatusUnauthorized)
	return false
}

func (s *Server) equalToken(t string) bool {
	return subtle.ConstantTimeCompare([]byte(t), []byte(s.token)) == 1
}

func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	sn := s.current()
	w.Header().Set("Content-Type", "application/json")
	// Revalidate rather than refuse to store: the document is large and usually
	// unchanged, and an ETag is no use to a client that was told not to keep it.
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Vary", "Accept-Encoding")
	w.Header().Set("ETag", sn.etag)
	w.Header().Set("X-Graph-Version", strconv.Itoa(sn.version))
	// The fingerprint is of the nodes and the edges, not of when they were read, so
	// a re-analysis that found the same project answers 304 - which is what a client
	// reconnecting to a server that restarted under it is asking about.
	if matches(r.Header.Get("If-None-Match"), sn.etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		w.Write(sn.gz)
		return
	}
	w.Write(sn.json)
}

// matches reports whether an If-None-Match header names this entity. The header is a
// comma-separated list and may be "*"; a weak validator (W/"…") compares by its tag,
// which is all the comparison this needs - there is one representation per graph, and
// gzip is negotiated with Vary.
func matches(header, etag string) bool {
	for part := range strings.SplitSeq(header, ",") {
		part = strings.TrimSpace(part)
		if part == "*" || strings.TrimPrefix(part, "W/") == etag {
			return true
		}
	}
	return false
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
		Findings   bool   `json:"findings"`
	}{cfg.UI, s.editor != "", cfg.Root, cfg.Watch, filepath.Base(cfg.ConfigFile), cfg.LSP, cfg.FindingsEnabled()})
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
	version, etag, seq := s.snap.version, s.snap.etag, s.seq
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.subs, ch)
		s.mu.Unlock()
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	// The greeting says where the stream is, so a client knows whether it missed
	// anything while it was away. resumed is the point of the event ids: the browser
	// retries an EventSource by itself, and without one a client cannot tell a
	// reconnection from a first connection - so it either refetches everything or
	// carries on listening, never learning what it slept through.
	hello, _ := json.Marshal(struct {
		Version int    `json:"version"`
		ETag    string `json:"etag"`
		Seq     uint64 `json:"seq"`
		Resumed bool   `json:"resumed"`
	}{version, etag, seq, resumed(r, seq)})
	fmt.Fprintf(w, "retry: 2000\nevent: hello\ndata: %s\n\n", hello)
	flusher.Flush()

	tick := time.NewTicker(heartbeat)
	defer tick.Stop()
	for {
		select {
		case ev := <-ch:
			fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", ev.seq, ev.name, ev.data)
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

// resumed reports whether a reconnecting client is still up to date: it hands back
// the id of the last event it saw (Last-Event-ID, which EventSource sends by itself),
// and nothing has been announced since. A client that never saw one, or that is
// talking to a server restarted under it, is not resumed and refetches.
func resumed(r *http.Request, seq uint64) bool {
	last, err := strconv.ParseUint(strings.TrimSpace(r.Header.Get("Last-Event-ID")), 10, 64)
	return err == nil && last == seq
}

// handleResolution serves how the analysis reached its dependencies: JSON by default,
// the document the editor opens with ?format=md, and the command line's digest with
// ?format=text.
func (s *Server) handleResolution(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	rep := s.resolution
	s.mu.RUnlock()
	if rep == nil {
		http.Error(w, "no resolution report: this build served a graph it did not analyze", http.StatusNotFound)
		return
	}
	var buf bytes.Buffer
	var err error
	switch r.URL.Query().Get("format") {
	case "", "json":
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(&buf).Encode(rep)
	case "md":
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		err = rep.Markdown(&buf)
	case "text":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		err = rep.Text(&buf)
	default:
		http.Error(w, "format must be one of json, md, text", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format == "html" {
		sn := s.current()
		s.mu.RLock()
		ui := s.cfg.UI
		s.mu.RUnlock()
		// The browser sends the view it is showing, so the exported page opens like
		// the map on screen instead of like the config file.
		if q := r.URL.Query().Get("ui"); q != "" {
			var from config.UI
			if err := json.Unmarshal([]byte(q), &from); err != nil || from.Validate() != nil {
				http.Error(w, "invalid ui settings", http.StatusBadRequest)
				return
			}
			ui = from
		}
		var buf bytes.Buffer
		if err := web.WriteStatic(&buf, sn.g, ui, s.root, map[string]any{
			"history": s.Lazy("history"), "references": s.Lazy("references"), "findings": s.Lazy("findings"),
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

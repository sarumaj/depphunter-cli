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
	"io"
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
	// cookiePrefix starts the name of the cookie the token is exchanged for. The rest
	// of the name is particular to the server: browsers keep cookies per host, not
	// per port, so with one fixed name a second map opened on this machine would
	// overwrite the first one's cookie and every tab of the first would be refused.
	cookiePrefix  = "depphunter_"
	maxServedFile = 4 << 20
	// maxServedMedia is as large as a picture, a clip or a recording may be and still
	// be played in the panel (?as=raw). Larger than source, because such a file is not
	// read into the page at once: an <img>, a <video> or an <audio> asks for what it
	// needs, in ranges.
	maxServedMedia = 64 << 20
	// binaryHeader answers a request for a file's text when the file has none, and
	// says what it is instead (mediaType), so the panel can decide what to show.
	binaryHeader = "X-Depphunter-Binary"
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
	cookie string // the cookie's name; see cookiePrefix
	root   string
	assets fs.FS
	editor string // command template; "" disables /api/open
	config config.Config
	// allowedHosts is nil when listening on a non-loopback address.
	allowedHosts map[string]bool
	// embed lists the origins allowed to show the map in a frame of their own; empty
	// - the default - means nobody may.
	embed []string

	mu   sync.RWMutex
	snap *snapshot
	lazy map[dataset]*lazyData // computed after startup
	subs map[chan event]struct{}
	// hexers are the streams that said they can open a file in a hex editor
	// (/api/events?opens=hex) - the VS Code extension, which can ask the editor it
	// runs in for one where the launcher on the command line cannot.
	hexers map[chan event]struct{}
	done   chan struct{}
	// What the clients share while the map is open (session.go): the selected node
	// and the catch, so the page and the editor's side panel are one interface.
	selected string
	pack     []PackItem
	// sequence counts the announcements made on the event stream; see event.sequence.
	sequence uint64
	// resolution is the account the last analysis gave of itself (internal/trace).
	// It is served rather than announced: nothing on the map is drawn from it, and
	// the editor asks for it when somebody opens the report.
	resolution *trace.Report

	closeOnce sync.Once
}

type event struct {
	name string
	data []byte
	// sequence numbers every announcement this server has made, and goes out as the
	// stream's event id. A client that reconnects hands its last one back
	// (Last-Event-ID) and is told whether it is still current; without it a dropped
	// connection is indistinguishable from a quiet one, and whatever was announced
	// while it was down is simply lost.
	sequence uint64
}

// dataset names one of the datasets computed in the background after the map is
// served. The name is also the event that announces it and the last element of the
// path it is served at.
type dataset string

const (
	datasetHistory    dataset = "history"
	datasetReferences dataset = "references"
	datasetFindings   dataset = "findings"
)

// lazyData is a dataset computed in the background after the map is served.
type lazyData struct {
	pending bool
	value   any // nil when unavailable
	// body is the value encoded, as it is served; set only with a value.
	body encoded
	// sum fingerprints the encoded value, so a re-read that produced the same answer
	// can be recognized and not announced again.
	sum [32]byte
}

// snapshot is one immutable analysis result with its encodings.
type snapshot struct {
	g             *graph.Graph
	version       int
	json, gzipped []byte
	fingerprint   [32]byte
	// etag is the fingerprint as an HTTP entity tag, quoted and ready to compare.
	etag  string
	files map[string]int // path -> lines, also the allow-list for /api/file
}

// Implements: REQ-SEC-002
func New(settings config.Config, g *graph.Graph, assets fs.FS) (*Server, error) {
	token := make([]byte, 24)
	if _, err := rand.Read(token); err != nil {
		return nil, err
	}
	s := &Server{
		token: hex.EncodeToString(token), cookie: cookieFor(token), root: settings.Root, assets: assets, editor: settings.Editor, config: settings,
		embed: settings.Embed,
		subs:  map[chan event]struct{}{}, hexers: map[chan event]struct{}{}, done: make(chan struct{}),
		lazy: map[dataset]*lazyData{
			datasetHistory:    {pending: settings.History},
			datasetReferences: {pending: settings.LSP},
			datasetFindings:   {pending: settings.FindingsEnabled()},
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

// Implements: REQ-SRV-013
func newSnapshot(g *graph.Graph, version int) (*snapshot, error) {
	created := &snapshot{g: g, version: version, files: make(map[string]int, len(g.Nodes))}
	for n := range g.Of(graph.KindFile) {
		created.files[n.Path] = n.LOC
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
	sum.Sum(created.fingerprint[:0])
	created.etag = `"` + hex.EncodeToString(created.fingerprint[:16]) + `"`

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
	created.json = doc.Bytes()
	if created.gzipped, err = compress(created.json, gzip.DefaultCompression); err != nil {
		return nil, err
	}
	return created, nil
}

// Update publishes a new analysis and notifies connected browsers. touched lists files
// whose contents were re-read; together with added, removed and resized files they are
// reported as changed. It returns false when the graph did not change.
//
// Implements: REQ-WATCH-004
func (s *Server) Update(g *graph.Graph, touched []string) (bool, error) {
	// Encoded before the lock is taken: it is the expensive part of an update, and
	// every request reading the server's state would wait for it. The version is
	// assigned under the lock, once the snapshot is known to replace the current one.
	snapshot, err := newSnapshot(g, 0)
	if err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if snapshot.fingerprint == s.snap.fingerprint {
		return false, nil
	}
	snapshot.version = s.snap.version + 1
	changed := map[string]bool{}
	for p, lineCount := range snapshot.files {
		if old, ok := s.snap.files[p]; !ok || old != lineCount {
			changed[p] = true
		}
	}
	for p := range s.snap.files {
		if _, ok := snapshot.files[p]; !ok {
			changed[p] = true
		}
	}
	for _, p := range touched {
		if _, ok := snapshot.files[p]; ok {
			changed[p] = true
		}
	}
	paths := make([]string, 0, len(changed))
	for p := range changed {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	message, _ := json.Marshal(struct {
		Version int      `json:"version"`
		Changed []string `json:"changed"`
	}{snapshot.version, paths})

	s.snap = snapshot
	s.broadcast(event{name: "graph", data: message})
	return true, nil
}

// broadcast sends ev to every event stream; callers hold s.mu.
//
// Implements: REQ-SRV-015
func (s *Server) broadcast(sent event) {
	s.sequence++
	sent.sequence = s.sequence
	for channel := range s.subs {
		select {
		case channel <- sent:
		default: // a slow client still refetches the latest state on its next event
		}
	}
}

// SetHistory publishes the git history (nil: none available) and notifies browsers.
func (s *Server) SetHistory(h *history.History) error {
	return setPointer(s, datasetHistory, h)
}

// SetReferences publishes symbol references (nil: none available).
func (s *Server) SetReferences(r *References) error {
	return setPointer(s, datasetReferences, r)
}

// setPointer publishes a dataset held by a pointer. A nil pointer is no dataset: it
// is handed on as a nil interface, not as a typed nil, which would read as a value.
func setPointer[T any](s *Server, name dataset, v *T) error {
	if v == nil {
		return s.setLazy(name, nil)
	}
	return s.setLazy(name, v)
}

// SetResolution publishes the report of the analysis now being served. --watch
// analyzes again on every change, and each re-analysis brings its own.
//
// Implements: REQ-TRC-016
func (s *Server) SetResolution(r *trace.Report) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resolution = r
}

// SetFindings publishes what the scanners said (nil: nothing was found or asked).
func (s *Server) SetFindings(f *findings.Set) error {
	if f.Empty() {
		return s.setLazy(datasetFindings, nil)
	}
	return s.setLazy(datasetFindings, f)
}

// References is the /api/references document.
type References struct {
	Edges   []*graph.Edge `json:"edges"`
	Servers []string      `json:"servers"`
	Partial bool          `json:"partial"`
}

// Implements: REQ-HIST-008, REQ-LSP-005, REQ-FND-022
func (s *Server) setLazy(name dataset, v any) error {
	d := &lazyData{value: v}
	if v != nil {
		var raw bytes.Buffer
		if err := json.NewEncoder(&raw).Encode(v); err != nil {
			return err
		}
		d.sum = sha256.Sum256(raw.Bytes())
		gzipped, err := compress(raw.Bytes(), gzip.DefaultCompression)
		if err != nil {
			return err
		}
		// The history and the findings can be megabytes and rarely change between
		// page loads: revalidated like the graph, an unchanged one is a 304.
		d.body = encoded{plain: raw.Bytes(), gzipped: gzipped, etag: `"` + hex.EncodeToString(d.sum[:16]) + `"`, contentType: "application/json"}
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
	s.broadcast(event{name: string(name), data: []byte(fmt.Sprintf(`{"available":%t}`, v != nil))})
	return nil
}

// Lazy returns a background dataset for exports; nil while pending or unavailable.
func (s *Server) Lazy(name dataset) any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if d := s.lazy[name]; d != nil {
		return d.value
	}
	return nil
}

// handleLazy serves a background dataset: 202 while it is computed, 204 without one.
//
// Implements: REQ-HIST-007, REQ-LSP-005, REQ-FND-022
func (s *Server) handleLazy(name dataset) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.mu.RLock()
		d := *s.lazy[name]
		s.mu.RUnlock()
		switch {
		case d.pending:
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusAccepted)
		case d.value == nil:
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusNoContent)
		default:
			d.body.serve(w, r)
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
//
// Implements: REQ-SEC-002, REQ-SEC-005
func (s *Server) Listen(address string) (net.Listener, string, error) {
	line, err := net.Listen("tcp", address)
	if err != nil {
		return nil, "", err
	}
	tcp := line.Addr().(*net.TCPAddr)
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
	return line, "http://" + net.JoinHostPort(host, strconv.Itoa(port)) + "/?token=" + s.token, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/graph", s.handleGraph)
	mux.HandleFunc("GET /api/config", s.handleConfig)
	mux.HandleFunc("GET /api/file", s.handleFile)
	mux.HandleFunc("GET /api/events", s.handleEvents)
	mux.HandleFunc("GET /api/history", s.handleLazy(datasetHistory))
	mux.HandleFunc("GET /api/references", s.handleLazy(datasetReferences))
	mux.HandleFunc("GET /api/findings", s.handleLazy(datasetFindings))
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

// Implements: REQ-SEC-004, REQ-SEC-005, REQ-SEC-007, REQ-SEC-010
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
			// name an origin that browsers still honor, so in embed mode the
			// Content-Security-Policy below is the whole of the answer.
			h.Set("X-Frame-Options", "DENY")
		}
		// blob: is for the photographs walk mode's camera keeps (web/static/stash.js):
		// a picture is rendered off the canvas and held as a blob, and the panel that
		// shows the ones taken so far draws each as an <img> pointing at it.
		h.Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; frame-ancestors "+ancestors)

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
//
// Implements: REQ-SEC-003, REQ-SEC-004
func (s *Server) allowed(w http.ResponseWriter, r *http.Request) bool {
	if len(s.embed) > 0 {
		return s.embedAllowed(w, r)
	}
	// The ordinary way in: the token arrives once in the URL and is exchanged for a
	// cookie, so it leaves the address bar and never reaches a link or a log.
	if t := r.URL.Query().Get("token"); t != "" && s.equalToken(t) {
		http.SetCookie(w, &http.Cookie{Name: s.cookie, Value: s.token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
		u := *r.URL
		q := u.Query()
		q.Del("token")
		u.RawQuery = q.Encode()
		http.Redirect(w, r, u.String(), http.StatusSeeOther)
		return false
	}
	if c, err := r.Cookie(s.cookie); err != nil || !s.equalToken(c.Value) {
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

// cookieFor names the cookie of the server with this token. It is derived from the
// token rather than taken from it, so the name, which is not guarded as the value
// is, gives nothing of the token away.
//
// Implements: REQ-SEC-003
func cookieFor(token []byte) string {
	sum := sha256.Sum256(append([]byte("cookie:"), token...))
	return cookiePrefix + hex.EncodeToString(sum[:6])
}

func (s *Server) equalToken(t string) bool {
	return subtle.ConstantTimeCompare([]byte(t), []byte(s.token)) == 1
}

// Implements: REQ-SRV-002, REQ-SRV-013, REQ-SRV-014
func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	snapshot := s.current()
	w.Header().Set("X-Graph-Version", strconv.Itoa(snapshot.version))
	// Revalidate rather than refuse to store: the document is large and usually
	// unchanged, and an ETag is no use to a client that was told not to keep it. The
	// fingerprint is of the nodes and the edges, not of when they were read, so a
	// re-analysis that found the same project answers 304 - which is what a client
	// reconnecting to a server that restarted under it is asking about.
	body := encoded{plain: snapshot.json, gzipped: snapshot.gzipped, etag: snapshot.etag, contentType: "application/json"}
	body.serve(w, r)
}

// Implements: REQ-CFG-015, REQ-SRV-004
func (s *Server) handleConfig(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	settings := s.config
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
	}{settings.UI, s.editor != "", settings.Root, settings.Watch, filepath.Base(settings.ConfigFile), settings.LSP, settings.FindingsEnabled()})
}

// handleSettings saves the browser's view settings into the project config file.
//
// Implements: REQ-CFG-012, REQ-CFG-014, REQ-CFG-015
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	ui, ok := decodeBody[config.UI](w, r, 64<<10)
	if !ok {
		return
	}
	if err := ui.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	save := config.SaveUI
	if s.config.ConfigFile == filepath.Join(s.config.Root, config.ProjectFile) {
		save = config.SaveProjectUI // the repository's own file
	}
	if err := save(s.config.ConfigFile, ui); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.config.UI = ui
	w.WriteHeader(http.StatusNoContent)
}

// mediaTypes are the files the panel previews rather than lists: pictures, clips and
// recordings a browser plays, by extension. Nothing else is ever served under a type
// a browser would render - an SVG is text and is shown as source, and anything that
// is not on this list goes out as application/octet-stream.
var mediaTypes = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif",
	".webp": "image/webp", ".bmp": "image/bmp", ".ico": "image/x-icon", ".avif": "image/avif",
	".mp4": "video/mp4", ".m4v": "video/mp4", ".webm": "video/webm", ".ogv": "video/ogg",
	".mov": "video/quicktime",
	".mp3": "audio/mpeg", ".wav": "audio/wav", ".ogg": "audio/ogg", ".oga": "audio/ogg",
	".flac": "audio/flac", ".m4a": "audio/mp4",
}

// mediaType is the type a file's bytes are served under: its media type if it is one
// the panel previews, and application/octet-stream otherwise.
func mediaType(relative string) string {
	if t, ok := mediaTypes[strings.ToLower(filepath.Ext(relative))]; ok {
		return t
	}
	return "application/octet-stream"
}

// handleFile serves a file that is part of the graph, and nothing else: its source by
// default, or with ?as=raw its bytes, for the panel to preview a picture, a clip or a
// recording, or to show a binary file's first bytes when asked to.
//
// Asked for the source of a file that has none, it answers 415 with the file's type in
// X-Depphunter-Binary, rather than pouring its bytes into the page as text. Binary
// means what the scanner means by it (internal/scan): a NUL in the first 8000 bytes.
//
// The raw bytes of somebody's repository are served from the same origin as the map
// and its token, so they go out sandboxed: no script, no style, no frame, whatever
// the file claims to be. A browser only renders them where the page puts them - in an
// <img>, a <video> or an <audio> - and only as one of mediaTypes.
//
// Implements: REQ-SEC-006, REQ-SRV-003, REQ-MAP-062
func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	relative := r.URL.Query().Get("path")
	if _, ok := s.current().files[relative]; !ok {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(filepath.Join(s.root, filepath.FromSlash(relative)))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	fileInfo, err := f.Stat()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	raw := r.URL.Query().Get("as") == "raw"
	limit := int64(maxServedFile)
	if raw && mediaType(relative) != "application/octet-stream" {
		limit = maxServedMedia
	}
	if fileInfo.Size() > limit {
		http.Error(w, "file too large to display", http.StatusRequestEntityTooLarge)
		return
	}
	h := w.Header()
	h.Set("Cache-Control", "no-store")
	if raw {
		h.Set("Content-Type", mediaType(relative))
		h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
		http.ServeContent(w, r, "", time.Time{}, f)
		return
	}
	head := make([]byte, 8000)
	n, _ := io.ReadFull(f, head)
	if bytes.IndexByte(head[:n], 0) >= 0 {
		h.Set(binaryHeader, mediaType(relative))
		http.Error(w, "binary file", http.StatusUnsupportedMediaType)
		return
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.Set("Content-Type", "text/plain; charset=utf-8")
	http.ServeContent(w, r, "", time.Time{}, f)
}

// handToHexer announces a file to be opened in a hex editor to the streams that can,
// and reports whether any was listening. It is not an announcement to everybody, so
// it does not move the stream's position (seq): a page reconnecting afterwards has
// missed nothing it would have wanted.
func (s *Server) handToHexer(path string) bool {
	data, _ := json.Marshal(struct {
		Path string `json:"path"`
		Hex  bool   `json:"hex"`
	}{path, true})
	s.mu.Lock()
	defer s.mu.Unlock()
	sent := false
	for channel := range s.hexers {
		select {
		case channel <- event{name: "open", data: data, sequence: s.sequence}:
			sent = true
		default: // a stream too far behind to take it is not one to rely on
		}
	}
	return sent
}

// handleEvents streams graph updates as Server-Sent Events.
//
// Implements: REQ-WATCH-004, REQ-SRV-015, REQ-SRV-016
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	channel := make(chan event, 8)
	hexer := r.URL.Query().Get("opens") == "hex"
	s.mu.Lock()
	s.subs[channel] = struct{}{}
	if hexer {
		s.hexers[channel] = struct{}{}
	}
	version, etag, sequence := s.snap.version, s.snap.etag, s.sequence
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.subs, channel)
		delete(s.hexers, channel)
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
		Version  int    `json:"version"`
		ETag     string `json:"etag"`
		Sequence uint64 `json:"seq"`
		Resumed  bool   `json:"resumed"`
	}{version, etag, sequence, resumed(r, sequence)})
	fmt.Fprintf(w, "retry: 2000\nevent: hello\ndata: %s\n\n", hello)
	flusher.Flush()

	tick := time.NewTicker(heartbeat)
	defer tick.Stop()
	for {
		select {
		case event := <-channel:
			fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.sequence, event.name, event.data)
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
//
// Implements: REQ-SRV-016
func resumed(r *http.Request, sequence uint64) bool {
	last, err := strconv.ParseUint(strings.TrimSpace(r.Header.Get("Last-Event-ID")), 10, 64)
	return err == nil && last == sequence
}

// handleResolution serves how the analysis reached its dependencies: JSON by default,
// the document the editor opens with ?format=md, and the command line's digest with
// ?format=text.
//
// Implements: REQ-TRC-011, REQ-TRC-012, REQ-TRC-013
func (s *Server) handleResolution(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	report := s.resolution
	s.mu.RUnlock()
	if report == nil {
		http.Error(w, "no resolution report: this build served a graph it did not analyze", http.StatusNotFound)
		return
	}
	var buffer bytes.Buffer
	var err error
	switch r.URL.Query().Get("format") {
	case "", "json":
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(&buffer).Encode(report)
	case "md":
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		err = report.Markdown(&buffer)
	case "text":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		err = report.Text(&buffer)
	default:
		http.Error(w, "format must be one of json, md, text", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buffer.Bytes())
}

// Implements: REQ-EXP-005, REQ-EXP-009, REQ-HIST-015
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format == "html" {
		snapshot := s.current()
		s.mu.RLock()
		ui := s.config.UI
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
		var buffer bytes.Buffer
		if err := web.WriteStatic(&buffer, snapshot.g, ui, s.root, map[string]any{
			"history": s.Lazy(datasetHistory), "references": s.Lazy(datasetReferences), "findings": s.Lazy(datasetFindings),
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", snapshot.g.Root+".html"))
		w.Write(buffer.Bytes())
		return
	}
	ct, ok := export.ContentTypes[format]
	if !ok {
		http.Error(w, "format must be one of "+strings.Join(export.Formats, ", "), http.StatusBadRequest)
		return
	}
	snapshot := s.current()
	g := snapshot.g
	if references, ok := s.Lazy(datasetReferences).(*References); ok && references != nil {
		g = export.WithEdges(g, references.Edges)
	}
	var buffer bytes.Buffer
	if err := export.Write(&buffer, g, format); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", snapshot.g.Root+export.Extensions[format]))
	w.Write(buffer.Bytes())
}

// openedHeader says how a file asked for in a hex editor was opened after all:
// "as-is" when nothing that can open one was listening, and the editor got it as
// any other file.
const openedHeader = "X-Depphunter-Opened"

// handleOpen opens a graph file in the configured editor: {"path": "...", "line": 12}.
//
// With "hex": true it is a binary file wanted in a hex editor. A launcher on the
// command line has no way to ask for one, but the VS Code extension can, so if a
// stream that said it opens files that way (?opens=hex) is listening, the file is
// handed to it as an "open" event - to it alone - and the answer is 202. Otherwise
// the editor opens it as it would any file, and X-Depphunter-Opened says so.
//
// Implements: REQ-SEC-008, REQ-SRV-005, REQ-EXT-034
func (s *Server) handleOpen(w http.ResponseWriter, r *http.Request) {
	type openRequest struct {
		Path string `json:"path"`
		Line int    `json:"line"`
		Hex  bool   `json:"hex"`
	}
	request, ok := decodeBody[openRequest](w, r, 4096)
	if !ok {
		return
	}
	if _, ok := s.current().files[request.Path]; !ok {
		http.NotFound(w, r)
		return
	}
	if request.Hex && s.handToHexer(request.Path) {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if s.editor == "" {
		http.Error(w, "no editor configured (set --editor or DEPPHUNTER_EDITOR)", http.StatusNotImplemented)
		return
	}
	if request.Hex {
		w.Header().Set(openedHeader, "as-is")
	}
	command, err := editor.Command(s.editor, filepath.Join(s.root, filepath.FromSlash(request.Path)), request.Line)
	if err == nil {
		err = command.Start()
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	go command.Wait() // reap the editor launcher
	w.WriteHeader(http.StatusNoContent)
}

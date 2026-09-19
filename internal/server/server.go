// Package server exposes the analysed graph and the embedded UI on a local HTTP port.
package server

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/config"
	"github.com/sarumaj/depphunter-cli/internal/graph"
)

const (
	cookieName    = "depphunter_token"
	maxServedFile = 4 << 20
)

type Server struct {
	token     string
	root      string
	files     map[string]bool // relative paths that may be served through /api/file
	graphJSON []byte
	graphGz   []byte
	uiConfig  []byte
	assets    fs.FS
	// allowedHosts is nil when listening on a non-loopback address.
	allowedHosts map[string]bool
}

func New(cfg config.Config, g *graph.Graph, assets fs.FS) (*Server, error) {
	tok := make([]byte, 24)
	if _, err := rand.Read(tok); err != nil {
		return nil, err
	}
	s := &Server{token: hex.EncodeToString(tok), root: cfg.Root, files: map[string]bool{}, assets: assets}
	for _, n := range g.Nodes {
		if n.Kind == graph.KindFile {
			s.files[n.Path] = true
		}
	}

	var err error
	if s.graphJSON, err = json.Marshal(g); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	zw.Write(s.graphJSON)
	if err := zw.Close(); err != nil {
		return nil, err
	}
	s.graphGz = buf.Bytes()

	if s.uiConfig, err = json.Marshal(cfg.UI); err != nil {
		return nil, err
	}
	return s, nil
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
		next.ServeHTTP(w, r)
	})
}

func (s *Server) equalToken(t string) bool {
	return subtle.ConstantTimeCompare([]byte(t), []byte(s.token)) == 1
}

func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Vary", "Accept-Encoding")
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		w.Write(s.graphGz)
		return
	}
	w.Write(s.graphJSON)
}

func (s *Server) handleConfig(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(s.uiConfig)
}

// handleFile serves the source of a file that is part of the graph, and nothing else.
func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	if !s.files[rel] {
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

package server

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/sarumaj/depphunter-cli/internal/minify"
)

// assets serves the UI's own files: the modules, the stylesheet, the page and the
// models. It does three things http.FileServerFS does not.
//
// It minifies. The comments in this project are its documentation and there are a lot
// of them; they belong in the repository, not on the wire. The files on disk are left
// alone and what goes out has its comments and indentation removed, with every line
// break kept, so a stack trace in the browser still points at the line it came from.
//
// It compresses. The UI is about 1.4 MB of text and gzip takes roughly three quarters
// of that off. Each file is compressed once, the first time it is asked for, and the
// result is kept: a reload costs nothing, and a browser that does not want gzip gets
// the plain bytes.
//
// And it lets the browser skip all of it. Every file carries an ETag over what is
// actually served, so a reload is a 304 and no bytes at all.
type assets struct {
	fsys  fs.FS
	mu    sync.RWMutex
	cache map[string]*asset
}

type asset struct {
	body  []byte // as served: minified, where that means anything
	gz    []byte // ... and compressed, or nil when compression did not pay
	etag  string
	ctype string
}

func newAssets(fsys fs.FS) *assets {
	return &assets{fsys: fsys, cache: map[string]*asset{}}
}

func (a *assets) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if name == "" || strings.HasSuffix(r.URL.Path, "/") {
		name = path.Join(name, "index.html")
	}
	f, err := a.load(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	h := w.Header()
	h.Set("Content-Type", f.ctype)
	h.Set("ETag", f.etag)
	// The files change whenever the binary does, so the browser keeps them and asks
	// each time whether they are still current; the answer is one header long.
	h.Set("Cache-Control", "no-cache")
	h.Set("Vary", "Accept-Encoding")
	if match := r.Header.Get("If-None-Match"); match != "" && etagMatch(match, f.etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	body := f.body
	if f.gz != nil && strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		h.Set("Content-Encoding", "gzip")
		body = f.gz
	}
	h.Set("Content-Length", strconv.Itoa(len(body)))
	w.Write(body) // a HEAD request drops it again, which is the whole of HEAD here
}

// load reads, minifies and compresses a file the first time it is asked for.
func (a *assets) load(name string) (*asset, error) {
	a.mu.RLock()
	f := a.cache[name]
	a.mu.RUnlock()
	if f != nil {
		return f, nil
	}
	if !fs.ValidPath(name) {
		return nil, fs.ErrNotExist
	}
	raw, err := fs.ReadFile(a.fsys, name)
	if err != nil {
		return nil, err
	}
	f = &asset{body: raw, ctype: contentType(name)}
	switch path.Ext(name) {
	case ".js":
		f.body = []byte(minify.JS(string(raw)))
	case ".css":
		f.body = []byte(minify.CSS(string(raw)))
	case ".html":
		f.body = []byte(minify.HTML(string(raw)))
	}
	sum := sha256.Sum256(f.body)
	f.etag = `"` + base64.RawURLEncoding.EncodeToString(sum[:12]) + `"`
	if gz := compress(f.body); len(gz) < len(f.body)*9/10 {
		f.gz = gz
	}
	a.mu.Lock()
	a.cache[name] = f
	a.mu.Unlock()
	return f, nil
}

func compress(b []byte) []byte {
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil
	}
	if _, err := zw.Write(b); err != nil {
		return nil
	}
	if err := zw.Close(); err != nil {
		return nil
	}
	return buf.Bytes()
}

// contentType names a file's type. Go's table knows the web ones; the model format
// is new enough that it may not.
func contentType(name string) string {
	switch path.Ext(name) {
	case ".js":
		return "text/javascript; charset=utf-8"
	case ".glb":
		return "model/gltf-binary"
	}
	if t := mime.TypeByExtension(path.Ext(name)); t != "" {
		return t
	}
	return "application/octet-stream"
}

// etagMatch reports whether an If-None-Match header lists this tag. A proxy may have
// weakened it on the way, and the list may hold several.
func etagMatch(header, etag string) bool {
	for _, want := range strings.Split(header, ",") {
		want = strings.TrimSpace(want)
		if want == "*" || strings.TrimPrefix(want, "W/") == etag {
			return true
		}
	}
	return false
}

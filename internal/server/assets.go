package server

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"io/fs"
	"mime"
	"net/http"
	"path"
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
	fileSystem fs.FS
	mu         sync.RWMutex
	// cache holds each file as served: minified, where that means anything, and
	// compressed, where that takes at least a tenth off.
	cache map[string]*encoded
}

func newAssets(fileSystem fs.FS) *assets {
	return &assets{fileSystem: fileSystem, cache: map[string]*encoded{}}
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
	// The files change whenever the binary does, so the browser keeps them and asks
	// each time whether they are still current; the answer is one header long.
	f.serve(w, r)
}

// load reads, minifies and compresses a file the first time it is asked for.
func (a *assets) load(name string) (*encoded, error) {
	a.mu.RLock()
	f := a.cache[name]
	a.mu.RUnlock()
	if f != nil {
		return f, nil
	}
	if !fs.ValidPath(name) {
		return nil, fs.ErrNotExist
	}
	raw, err := fs.ReadFile(a.fileSystem, name)
	if err != nil {
		return nil, err
	}
	f = &encoded{plain: raw, contentType: contentType(name), contentLength: true}
	switch path.Ext(name) {
	case ".js":
		f.plain = []byte(minify.JS(string(raw)))
	case ".css":
		f.plain = []byte(minify.CSS(string(raw)))
	case ".html":
		f.plain = []byte(minify.HTML(string(raw)))
	}
	sum := sha256.Sum256(f.plain)
	f.etag = `"` + base64.RawURLEncoding.EncodeToString(sum[:12]) + `"`
	if gzipped, err := compress(f.plain, gzip.BestCompression); err == nil && len(gzipped) < len(f.plain)*9/10 {
		f.gzipped = gzipped
	}
	a.mu.Lock()
	a.cache[name] = f
	a.mu.Unlock()
	return f, nil
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

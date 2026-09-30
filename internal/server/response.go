package server

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// encoded is a response body kept ready to serve - the graph, a background dataset or
// one of the UI's files - so that serving it again costs a comparison and a write.
type encoded struct {
	plain []byte
	// gzipped is plain compressed, or nil when it is always served plain.
	gzipped []byte
	// etag is the entity tag of plain, quoted and ready to compare.
	etag        string
	contentType string
	// contentLength sends the length of the body served. Without it the length goes
	// out only when the body fits in the server's buffer, and a longer one is sent in
	// chunks.
	contentLength bool
}

// serve answers a request for the body. The client is told to keep it and revalidate
// (no-cache) and that the answer depends on Accept-Encoding; one that already holds
// this entity gets a 304 with no body, and one that accepts gzip gets the gzipped
// bytes where there are any. Headers set before the call go out with either answer.
func (e *encoded) serve(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Content-Type", e.contentType)
	h.Set("Cache-Control", "no-cache")
	h.Set("Vary", "Accept-Encoding")
	h.Set("ETag", e.etag)
	if etagMatch(r.Header.Get("If-None-Match"), e.etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	body := e.plain
	if e.gzipped != nil && strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		h.Set("Content-Encoding", "gzip")
		body = e.gzipped
	}
	if e.contentLength {
		h.Set("Content-Length", strconv.Itoa(len(body)))
	}
	w.Write(body) // a HEAD request drops it again, which is the whole of HEAD here
}

// etagMatch reports whether an If-None-Match header names this entity. The header is
// a comma-separated list and may be "*"; a weak validator (W/"…"), which a proxy may
// have made of the tag on the way, compares by its tag. That is all the comparison
// this needs: there is one representation per entity, and gzip is negotiated with
// Vary.
func etagMatch(header, etag string) bool {
	for part := range strings.SplitSeq(header, ",") {
		part = strings.TrimSpace(part)
		if part == "*" || strings.TrimPrefix(part, "W/") == etag {
			return true
		}
	}
	return false
}

// compress gzips data at a compression level of compress/gzip.
func compress(data []byte, level int) ([]byte, error) {
	var buffer bytes.Buffer
	gzipWriter, err := gzip.NewWriterLevel(&buffer, level)
	if err != nil {
		return nil, err
	}
	if _, err := gzipWriter.Write(data); err != nil {
		return nil, err
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// decodeBody reads a request's JSON body of at most limit bytes. When the body is
// not one, or is longer, it answers 400 itself and reports false.
func decodeBody[T any](w http.ResponseWriter, r *http.Request, limit int64) (T, bool) {
	var request T
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit)).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return request, false
	}
	return request, true
}

package lsp

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/graph"
)

// Cached returns References for g, reusing the result of an earlier run on the same
// content (the graph's nodes and edges) with the same installed servers. Complete
// results are stored in directory; "" disables caching.
//
// Implements: REQ-LSP-006
func Cached(ctx context.Context, directory string, g *graph.Graph, options Options) (*Result, error) {
	if options.LookPath == nil {
		options.LookPath = exec.LookPath
	}
	h := sha256.New()
	json.NewEncoder(h).Encode([]any{g.Nodes, g.Edges})
	for _, s := range Servers {
		if argv := s.command(options.LookPath); argv != nil {
			h.Write([]byte(strings.Join(argv, " ") + "\n"))
		}
	}
	root := sha256.Sum256([]byte(options.Root))
	prefix := filepath.Join(directory, hex.EncodeToString(root[:12])+"-references-")
	file := prefix + hex.EncodeToString(h.Sum(nil)[:12]) + ".json.gz"

	if directory != "" {
		if f, err := os.Open(file); err == nil {
			defer f.Close()
			if gzipReader, err := gzip.NewReader(f); err == nil {
				var r Result
				if json.NewDecoder(gzipReader).Decode(&r) == nil {
					return &r, nil
				}
			}
		}
	}
	r, err := References(ctx, g, options)
	if err != nil || r.Partial || directory == "" {
		return r, err
	}
	old, _ := filepath.Glob(prefix + "*.json.gz")
	for _, o := range old {
		os.Remove(o)
	}
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	if json.NewEncoder(gzipWriter).Encode(r) == nil && gzipWriter.Close() == nil && os.MkdirAll(directory, 0o755) == nil {
		if os.WriteFile(file+".tmp", buffer.Bytes(), 0o644) == nil {
			os.Rename(file+".tmp", file)
		}
	}
	return r, nil
}

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
// results are stored in dir; "" disables caching.
func Cached(ctx context.Context, dir string, g *graph.Graph, opts Options) (*Result, error) {
	if opts.LookPath == nil {
		opts.LookPath = exec.LookPath
	}
	h := sha256.New()
	json.NewEncoder(h).Encode([]any{g.Nodes, g.Edges})
	for _, s := range Servers {
		if argv := s.command(opts.LookPath); argv != nil {
			h.Write([]byte(strings.Join(argv, " ") + "\n"))
		}
	}
	root := sha256.Sum256([]byte(opts.Root))
	prefix := filepath.Join(dir, hex.EncodeToString(root[:12])+"-references-")
	file := prefix + hex.EncodeToString(h.Sum(nil)[:12]) + ".json.gz"

	if dir != "" {
		if f, err := os.Open(file); err == nil {
			defer f.Close()
			if zr, err := gzip.NewReader(f); err == nil {
				var r Result
				if json.NewDecoder(zr).Decode(&r) == nil {
					return &r, nil
				}
			}
		}
	}
	r, err := References(ctx, g, opts)
	if err != nil || r.Partial || dir == "" {
		return r, err
	}
	old, _ := filepath.Glob(prefix + "*.json.gz")
	for _, o := range old {
		os.Remove(o)
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if json.NewEncoder(zw).Encode(r) == nil && zw.Close() == nil && os.MkdirAll(dir, 0o755) == nil {
		if os.WriteFile(file+".tmp", buf.Bytes(), 0o644) == nil {
			os.Rename(file+".tmp", file)
		}
	}
	return r, nil
}

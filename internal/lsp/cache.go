package lsp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/graph"
	"github.com/sarumaj/depphunter-cli/internal/store"
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
	results, key := store.NewResult[Result](directory, options.Root, "references"), hex.EncodeToString(h.Sum(nil)[:12])
	if r, ok := results.Load(key); ok {
		return r, nil
	}
	r, err := References(ctx, g, options)
	if err != nil || r.Partial {
		return r, err
	}
	results.Save(key, r)
	return r, nil
}

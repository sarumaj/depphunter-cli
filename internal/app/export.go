package app

import (
	"context"
	"io"
	"os"

	"github.com/sarumaj/depphunter-cli/internal/export"
	"github.com/sarumaj/depphunter-cli/internal/graph"
	"github.com/sarumaj/depphunter-cli/web"
)

// export writes the graph in the format --export named. The HTML page carries what
// the served map would load in the background (history, references, findings);
// the other formats take the references as edges of the graph.
//
// Implements: REQ-EXP-004, REQ-EXP-010, REQ-HIST-015
func (a *app) export(ctx context.Context, g *graph.Graph) error {
	extra := map[string]any{}
	references := a.loadReferences(ctx, g)
	if a.settings.Export == "html" {
		if h := a.loadHistory(ctx, g); h != nil {
			extra["history"] = h
		}
		if references != nil {
			extra["references"] = references
		}
		if f := a.loadFindings(ctx, g); !f.Empty() {
			extra["findings"] = f
		}
	} else if references != nil {
		g = export.WithEdges(g, references.Edges)
	}
	return a.writeExport(g, extra)
}

// writeExport writes the export to --output, or to stdout without one.
//
// Implements: REQ-EXP-004
func (a *app) writeExport(g *graph.Graph, extra map[string]any) (err error) {
	var w io.Writer = os.Stdout
	if a.settings.Output != "" {
		f, err := os.Create(a.settings.Output)
		if err != nil {
			return err
		}
		// Close is where a full disk or a network share reports that the write did
		// not happen; ignoring it would leave a truncated export and exit 0.
		defer func() {
			if cErr := f.Close(); err == nil {
				err = cErr
			}
		}()
		w = f
	}
	if a.settings.Export == "html" {
		return web.WriteStatic(w, g, a.settings.UI, a.settings.Root, extra)
	}
	return export.Write(w, g, a.settings.Export)
}

package server

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// What the map's clients share while it is open: what is selected, and what has been
// caught.
//
// Neither is analysis - they are what somebody is doing with it - but they cannot
// live in one browser tab, because the editor's side panel lists the same
// dependencies and the same catch, and a selection one of the two knows nothing
// about is two interfaces rather than one. So the server holds them, says when they
// change, and every client is a view.
//
// Nothing here is durable: the backpack's own store is the browser's, per repository,
// since it has to outlive a server that runs only while somebody is looking. What the
// server keeps is this session's copy, pushed up as the page loads and kept in step
// afterwards.

// maxPack is what the browser's own backpack holds (backpack.js), and there is no
// reason for the relay to take more.
const maxPack = 500

// PackItem is one caught finding, as the page records it.
type PackItem struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Title    string `json:"title"`
	Where    string `json:"where"`
	Line     int    `json:"line,omitempty"`
	NodeID   string `json:"nodeId,omitempty"`
	CaughtAt int64  `json:"caughtAt,omitempty"` // milliseconds, as the page counts them
	Fixed    bool   `json:"fixed,omitempty"`
	FixedAt  int64  `json:"fixedAt,omitempty"`
}

// Session is what /api/session answers with: everything a client that has just
// attached needs in one round trip.
type Session struct {
	// Selected is the id of the selected graph node, empty for no selection.
	Selected string     `json:"selected"`
	Backpack []PackItem `json:"backpack"`
}

// handleSession serves the shared state whole.
//
// Implements: REQ-SRV-009
func (s *Server) handleSession(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	out := Session{Selected: s.selected, Backpack: append([]PackItem(nil), s.pack...)}
	s.mu.RUnlock()
	if out.Backpack == nil {
		out.Backpack = []PackItem{}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(out)
}

// handleSelection records what a client selected and tells the others.
//
// origin is the client that made the change, echoed back in the announcement: without
// it every client would apply its own selection a second time on the way back, and
// two of them watching each other would never settle.
//
// Implements: REQ-SRV-010, REQ-SRV-012
func (s *Server) handleSelection(w http.ResponseWriter, r *http.Request) {
	type selectionRequest struct {
		ID     string `json:"id"`
		Origin string `json:"origin"`
	}
	request, ok := decodeBody[selectionRequest](w, r, 8<<10)
	if !ok {
		return
	}
	s.mu.Lock()
	changed := s.selected != request.ID
	s.selected = request.ID
	if changed {
		s.broadcast(event{name: "selection", data: mustJSON(map[string]string{"id": request.ID, "origin": request.Origin})})
	}
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

// handlePack takes the browser's backpack and tells the others it moved. The page
// owns the list - it is the one with a store that survives the server - so this
// replaces rather than merges: a merge would resurrect what somebody just cleared.
//
// Implements: REQ-SRV-011, REQ-SRV-012
func (s *Server) handlePack(w http.ResponseWriter, r *http.Request) {
	type packRequest struct {
		Items  []PackItem `json:"items"`
		Origin string     `json:"origin"`
	}
	request, ok := decodeBody[packRequest](w, r, 1<<20)
	if !ok {
		return
	}
	items := request.Items
	for i := range items {
		if items[i].ID == "" {
			http.Error(w, "every backpack item needs an id", http.StatusBadRequest)
			return
		}
	}
	if len(items) > maxPack {
		items = items[:maxPack]
	}
	s.mu.Lock()
	changed := !samePack(s.pack, items)
	s.pack = items
	if changed {
		s.broadcast(event{name: "backpack", data: mustJSON(map[string]any{"count": len(items), "origin": request.Origin})})
	}
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

// handlePackExport writes the catch out for somewhere that is not this map: JSON to
// feed something else, CSV for a spreadsheet, Markdown to paste into the issue the
// whole exercise was for.
//
// Implements: REQ-EXP-014
func (s *Server) handlePackExport(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}
	i := slices.IndexFunc(packFormats, func(f packFormat) bool { return f.name == format })
	if i < 0 {
		names := make([]string, len(packFormats))
		for i, f := range packFormats {
			names[i] = f.name
		}
		http.Error(w, "format must be one of "+strings.Join(names, ", "), http.StatusBadRequest)
		return
	}
	s.mu.RLock()
	items := append([]PackItem(nil), s.pack...)
	name := s.snap.g.Root
	s.mu.RUnlock()
	// Worst first, and within a severity the most recently caught first: the order
	// somebody would put them in themselves.
	sort.SliceStable(items, func(i, j int) bool {
		if a, b := severityRank(items[i].Severity), severityRank(items[j].Severity); a != b {
			return a > b
		}
		return items[i].CaughtAt > items[j].CaughtAt
	})
	var buffer bytes.Buffer
	f := packFormats[i]
	f.write(&buffer, name, items)
	attachment(w, f.contentType, name+"-backpack"+f.extension, buffer.Bytes())
}

// A way to write the catch out (handlePackExport).
type packFormat struct {
	name, contentType, extension string
	write                        func(w io.Writer, name string, items []PackItem)
}

var packFormats = []packFormat{
	{"json", "application/json", ".json", func(w io.Writer, _ string, items []PackItem) {
		if items == nil {
			items = []PackItem{}
		}
		json.NewEncoder(w).Encode(items)
	}},
	{"csv", "text/csv; charset=utf-8", ".csv", func(w io.Writer, _ string, items []PackItem) {
		cw := csv.NewWriter(w)
		cw.Write([]string{"id", "severity", "title", "where", "line", "caught", "fixed"})
		for _, it := range items {
			cw.Write([]string{
				it.ID, it.Severity, it.Title, it.Where, lineOf(it), stamp(it.CaughtAt),
				map[bool]string{true: "yes", false: "no"}[it.Fixed],
			})
		}
		cw.Flush()
	}},
	{"md", "text/markdown; charset=utf-8", ".md", func(w io.Writer, name string, items []PackItem) {
		fmt.Fprintf(w, "# %s - caught findings\n\n", name)
		if len(items) == 0 {
			io.WriteString(w, "Nothing caught yet.\n")
		}
		for _, it := range items {
			where := it.Where
			if it.Line > 0 {
				where = fmt.Sprintf("%s:%d", where, it.Line)
			}
			done := " "
			if it.Fixed {
				done = "x"
			}
			fmt.Fprintf(w, "- [%s] **%s** %s", done, severityLabel(it.Severity), it.Title)
			if where != "" {
				fmt.Fprintf(w, " - `%s`", where)
			}
			fmt.Fprintf(w, " (%s)\n", it.ID)
		}
	}},
}

// samePack reports whether two lists hold the same catch in the same state. A client
// hands the whole list up whenever it redraws, and most of those are the same list;
// announcing them would have every other client refetch for nothing.
func samePack(a, b []PackItem) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func lineOf(it PackItem) string {
	if it.Line == 0 {
		return ""
	}
	return strconv.Itoa(it.Line)
}

func stamp(ms int64) string {
	if ms == 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}

// severityRank orders the severities the scanners use; anything unrecognized sorts
// with the least urgent, which is where an unknown belongs when the known ones are
// what the reader came for.
func severityRank(s string) int {
	switch strings.ToLower(s) {
	case "critical":
		return 5
	case "high":
		return 4
	case "medium", "moderate":
		return 3
	case "low":
		return 2
	case "info":
		return 1
	}
	return 0
}

func severityLabel(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

func mustJSON(v any) []byte {
	// The values here are maps of strings and ints, which cannot fail to encode.
	data, _ := json.Marshal(v)
	return data
}

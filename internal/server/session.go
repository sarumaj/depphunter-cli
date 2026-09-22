package server

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// What the map's clients share while it is open: what is selected, and what has been
// caught.
//
// Neither is analysis. They are what somebody is doing with it, and up to now they
// lived in the one browser tab that was doing it - which was enough while that tab
// was the whole interface. It is not any more: the editor's side panel lists the same
// dependencies and the same catch, and a selection made in one of the two that the
// other knows nothing about is two interfaces rather than one.
//
// So the server holds them and says when they change, and every client is a view.
// Nothing here is durable: the backpack's own store is still the browser's, per
// repository, because it has to outlive the server that is only running while
// somebody is looking. What the server keeps is this session's copy, pushed up as the
// page loads and kept in step afterwards, which is what the side panel reads.

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
func (s *Server) handleSelection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     string `json:"id"`
		Origin string `json:"origin"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	changed := s.selected != req.ID
	s.selected = req.ID
	if changed {
		s.broadcast(event{name: "selection", data: mustJSON(map[string]string{"id": req.ID, "origin": req.Origin})})
	}
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

// handlePack takes the browser's backpack and tells the others it moved. The page
// owns the list - it is the one with a store that survives the server - so this
// replaces rather than merges: a merge would resurrect what somebody just cleared.
func (s *Server) handlePack(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Items  []PackItem `json:"items"`
		Origin string     `json:"origin"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	items := req.Items
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
		s.broadcast(event{name: "backpack", data: mustJSON(map[string]any{"count": len(items), "origin": req.Origin})})
	}
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

// handlePackExport writes the catch out for somewhere that is not this map: JSON to
// feed something else, CSV for a spreadsheet, Markdown to paste into the issue the
// whole exercise was for.
func (s *Server) handlePackExport(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
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

	filename := func(ext string) {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name+"-backpack"+ext))
	}
	switch format {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		filename(".json")
		if items == nil {
			items = []PackItem{}
		}
		json.NewEncoder(w).Encode(items)
	case "csv":
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		filename(".csv")
		cw := csv.NewWriter(w)
		cw.Write([]string{"id", "severity", "title", "where", "line", "caught", "fixed"})
		for _, it := range items {
			cw.Write([]string{
				it.ID, it.Severity, it.Title, it.Where, lineOf(it), stamp(it.CaughtAt),
				map[bool]string{true: "yes", false: "no"}[it.Fixed],
			})
		}
		cw.Flush()
	case "md":
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		filename(".md")
		var b strings.Builder
		fmt.Fprintf(&b, "# %s - caught findings\n\n", name)
		if len(items) == 0 {
			b.WriteString("Nothing caught yet.\n")
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
			fmt.Fprintf(&b, "- [%s] **%s** %s", done, severityLabel(it.Severity), it.Title)
			if where != "" {
				fmt.Fprintf(&b, " - `%s`", where)
			}
			fmt.Fprintf(&b, " (%s)\n", it.ID)
		}
		w.Write([]byte(b.String()))
	default:
		http.Error(w, "format must be one of json, csv, md", http.StatusBadRequest)
	}
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

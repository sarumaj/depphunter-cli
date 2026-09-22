package trace

import (
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

// Text writes the report the way the command line shows it: columns, no markup, and
// the long lists cut off at maxListed with a count of what was left out. It is the
// digest. The whole of it - every question, in order - is in the JSON the server
// serves and in the document Markdown writes.
func (r *Report) Text(w io.Writer) error {
	if r == nil {
		return nil
	}
	out := &writer{w: w}
	out.printf("resolution report for %s (%s)\n", or(r.Root, "the analysis"), r.settings())
	for _, line := range r.preamble() {
		out.printf("  %s\n", line)
	}

	out.printf("\nindexes this run knew about\n")
	if len(r.Sources) == 0 {
		out.printf("  nothing on this machine or in the repository names one; every ecosystem falls back to its public index\n")
	} else {
		out.text(r.sourceTable())
	}
	if len(r.Indexes) > 0 {
		out.printf("\nwhat resolved from where\n")
		out.text(r.useTable())
	}
	if len(r.Levels) > 0 {
		out.printf("\nthe walk past what the code imports\n")
		out.text(r.levelTable())
	}
	if len(r.Skipped) > 0 {
		out.printf("\nnot walked at all\n")
		out.text(r.skipTable())
	}
	if r.Totals.Asked > 0 {
		out.printf("\nanswers\n  %s\n", r.answers())
	}
	if un := r.Unanswered(); len(un) > 0 {
		out.printf("\nnothing answered for these\n")
		out.text(r.reasonTable())
		out.printf("\n")
		out.text(lookupTable(un, maxListed, false))
	}
	if r.Dropped > 0 {
		out.printf("\n%d further questions were counted but not kept: the detail stops at %d.\n",
			r.Dropped, maxLookups)
	}
	return out.err
}

// Markdown writes the same report as a document, which is what the editor opens: the
// tables render, and the full list of questions is here rather than cut short,
// because a document is scrolled and searched rather than read past.
func (r *Report) Markdown(w io.Writer) error {
	if r == nil {
		return nil
	}
	out := &writer{w: w}
	out.printf("# Resolution report\n\n")
	out.printf("`%s` - %s", or(r.Root, "the analysis"), r.settings())
	if !r.GeneratedAt.IsZero() {
		out.printf(", %s", r.GeneratedAt.Format(time.RFC3339))
	}
	out.printf("\n")
	if lines := r.preamble(); len(lines) > 0 {
		out.printf("\n")
		for _, line := range lines {
			out.printf("- %s\n", line)
		}
	}

	out.printf("\n## Indexes this run knew about\n\n")
	if len(r.Sources) == 0 {
		out.printf("No index configuration was found; every ecosystem falls back to its public index.\n")
	} else {
		out.markdown(r.sourceTable())
	}

	out.printf("\n## What resolved from where\n\n")
	if len(r.Indexes) == 0 {
		out.printf("No external package on the map resolves from a named index.\n")
	} else {
		out.markdown(r.useTable())
	}

	out.printf("\n## The walk past what the code imports\n\n")
	switch {
	case !r.Walked():
		out.printf("Nothing was walked: `--resolve-depth` is %d.\n", r.ResolveDepth)
	case len(r.Levels) == 0:
		out.printf("No ecosystem could be walked.\n")
	default:
		out.markdown(r.levelTable())
	}
	if len(r.Skipped) > 0 {
		out.printf("\n### Not walked at all\n\n")
		out.markdown(r.skipTable())
	}

	if r.Totals.Asked > 0 {
		out.printf("\n## Answers\n\n%s\n", r.answers())
	}
	if un := r.Unanswered(); len(un) > 0 {
		out.printf("\n## Nothing answered for these\n\n")
		out.markdown(r.reasonTable())
		out.printf("\n")
		out.markdown(lookupTable(un, 0, false))
	}
	if len(r.Lookups) > 0 {
		out.printf("\n## Every question asked\n\n")
		out.markdown(lookupTable(r.Lookups, 0, true))
		if r.Dropped > 0 {
			out.printf("\n%d further questions were counted but not kept: the detail stops at %d.\n",
				r.Dropped, maxLookups)
		}
	}
	return out.err
}

// settings is what the run was asked to do, in one clause.
func (r *Report) settings() string {
	depth := fmt.Sprintf("--resolve-depth %d", r.ResolveDepth)
	switch r.ResolveDepth {
	case 0:
		depth = "no dependencies-of-dependencies"
	case -1:
		depth = "dependencies-of-dependencies as far as they reach"
	}
	if r.Online {
		return depth + ", --online"
	}
	return depth + ", offline"
}

// preamble is what the run was told about this organization, which is what decides
// the two questions depphunter declines to ask.
func (r *Report) preamble() []string {
	var out []string
	if len(r.Private) > 0 {
		out = append(out, "private packages: "+strings.Join(r.Private, ", "))
	}
	if len(r.TrustedIndexes) > 0 {
		out = append(out, "vouched indexes: "+strings.Join(r.TrustedIndexes, ", "))
	}
	return out
}

// answers is the one-line account of who answered the questions that were asked.
func (r *Report) answers() string {
	t := r.Totals
	parts := []string{fmt.Sprintf("%d asked", t.Asked)}
	if t.FromLock > 0 {
		parts = append(parts, fmt.Sprintf("%d from lock files", t.FromLock))
	}
	if from := t.FromIndex + t.FromCache + t.FromMemo; from > 0 {
		parts = append(parts, fmt.Sprintf("%d from indexes (%d fetched, %d cached, %d already asked)",
			from, t.FromIndex, t.FromCache, t.FromMemo))
	}
	switch {
	case t.Failed > 0:
		parts = append(parts, fmt.Sprintf("%d unanswered (%d of them asked and failed)", t.Unanswered, t.Failed))
	case t.Unanswered > 0:
		// Nothing failed, so nothing was even asked: every one of them was
		// answered by a silence decided before a request was made.
		parts = append(parts, fmt.Sprintf("%d unanswered, none of which was asked at all", t.Unanswered))
	}
	if t.Requests > 0 {
		parts = append(parts, plural(t.Requests, "request", "requests"))
	}
	if t.Packages > 0 {
		parts = append(parts, fmt.Sprintf("%s on the map (%d transitive, %d private, %d from an index nothing here vouches for)",
			plural(t.Packages, "external package", "external packages"), t.Transitive, t.PrivatePkg, t.Untrusted))
	}
	return strings.Join(parts, "; ")
}

func (r *Report) sourceTable() *table {
	t := &table{head: []string{"ecosystem", "index", "scope", "learned from", "fetched from"}}
	for _, s := range r.Sources {
		t.add(s.Ecosystem, s.URL, or(s.Scope, "-"), s.Origin, yes(s.Trusted))
	}
	return t
}

func (r *Report) useTable() *table {
	t := &table{head: []string{"ecosystem", "index", "packages", "private", "vouched for"}}
	for _, u := range r.Indexes {
		t.add(u.Ecosystem, u.Index, fmt.Sprint(u.Packages), fmt.Sprint(u.Private), yes(u.Trusted))
	}
	return t
}

func (r *Report) skipTable() *table {
	t := &table{head: []string{"plugin", "why"}}
	for _, s := range r.Skipped {
		t.add(s.Plugin, s.Reason)
	}
	return t
}

func (r *Report) reasonTable() *table {
	t := &table{head: []string{"count", "why"}}
	for _, reason := range r.Reasons() {
		t.add(fmt.Sprint(reason.Count), reason.Why)
	}
	return t
}

func (r *Report) levelTable() *table {
	t := &table{head: []string{"plugin", "level", "asked", "answered", "added", "edges", "time"}}
	for _, l := range r.Levels {
		t.add(l.Plugin, fmt.Sprint(l.Depth), fmt.Sprint(l.Asked), fmt.Sprint(l.Answered),
			fmt.Sprint(l.Added), fmt.Sprint(l.Edges), fmt.Sprintf("%dms", l.Millis))
	}
	return t
}

// lookupTable lays out questions. limit cuts the list short (0 keeps all), and full
// adds the columns that only matter when every question is listed.
func lookupTable(ls []Lookup, limit int, full bool) *table {
	head := []string{"level", "ecosystem", "package", "version", "answered by", "deps", "why / index"}
	if !full {
		head = []string{"level", "ecosystem", "package", "version", "why", "index"}
	}
	t := &table{head: head}
	shown := ls
	if limit > 0 && len(shown) > limit {
		shown = shown[:limit]
	}
	for _, l := range shown {
		if full {
			t.add(fmt.Sprint(l.Level), l.Ecosystem, l.Package, or(l.Version, "-"),
				string(l.Answer), fmt.Sprint(l.Deps), or(l.Reason, requestSummary(l)))
			continue
		}
		t.add(fmt.Sprint(l.Level), l.Ecosystem, l.Package, or(l.Version, "-"),
			or(l.Reason, requestSummary(l)), or(l.Index, "-"))
	}
	t.more = len(ls) - len(shown)
	return t
}

// requestSummary is what the round trips came to, for a question with no reason of
// its own to give: the status of the last one is usually the whole story.
func requestSummary(l Lookup) string {
	if len(l.Requests) == 0 {
		return "-"
	}
	last := l.Requests[len(l.Requests)-1]
	if len(l.Requests) == 1 {
		return last.Status
	}
	return fmt.Sprintf("%s (%d requests)", last.Status, len(l.Requests))
}

// table is a set of rows with a header, laid out either as aligned columns or as a
// Markdown table. Both renderings read the same rows, so the two reports cannot
// drift apart.
type table struct {
	head []string
	rows [][]string
	// more is how many rows were left out, for the line that says so.
	more int
}

func (t *table) add(cells ...string) { t.rows = append(t.rows, cells) }

// How wide one cell may get before the written report cuts it short. A registry
// redirect signs its URLs, and one such error message is several hundred characters
// of base64 that turns the whole table into a ragged wall. Nothing is lost: the JSON
// at /api/resolution carries every value whole.
const (
	textCell = 72
	mdCell   = 200
)

// clip returns the table with no cell wider than max.
func (t *table) clip(max int) *table {
	out := &table{head: t.head, more: t.more, rows: make([][]string, 0, len(t.rows))}
	for _, row := range t.rows {
		cells := make([]string, len(row))
		for i, cell := range row {
			cells[i] = clip(cell, max)
		}
		out.rows = append(out.rows, cells)
	}
	return out
}

// clip shortens one value to max characters, ending it with an ellipsis so that what
// is read is never mistaken for the whole of it.
func clip(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}

// widths is the width each column needs, counted in characters rather than bytes:
// a clipped cell ends in an ellipsis, which is three bytes and one column.
func (t *table) widths() []int {
	w := make([]int, len(t.head))
	for i, h := range t.head {
		w[i] = utf8.RuneCountInString(h)
	}
	for _, row := range t.rows {
		for i, cell := range row {
			if n := utf8.RuneCountInString(cell); i < len(w) && n > w[i] {
				w[i] = n
			}
		}
	}
	return w
}

// writer collects the first write error so the renderers can print without checking
// every line: the destination is a log or an HTTP response, and neither gets better
// halfway through.
type writer struct {
	w   io.Writer
	err error
}

func (o *writer) printf(format string, args ...any) {
	if o.err != nil {
		return
	}
	_, o.err = fmt.Fprintf(o.w, format, args...)
}

func (o *writer) text(t *table) {
	t = t.clip(textCell)
	w := t.widths()
	o.printf("  %s\n", strings.TrimRight(pad(upper(t.head), w), " "))
	for _, row := range t.rows {
		o.printf("  %s\n", strings.TrimRight(pad(row, w), " "))
	}
	if t.more > 0 {
		o.printf("  … and %d more\n", t.more)
	}
}

func (o *writer) markdown(t *table) {
	t = t.clip(mdCell)
	o.printf("| %s |\n", strings.Join(t.head, " | "))
	o.printf("|%s\n", strings.Repeat("---|", len(t.head)))
	for _, row := range t.rows {
		o.printf("| %s |\n", strings.Join(escape(row), " | "))
	}
	if t.more > 0 {
		o.printf("\n… and %d more\n", t.more)
	}
}

func pad(row []string, w []int) string {
	var b strings.Builder
	for i, cell := range row {
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(cell)
		if n := utf8.RuneCountInString(cell); i < len(w) && n < w[i] {
			b.WriteString(strings.Repeat(" ", w[i]-n))
		}
	}
	return b.String()
}

func upper(row []string) []string {
	out := make([]string, len(row))
	for i, cell := range row {
		out[i] = strings.ToUpper(cell)
	}
	return out
}

// escape keeps a cell inside its Markdown cell: a package name may hold a pipe, and
// one unescaped pipe shifts every column after it.
func escape(row []string) []string {
	out := make([]string, len(row))
	for i, cell := range row {
		out[i] = strings.ReplaceAll(cell, "|", `\|`)
	}
	return out
}

func or(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// plural writes a count with the word in the number the count calls for. A report
// that says "1 requests" reads as a report nobody proof-read.
func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

func yes(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

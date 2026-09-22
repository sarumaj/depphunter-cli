package findings

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// docs writes a small documented project and returns its root.
func docs(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for p, content := range files {
		abs := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// reasons maps each finding's location to the rule it broke, which is what the tests
// below are actually asserting.
func reasons(fs []*Finding) map[string]string {
	out := map[string]string{}
	for _, f := range fs {
		out[f.Path+":"+strconv.Itoa(f.Line)] = f.Ref
	}
	return out
}

func TestBrokenLinksAreFound(t *testing.T) {
	root := docs(t, map[string]string{
		"README.md": `# Home

Line three has [a file that is there](docs/SPEC.md) and [one that is not](docs/GONE.md).
Line four names [a heading that is there](docs/SPEC.md#how-it-works) and [one that is not](docs/SPEC.md#nope).
Line five jumps to [Home](#home) and to [nothing](#nothing).
Line six uses [a reference][missing] and [another][spec].

[spec]: docs/SPEC.md
`,
		"docs/SPEC.md": "# Spec\n\n## How it works\n",
	})
	found, partial := checkLinks(context.Background(), root,
		[]string{"README.md", "docs/SPEC.md"}, nil, func(string, ...any) {})
	if partial {
		t.Error("the check reported itself incomplete with every document readable")
	}
	want := map[string]string{
		"README.md:3": refMissingFile,
		"README.md:4": refMissingAnchor,
		"README.md:5": refMissingAnchor,
		"README.md:6": refUndefinedRef,
	}
	got := reasons(found)
	for where, ref := range want {
		if got[where] != ref {
			t.Errorf("%s: %q, want %q", where, got[where], ref)
		}
	}
	if len(found) != len(want) {
		for _, f := range found {
			t.Logf("  %s:%d:%d %s %s", f.Path, f.Line, f.Column, f.Ref, f.Title)
		}
		t.Errorf("%d findings, want %d", len(found), len(want))
	}
	// Each carries where it is and what was written, which is what makes it fixable
	// from the panel.
	for _, f := range found {
		if f.Kind != KindLink || f.Source != "links" || f.Column == 0 || f.Detail == "" {
			t.Errorf("incomplete finding: %+v", f)
		}
	}
}

// A link is broken when nothing is there, not when the target is merely absent from
// the map: a repository links to files it generates and files it ignores, and neither
// is a defect in the document.
func TestATargetOutsideTheScanIsNotBroken(t *testing.T) {
	root := docs(t, map[string]string{
		"README.md":        "# Home\n\nThe [report](dist/report.html) and the [log](dist/build.log).\n",
		"dist/report.html": "<!doctype html>",
	})
	found, _ := checkLinks(context.Background(), root, []string{"README.md"}, nil, func(string, ...any) {})
	if len(found) != 1 || !strings.Contains(found[0].Title, "dist/build.log") {
		t.Errorf("findings %v, want only the one that is not on disk", reasons(found))
	}
}

// Two broken links on one line are two findings: they are told apart by the column,
// without which the set would keep the first and drop the second as a repeat.
func TestTwoBreaksOnOneLine(t *testing.T) {
	root := docs(t, map[string]string{
		"README.md": "# Home\n\n| [a](a.md) | [b](b.md) |\n",
	})
	found, _ := checkLinks(context.Background(), root, []string{"README.md"}, nil, func(string, ...any) {})
	set := &Set{}
	set.Add("links", found)
	set.Finish()
	if len(set.Findings) != 2 {
		t.Errorf("%d findings kept, want 2: %+v", len(set.Findings), set.Findings)
	}
}

func TestADocumentThatCannotBeReadIsSaidSo(t *testing.T) {
	root := docs(t, map[string]string{"README.md": "# Home\n"})
	found, partial := checkLinks(context.Background(), root,
		[]string{"README.md", "gone/NOPE.md"}, nil, func(string, ...any) {})
	if len(found) != 0 || !partial {
		t.Errorf("findings %v, partial %v; want none and incomplete", found, partial)
	}
}

// The web checker is deliberately hard to convince: only a host saying the thing is
// gone is a finding. A refusal, a rate limit and a server error are the answers a
// link checker gets from hosts that block robots, and reading them as rot would
// report links that work perfectly well in a browser.
func TestOnlyAGoneAnswerIsAFinding(t *testing.T) {
	var asked atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked.Add(1)
		switch r.URL.Path {
		case "/ok":
			w.WriteHeader(http.StatusOK)
		case "/gone":
			w.WriteHeader(http.StatusNotFound)
		case "/retired":
			w.WriteHeader(http.StatusGone)
		case "/robot":
			w.WriteHeader(http.StatusForbidden)
		case "/busy":
			w.WriteHeader(http.StatusTooManyRequests)
		case "/broken":
			w.WriteHeader(http.StatusInternalServerError)
		case "/head-refused":
			// Plenty of hosts answer a HEAD with 405 and a GET with the page.
			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	body := "# Home\n\n"
	for _, p := range []string{"ok", "gone", "retired", "robot", "busy", "broken", "head-refused"} {
		body += "A [link](" + srv.URL + "/" + p + ").\n"
	}
	root := docs(t, map[string]string{"README.md": body})
	web := NewWeb(dir, time.Hour, 5*time.Second)
	web.http = srv.Client()

	found, _ := checkLinks(context.Background(), root, []string{"README.md"}, web, func(string, ...any) {})
	var gone []string
	for _, f := range found {
		gone = append(gone, strings.TrimPrefix(f.Title, srv.URL+"/"))
	}
	sort.Strings(gone)
	if strings.Join(gone, ",") != "gone answers 404,retired answers 410" {
		t.Errorf("reported %v, want only the two that are gone", gone)
	}

	// The answers worth keeping are kept, so a second run asks again only about the
	// ones that said nothing usable.
	before := asked.Load()
	again := NewWeb(dir, time.Hour, 5*time.Second)
	again.http = srv.Client()
	checkLinks(context.Background(), root, []string{"README.md"}, again, func(string, ...any) {})
	if asked.Load()-before >= before {
		t.Errorf("the second run asked %d times, the first %d: nothing was cached",
			asked.Load()-before, before)
	}
}

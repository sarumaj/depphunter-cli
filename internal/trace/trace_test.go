package trace

import (
	"strings"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/graph"
)

// sample is a report of a small run: one lock-file answer, one index answer, and the
// three ways a question comes back with nothing.
func sample() *Report {
	r := New(2, true, []string{"corp.example/*"}, []string{"https://nexus.corp/npm"})
	r.SetSources([]Source{
		// The origin strings are internal/index's; naming them here rather than
		// importing it keeps the dependency going one way.
		{Ecosystem: "npm", URL: "https://registry.npmjs.org", Origin: "public default", Trusted: true},
		{Ecosystem: "npm", URL: "https://evil.test/npm", Origin: "the repository"},
	})
	r.Enter("javascript", 0)
	r.Add(Lookup{Ecosystem: "npm", Package: "lodash", Version: "4.17.21", Answer: FromLock, Deps: 3})
	r.Add(Lookup{Ecosystem: "npm", Package: "chalk", Version: "5.3.0", Answer: FromIndex, Deps: 1,
		Index: "https://registry.npmjs.org", Requests: []Request{{URL: "https://registry.npmjs.org/chalk/5.3.0", Status: "200 OK"}}})
	r.Add(Lookup{Ecosystem: "npm", Package: "@acme/secret", Answer: NoAnswer, Reason: ReasonPrivate,
		Index: "https://registry.npmjs.org"})
	r.Add(Lookup{Ecosystem: "npm", Package: "confused", Answer: NoAnswer, Reason: ReasonUntrusted,
		Index: "https://evil.test/npm"})
	r.Add(Lookup{Ecosystem: "npm", Package: "gone", Answer: NoAnswer, Index: "https://registry.npmjs.org",
		Requests: []Request{{URL: "https://registry.npmjs.org/gone/1.0.0", Status: "404 Not Found"}}})
	r.Done(5, 2, 1, 1, 120*time.Millisecond)
	r.Skip("java", "the repository records no dependency graph for it")
	return r
}

func TestReportCountsWhoAnswered(t *testing.T) {
	r := sample()
	got := r.Totals
	for _, c := range []struct {
		name string
		got  int
		want int
	}{
		{"asked", got.Asked, 5},
		{"from lock files", got.FromLock, 1},
		{"from indexes", got.FromIndex, 1},
		{"unanswered", got.Unanswered, 3},
		// Two of the three were declined before anything was sent; only the 404 was
		// a question that was actually put and came back empty-handed.
		{"failed", got.Failed, 1},
		{"requests", got.Requests, 2},
	} {
		if c.got != c.want {
			t.Errorf("%s: %d, want %d", c.name, c.got, c.want)
		}
	}
	if len(r.Levels) != 1 || r.Levels[0].Plugin != "javascript" || r.Levels[0].Asked != 5 {
		t.Errorf("levels: %+v", r.Levels)
	}
	// The walk's level and plugin are stamped on by the report, not by the caller.
	for _, l := range r.Lookups {
		if l.Plugin != "javascript" || l.Level != 0 {
			t.Errorf("%s was recorded as %s level %d", l.Package, l.Plugin, l.Level)
		}
	}
}

func TestReasonsGroupTheUnanswered(t *testing.T) {
	reasons := sample().Reasons()
	if len(reasons) != 3 {
		t.Fatalf("reasons: %+v", reasons)
	}
	whys := map[string]int{}
	for _, r := range reasons {
		whys[r.Why] = r.Count
	}
	if whys[ReasonPrivate] != 1 || whys[ReasonUntrusted] != 1 {
		t.Errorf("reasons: %+v", whys)
	}
	// A question that failed has no reason of its own; the status it came back with
	// is the reason.
	if whys["404 Not Found"] != 1 {
		t.Errorf("the failed request is not reported by its status: %+v", whys)
	}
}

func TestSummarizeReadsTheMapForIndexes(t *testing.T) {
	g := &graph.Graph{Root: "app", Nodes: []*graph.Node{
		{ID: "e:npm", Kind: graph.KindEcosystem, Name: "npm"},
		{ID: "p:npm:lodash", Kind: graph.KindPackage, Parent: "e:npm", Index: "https://registry.npmjs.org"},
		{ID: "p:npm:@acme/x", Kind: graph.KindPackage, Parent: "e:npm", Index: "https://nexus.corp/npm", Private: true},
		{ID: "p:npm:confused", Kind: graph.KindPackage, Parent: "e:npm", Index: "https://evil.test/npm",
			IndexUnknown: true, Transitive: true},
		// A file is not a package, and a package with no index has nothing to say here.
		{ID: "f:main.js", Kind: graph.KindFile},
		{ID: "p:npm:local", Kind: graph.KindPackage, Parent: "e:npm"},
	}}
	r := New(0, false, nil, nil)
	r.Summarize(g)

	if r.Root != "app" {
		t.Errorf("root is %q", r.Root)
	}
	if len(r.Indexes) != 3 {
		t.Fatalf("indexes: %+v", r.Indexes)
	}
	// Sorted by ecosystem then index, so the same repository reports the same way twice.
	if r.Indexes[0].Index != "https://evil.test/npm" || r.Indexes[0].Trusted {
		t.Errorf("first index: %+v", r.Indexes[0])
	}
	if r.Indexes[1].Private != 1 {
		t.Errorf("the private package is not counted against its index: %+v", r.Indexes[1])
	}
	for _, c := range []struct {
		name string
		got  int
		want int
	}{
		{"packages", r.Totals.Packages, 3},
		{"transitive", r.Totals.Transitive, 1},
		{"private", r.Totals.PrivatePkg, 1},
		{"untrusted", r.Totals.Untrusted, 1},
	} {
		if c.got != c.want {
			t.Errorf("%s: %d, want %d", c.name, c.got, c.want)
		}
	}
}

func TestFinishOrdersTheQuestions(t *testing.T) {
	r := New(-1, false, nil, nil)
	r.Enter("go", 1)
	r.Add(Lookup{Ecosystem: "npm", Package: "b", Answer: FromLock})
	r.Add(Lookup{Ecosystem: "go", Package: "z", Answer: FromLock})
	r.Enter("go", 0)
	r.Add(Lookup{Ecosystem: "npm", Package: "a", Answer: FromLock})
	r.Finish()

	var got []string
	for _, l := range r.Lookups {
		got = append(got, l.Ecosystem+"/"+l.Package)
	}
	// Level first, then ecosystem, then name: answers come back in whatever order
	// twelve workers finish in, and the report must not.
	if want := "npm/a,go/z,npm/b"; strings.Join(got, ",") != want {
		t.Errorf("order is %v, want %s", got, want)
	}
	if r.GeneratedAt.IsZero() {
		t.Error("Finish left the report undated")
	}
}

func TestTextSaysWhatHappened(t *testing.T) {
	r := sample()
	r.Summarize(&graph.Graph{Root: "app"})
	r.Finish()
	var b strings.Builder
	if err := r.Text(&b); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{
		"--resolve-depth 2, --online",
		"corp.example/*",         // the patterns that decide what is not asked
		"https://nexus.corp/npm", // the index the user vouched for
		"https://evil.test/npm",  // the one only the repository names
		"javascript",             // the walk
		"1 from lock files",
		ReasonPrivate,
		"the repository records no dependency graph for it", // the ecosystem never walked
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the report does not mention %q:\n%s", want, out)
		}
	}
}

func TestMarkdownIsATableAndKeepsItsColumns(t *testing.T) {
	r := New(1, true, nil, nil)
	r.Enter("javascript", 0)
	// A package name with a pipe in it, and a message far too long for a cell.
	r.Add(Lookup{Ecosystem: "npm", Package: "a|b", Answer: NoAnswer, Reason: strings.Repeat("x", mdCell+50)})
	r.Done(1, 0, 0, 0, time.Millisecond)
	r.Finish()

	var b strings.Builder
	if err := r.Markdown(&b); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, `a\|b`) {
		t.Errorf("the pipe in a package name was not escaped:\n%s", out)
	}
	if strings.Contains(out, strings.Repeat("x", mdCell+1)) {
		t.Error("an over-long cell was not clipped")
	}
	if !strings.Contains(out, "# Resolution report") || !strings.Contains(out, "## Every question asked") {
		t.Errorf("the document is missing its sections:\n%s", out)
	}
}

// A report nobody asked for is not a special case anywhere: the analysis records
// into a nil report exactly as it records into a real one.
func TestNilReportRecordsNothing(t *testing.T) {
	var r *Report
	r.SetSources([]Source{{URL: "https://registry.npmjs.org"}})
	r.Enter("go", 0)
	r.Add(Lookup{Package: "x"})
	r.Done(1, 0, 0, 0, 0)
	r.Skip("go", "no")
	r.Summarize(&graph.Graph{})
	r.Finish()
	if r.Walked() || len(r.Unanswered()) != 0 || len(r.Reasons()) != 0 {
		t.Error("a nil report answered as though it had recorded something")
	}
	var b strings.Builder
	if err := r.Text(&b); err != nil || b.Len() != 0 {
		t.Errorf("a nil report wrote %q (%v)", b.String(), err)
	}
}

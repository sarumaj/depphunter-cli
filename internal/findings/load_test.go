package findings

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// copyReports puts the named fixtures into a fresh directory, under a reports/ folder,
// and returns the repository root.
func copyReports(t *testing.T, names ...string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "reports"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "reports", name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// Verifies: REQ-FND-001, REQ-FND-024
func TestCollectReadsEveryReportAndOrdersThem(t *testing.T) {
	root := copyReports(t, "govulncheck.json", "trivy.json", "eslint.json")
	set := Collect(context.Background(), Options{Root: root, Reports: []string{"reports/*.json"}})
	if set.Partial {
		t.Error("a readable set was reported as partial")
	}
	want := []string{"eslint", "govulncheck", "trivy"}
	if len(set.Sources) != len(want) {
		t.Fatalf("sources %v, want %v", set.Sources, want)
	}
	for i, s := range want {
		if set.Sources[i] != s {
			t.Fatalf("sources %v, want %v", set.Sources, want)
		}
	}
	// Most serious first, so the panel and the streets agree on what matters.
	for i := 1; i < len(set.Findings); i++ {
		if set.Findings[i-1].Severity.Rank() < set.Findings[i].Severity.Rank() {
			t.Fatalf("out of order at %d: %q before %q", i, set.Findings[i-1].Severity, set.Findings[i].Severity)
		}
	}
	ids := map[string]bool{}
	for _, f := range set.Findings {
		if f.ID == "" || ids[f.ID] {
			t.Fatalf("finding %q has no distinct id: %q", f.Reference, f.ID)
		}
		ids[f.ID] = true
	}
}

// A report path that points outside the repository still describes a package, so the
// finding is kept - it just no longer claims to be about a file on this map.
//
// Verifies: REQ-FND-017
func TestLocalizeKeepsOnlyPathsInsideTheRepository(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("path shapes differ")
	}
	root := t.TempDir()
	set := &Set{Findings: []*Finding{
		{Reference: "a", Path: filepath.Join(root, "web", "static", "app.js")},
		{Reference: "b", Path: "/etc/passwd", Package: "elsewhere"},
		{Reference: "c", Path: "internal/server/server.go"},
		{Reference: "d", Path: "../outside/thing.go"},
		{Reference: "e", Path: ""},
	}}
	set.Localize(root)
	want := map[string]string{"a": "web/static/app.js", "b": "", "c": "internal/server/server.go", "d": "", "e": ""}
	for _, f := range set.Findings {
		if f.Path != want[f.Reference] {
			t.Errorf("%s: path %q, want %q", f.Reference, f.Path, want[f.Reference])
		}
	}
}

// The same advisory read from two tools is one bug on the map.
func TestAddKeepsOneOfEachFinding(t *testing.T) {
	set := &Set{}
	f := func() []*Finding {
		return []*Finding{{Reference: "CVE-1", Ecosystem: "npm", Package: "lodash", Version: "4.17.15"}}
	}
	set.Add("trivy", f())
	set.Add("osv", f())
	if len(set.Findings) != 1 {
		t.Fatalf("%d findings, want 1", len(set.Findings))
	}
	if set.Findings[0].Source != "trivy" {
		t.Errorf("source %q, want the first report's", set.Findings[0].Source)
	}
	// Both tools ran, and the UI says so even though one of them found nothing new.
	if len(set.Sources) != 2 {
		t.Errorf("sources %v", set.Sources)
	}
}

// Verifies: REQ-FND-001, REQ-FND-016
func TestCollectSurvivesAReportItCannotRead(t *testing.T) {
	root := copyReports(t, "trivy.json")
	if err := os.WriteFile(filepath.Join(root, "reports", "broken.json"), []byte("{ not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	var logged int
	set := Collect(context.Background(), Options{
		Root: root, Reports: []string{"reports/*.json", "reports/missing.json"},
		Logf: func(string, ...any) { logged++ },
	})
	if !set.Partial {
		t.Error("a set missing a report was not reported as partial")
	}
	if set.Empty() {
		t.Error("one broken report lost the readable one")
	}
	if logged == 0 {
		t.Error("nothing was said about the report that would not parse")
	}
}

// A report the repository names is read inside it: one committed as a symbolic
// link out of the repository is not read - it could be any file of this
// machine - while one linked from inside it is, as is a report outside the
// repository that only this machine's configuration can name.
//
// Verifies: REQ-LANG-031, REQ-FND-001
func TestReportLinkedOutOfTheRepositoryIsNotRead(t *testing.T) {
	root := copyReports(t, "trivy.json")
	outside := filepath.Join(t.TempDir(), "trivy.json")
	data, err := os.ReadFile(filepath.Join("testdata", "trivy.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "reports", "out.json")); err != nil {
		t.Skipf("no symbolic links here: %v", err)
	}
	if err := os.Symlink("trivy.json", filepath.Join(root, "reports", "in.json")); err != nil {
		t.Fatal(err)
	}
	for _, testCase := range []struct {
		report string
		read   bool
	}{
		{"reports/trivy.json", true},
		{"reports/in.json", true},
		{"reports/out.json", false},
		{outside, true}, // named by this machine: read as it is
	} {
		set := Collect(context.Background(), Options{Root: root, Reports: []string{testCase.report}})
		if read := len(set.Findings) > 0; read != testCase.read || set.Partial == testCase.read {
			t.Errorf("%s: %d findings, partial %v, want read %v", testCase.report, len(set.Findings), set.Partial, testCase.read)
		}
	}
}

package findings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// read is a report from testdata, with the tool that wrote it.
func read(t *testing.T, name string) (string, []*Finding) {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	source, found, err := Read(f)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return source, found
}

// find is the one finding with this reference, or a failure naming what was there.
func find(t *testing.T, fs []*Finding, ref string) *Finding {
	t.Helper()
	var hit *Finding
	for _, f := range fs {
		if f.Ref == ref {
			if hit != nil {
				t.Fatalf("%s reported twice", ref)
			}
			hit = f
		}
	}
	if hit == nil {
		refs := make([]string, len(fs))
		for i, f := range fs {
			refs[i] = f.Ref
		}
		t.Fatalf("no finding %q; got %v", ref, refs)
	}
	return hit
}

// Every report shape is recognized from its own content: the file's name says nothing.
//
// Verifies: REQ-FND-003, REQ-FND-004, REQ-FND-005, REQ-FND-006, REQ-FND-007, REQ-FND-008, REQ-FND-009
func TestReadRecognizesEveryFormat(t *testing.T) {
	for name, want := range map[string]string{
		"govulncheck.json":   "govulncheck",
		"npm-audit.json":     "npm-audit",
		"npm-audit-v6.json":  "npm-audit",
		"trivy.json":         "trivy",
		"golangci-lint.json": "golangci-lint",
		"eslint.json":        "eslint",
		"osv-scanner.json":   "osv-scanner",
	} {
		if source, found := read(t, name); source != want {
			t.Errorf("%s: read as %q, want %q", name, source, want)
		} else if len(found) == 0 {
			t.Errorf("%s: no findings", name)
		}
	}
}

// Verifies: REQ-FND-003, REQ-FND-020
func TestGovulncheckKeepsTheCallSite(t *testing.T) {
	_, found := read(t, "govulncheck.json")
	reached := find(t, found, "GO-2024-2687")
	// Not the vulnerable function's own position, which is a path inside x/net: the
	// frame nearest it that is in this repository's module.
	if reached.Path != "internal/server/server.go" || reached.Line != 42 || reached.Column != 7 {
		t.Errorf("call site: %s:%d:%d", reached.Path, reached.Line, reached.Column)
	}
	if !strings.Contains(reached.Detail, "Reached from Server.serve.") {
		t.Errorf("detail does not name the caller: %q", reached.Detail)
	}
	if reached.Package != "golang.org/x/net" || reached.Version != "0.17.0" || reached.Fixed != "0.23.0" {
		t.Errorf("package: %s@%s fixed in %s", reached.Package, reached.Version, reached.Fixed)
	}
	// AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:H scores 7.5.
	if reached.Severity != High {
		t.Errorf("severity %q, want high", reached.Severity)
	}
	if want := "Also known as CVE-2023-45288"; !strings.Contains(reached.Detail, want) {
		t.Errorf("detail lost the aliases: %q", reached.Detail)
	}

	// An advisory nothing calls into is still reported, but it is not the same news.
	quiet := find(t, found, "GO-2023-9999")
	if quiet.Path != "" {
		t.Errorf("unreachable finding has a call site: %q", quiet.Path)
	}
	if quiet.Severity != Low {
		t.Errorf("unreachable severity %q, want low", quiet.Severity)
	}
	if !strings.Contains(quiet.Detail, "No call into the vulnerable code") {
		t.Errorf("detail does not say it is unreachable: %q", quiet.Detail)
	}
}

// Verifies: REQ-FND-004
func TestNPMAuditReadsAdvisoriesAndChains(t *testing.T) {
	_, found := read(t, "npm-audit.json")
	direct := find(t, found, "GHSA-p6mc-m468-83gg")
	if direct.Package != "lodash" || direct.Severity != High || direct.Fixed != "4.17.21" {
		t.Errorf("lodash: %+v", direct)
	}
	// build-tool is not itself vulnerable: it depends on something that is.
	chain := find(t, found, "npm:build-tool")
	if chain.Package != "build-tool" || !strings.Contains(chain.Detail, "Through lodash") {
		t.Errorf("chain: %+v", chain)
	}
}

// Verifies: REQ-FND-004
func TestNPMAuditV6PrefersTheCVE(t *testing.T) {
	_, found := read(t, "npm-audit-v6.json")
	f := find(t, found, "CVE-2021-44906")
	if f.Package != "minimist" || f.Version != "1.2.0" || f.Fixed != "1.2.6" || f.Severity != Critical {
		t.Errorf("minimist: %+v", f)
	}
}

// Trivy's own severity word is a distribution's rating; the advisory's CVSS vector is
// what the map shows.
//
// Verifies: REQ-FND-005, REQ-FND-018
func TestTrivyPrefersTheCVSSVector(t *testing.T) {
	_, found := read(t, "trivy.json")
	vuln := find(t, found, "CVE-2020-8203")
	if vuln.Severity != Critical {
		t.Errorf("severity %q, want critical (9.8 from the vector, not the MEDIUM word)", vuln.Severity)
	}
	if vuln.Ecosystem != "npm" || vuln.Package != "lodash" || vuln.Path != "package-lock.json" {
		t.Errorf("placement: %+v", vuln)
	}
	misconfig := find(t, found, "DS002")
	if misconfig.Kind != KindLint || misconfig.Path != "Dockerfile" || misconfig.Line != 1 {
		t.Errorf("misconfiguration: %+v", misconfig)
	}
}

// A linter's "error" is not a critical advisory: lint findings stay below vulnerabilities.
//
// Verifies: REQ-FND-019
func TestLintSeveritiesStayBelowVulnerabilities(t *testing.T) {
	_, golangci := read(t, "golangci-lint.json")
	errcheck := find(t, golangci, "errcheck")
	if errcheck.Severity != Medium || errcheck.Kind != KindLint {
		t.Errorf("errcheck: %+v", errcheck)
	}
	if errcheck.Path != "internal/server/server.go" || errcheck.Line != 112 || errcheck.Column != 4 {
		t.Errorf("position: %+v", errcheck)
	}
	if unused := find(t, golangci, "unused"); unused.Severity != Medium {
		t.Errorf("a linter that gives no severity defaults to %q", unused.Severity)
	}

	_, eslint := read(t, "eslint.json")
	if e := find(t, eslint, "no-unused-vars"); e.Severity != Medium {
		t.Errorf("eslint error: %q", e.Severity)
	}
	if e := find(t, eslint, "eqeqeq"); e.Severity != Low {
		t.Errorf("eslint warning: %q", e.Severity)
	}
	if len(eslint) != 2 {
		t.Errorf("a file with no messages produced findings: %d", len(eslint))
	}
}

// Verifies: REQ-FND-006, REQ-FND-020
func TestOSVScannerPlacesFindingsOnPackages(t *testing.T) {
	_, found := read(t, "osv-scanner.json")
	f := find(t, found, "GHSA-1234-5678-90ab")
	if f.Ecosystem != "go" || f.Package != "github.com/example/lib" || f.Version != "1.0.0" {
		t.Errorf("placement: %+v", f)
	}
	if f.Fixed != "1.1.0" {
		t.Errorf("fixed in %q", f.Fixed)
	}
	// AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:L scores 5.3.
	if f.Severity != Medium {
		t.Errorf("severity %q, want medium", f.Severity)
	}
	if f.URL != "https://github.com/advisories/GHSA-1234-5678-90ab" {
		t.Errorf("url %q", f.URL)
	}
}

// Verifies: REQ-FND-009
func TestReadRejectsWhatIsNotAReport(t *testing.T) {
	for _, in := range []string{
		"", "   \n", "not json at all", `<?xml version="1.0"?><report/>`,
		`{"hello": 1}`,              // JSON, but no tool wrote it
		`{"coverage": {"pct": 91}}`, // a report of something else entirely
	} {
		if _, _, err := Read(strings.NewReader(in)); err == nil {
			t.Errorf("%q was read as a report", in)
		}
	}
}

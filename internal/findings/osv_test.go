package findings

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/store"
)

// osvServer answers like api.osv.dev for one known-vulnerable package, and counts what
// was asked of it.
func osvServer(t *testing.T) (*httptest.Server, *int32, *int32) {
	t.Helper()
	var batches, details int32
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/querybatch", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&batches, 1)
		var body struct {
			Queries []struct {
				Package struct{ Name, Ecosystem string } `json:"package"`
				Version string                           `json:"version"`
			} `json:"queries"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("batch body: %v", err)
		}
		out := struct {
			Results []batchResult `json:"results"`
		}{}
		for _, q := range body.Queries {
			var res batchResult
			if q.Package.Name == "golang.org/x/net" && q.Package.Ecosystem == "Go" && q.Version == "0.17.0" {
				res.Vulns = append(res.Vulns, struct {
					ID string `json:"id"`
				}{"GO-2024-2687"})
			}
			out.Results = append(out.Results, res)
		}
		json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("GET /v1/vulns/{id}", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&details, 1)
		json.NewEncoder(w).Encode(osvEntry{
			ID:      r.PathValue("id"),
			Summary: "HTTP/2 CONTINUATION flood",
			Details: "An attacker may cause an endpoint to read arbitrary amounts of header data.",
			Aliases: []string{"CVE-2023-45288"},
			Severity: []struct {
				Type  string `json:"type"`
				Score string `json:"score"`
			}{{Type: "CVSS_V3", Score: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:H"}},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &batches, &details
}

func TestOSVQueriesTheDatabaseAndCachesTheAnswer(t *testing.T) {
	srv, batches, details := osvServer(t)
	dir := t.TempDir()
	pkgs := []Package{
		{Ecosystem: "go", Name: "golang.org/x/net", Version: "v0.17.0"}, // the v is the database's to drop
		{Ecosystem: "go", Name: "golang.org/x/text", Version: "v0.14.0"},
		{Ecosystem: "psgallery", Name: "Pester", Version: "5.5.0"}, // no OSV counterpart
		{Ecosystem: "npm", Name: "left-pad", Version: ""},          // nothing to ask about
	}

	o := &OSV{http: srv.Client(), cache: store.New(dir, time.Hour), API: srv.URL}
	found, partial := o.Query(context.Background(), pkgs)
	if partial {
		t.Error("a complete answer was reported as partial")
	}
	if len(found) != 1 {
		t.Fatalf("%d findings, want 1", len(found))
	}
	f := found[0]
	if f.Ref != "GO-2024-2687" || f.Package != "golang.org/x/net" || f.Version != "0.17.0" {
		t.Errorf("finding: %+v", f)
	}
	if f.Severity != High || f.Source != "osv" || f.Kind != KindVulnerability {
		t.Errorf("finding: %+v", f)
	}
	if f.URL != "https://osv.dev/vulnerability/GO-2024-2687" {
		t.Errorf("url %q", f.URL)
	}

	// A second run with the same cache directory asks nothing at all - including about
	// the packages that turned out to be fine, which is most of them.
	b1, d1 := atomic.LoadInt32(batches), atomic.LoadInt32(details)
	again := &OSV{http: srv.Client(), cache: store.New(dir, time.Hour), API: srv.URL}
	if found, _ := again.Query(context.Background(), pkgs); len(found) != 1 {
		t.Fatalf("cached run: %d findings, want 1", len(found))
	}
	if b, d := atomic.LoadInt32(batches), atomic.LoadInt32(details); b != b1 || d != d1 {
		t.Errorf("cached run asked again: %d batches, %d details (was %d, %d)", b, d, b1, d1)
	}
}

// A database that will not answer leaves the map standing: the set says it is partial.
func TestOSVSurvivesADatabaseThatWillNotAnswer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	o := &OSV{http: srv.Client(), API: srv.URL, Logf: func(string, ...any) {}}
	found, partial := o.Query(context.Background(), []Package{{Ecosystem: "go", Name: "example.com/m", Version: "1.0.0"}})
	if len(found) != 0 {
		t.Errorf("%d findings from a failing database", len(found))
	}
	if !partial {
		t.Error("a failed query was not reported as partial")
	}
}

// Nothing leaves the machine for an ecosystem the database does not cover.
func TestOSVAsksNothingWhenThereIsNothingToAsk(t *testing.T) {
	o := &OSV{http: &http.Client{}, API: "http://127.0.0.1:1"} // any request would fail
	for _, pkgs := range [][]Package{
		nil,
		{{Ecosystem: "psgallery", Name: "Pester", Version: "5.5.0"}},
		{{Ecosystem: "oci", Name: "alpine", Version: "3.19"}},
		{{Ecosystem: "go", Name: "example.com/m"}},
	} {
		if found, partial := o.Query(context.Background(), pkgs); len(found) != 0 || partial {
			t.Errorf("%v: %d findings, partial %t", pkgs, len(found), partial)
		}
	}
}

// An advisory that fixes two release lines separately suggests the fix on the line in
// use, not whichever the advisory happens to list first.
func TestOSVSuggestsTheFixOnTheLineInUse(t *testing.T) {
	var e osvEntry
	if err := json.Unmarshal([]byte(`{"id":"GHSA-x","affected":[
		{"package":{"name":"lib"},"ranges":[{"type":"SEMVER","events":[{"introduced":"0"},{"fixed":"2.9.10"}]}]},
		{"package":{"name":"lib"},"ranges":[{"type":"SEMVER","events":[{"introduced":"2.12.0"},{"fixed":"2.12.6"}]}]},
		{"package":{"name":"other"},"ranges":[{"type":"SEMVER","events":[{"introduced":"0"},{"fixed":"2.12.2"}]}]}
	]}`), &e); err != nil {
		t.Fatal(err)
	}
	for version, want := range map[string]string{
		"2.12.1": "2.12.6",
		"2.9.3":  "2.9.10",
		"":       "2.9.10", // nothing to compare with: the first fix, as before
		"r1234":  "2.9.10",
	} {
		if got := e.fixed("lib", version); got != want {
			t.Errorf("at %q: fixed in %q, want %q", version, got, want)
		}
	}
}

// An answer that does not match the questions one for one is not trusted: the
// packages it leaves out are not cached as having no advisories.
func TestOSVDoesNotCacheAShortAnswer(t *testing.T) {
	var batches int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&batches, 1)
		w.Write([]byte(`{"results":[{"vulns":[]}]}`)) // one answer to two questions
	}))
	defer srv.Close()
	o := &OSV{http: srv.Client(), cache: store.New(t.TempDir(), time.Hour), API: srv.URL, Logf: func(string, ...any) {}}
	pkgs := []Package{
		{Ecosystem: "go", Name: "example.com/a", Version: "v1.0.0"},
		{Ecosystem: "go", Name: "example.com/b", Version: "v1.0.0"},
	}
	if _, partial := o.Query(context.Background(), pkgs); !partial {
		t.Error("a short answer was reported as complete")
	}
	o.Query(context.Background(), pkgs)
	if n := atomic.LoadInt32(&batches); n != 2 {
		t.Errorf("the database was asked %d times, want 2: the short answer was cached", n)
	}
}

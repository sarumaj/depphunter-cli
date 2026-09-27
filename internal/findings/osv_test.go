package findings

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
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

// Verifies: REQ-FND-010, REQ-FND-011, REQ-FND-012
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
//
// Verifies: REQ-FND-016
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
//
// Verifies: REQ-FND-010, REQ-FND-014
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

// A Conan package is asked about as ConanCenter's; vcpkg has no OSV ecosystem, so a
// port is not asked about at all.
//
// Verifies: REQ-FND-010, REQ-CPP-011
func TestOSVAsksConanCenterForConanPackages(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Queries []struct {
				Package struct{ Name, Ecosystem string } `json:"package"`
				Version string                           `json:"version"`
			} `json:"queries"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		out := struct {
			Results []batchResult `json:"results"`
		}{}
		for _, q := range body.Queries {
			asked = append(asked, q.Package.Ecosystem+" "+q.Package.Name+" "+q.Version)
			out.Results = append(out.Results, batchResult{})
		}
		json.NewEncoder(w).Encode(out)
	}))
	defer srv.Close()
	o := &OSV{http: srv.Client(), cache: store.New(t.TempDir(), time.Hour), API: srv.URL}
	o.Query(context.Background(), []Package{
		{Ecosystem: "conan", Name: "zlib", Version: "1.2.13"},
		{Ecosystem: "vcpkg", Name: "zlib", Version: "1.2.13"},
	})
	if len(asked) != 1 || asked[0] != "ConanCenter zlib 1.2.13" {
		t.Errorf("asked %q, want only ConanCenter zlib 1.2.13", asked)
	}
}

// A Composer package is asked about as Packagist's, and a lock's "v6.4.2" as 6.4.2.
//
// Verifies: REQ-FND-010, REQ-PHP-009
func TestOSVAsksPackagistForComposerPackages(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Queries []struct {
				Package struct{ Name, Ecosystem string } `json:"package"`
				Version string                           `json:"version"`
			} `json:"queries"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		out := struct {
			Results []batchResult `json:"results"`
		}{}
		for _, q := range body.Queries {
			asked = append(asked, q.Package.Ecosystem+" "+q.Package.Name+" "+q.Version)
			out.Results = append(out.Results, batchResult{})
		}
		json.NewEncoder(w).Encode(out)
	}))
	defer srv.Close()
	o := &OSV{http: srv.Client(), cache: store.New(t.TempDir(), time.Hour), API: srv.URL}
	o.Query(context.Background(), []Package{
		{Ecosystem: "composer", Name: "symfony/http-foundation", Version: "v6.4.2"},
		{Ecosystem: "composer", Name: "monolog/monolog", Version: "3.5.0"},
	})
	sort.Strings(asked)
	want := []string{"Packagist monolog/monolog 3.5.0", "Packagist symfony/http-foundation 6.4.2"}
	if !reflect.DeepEqual(asked, want) {
		t.Errorf("asked %q, want %q", asked, want)
	}
}

// A gem is asked about in OSV's RubyGems ecosystem, by the version the lock pins.
//
// Verifies: REQ-FND-010, REQ-RUBY-009
func TestOSVAsksRubyGemsForGems(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Queries []struct {
				Package struct{ Name, Ecosystem string } `json:"package"`
				Version string                           `json:"version"`
			} `json:"queries"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		out := struct {
			Results []batchResult `json:"results"`
		}{}
		for _, q := range body.Queries {
			asked = append(asked, q.Package.Ecosystem+" "+q.Package.Name+" "+q.Version)
			out.Results = append(out.Results, batchResult{})
		}
		json.NewEncoder(w).Encode(out)
	}))
	defer srv.Close()
	o := &OSV{http: srv.Client(), cache: store.New(t.TempDir(), time.Hour), API: srv.URL}
	o.Query(context.Background(), []Package{{Ecosystem: "rubygems", Name: "nokogiri", Version: "1.15.4"}})
	if want := []string{"RubyGems nokogiri 1.15.4"}; !reflect.DeepEqual(asked, want) {
		t.Errorf("asked %q, want %q", asked, want)
	}
}

// A Swift package is asked about in OSV's SwiftURL ecosystem, by the URL it is named
// after and the version Package.resolved pins.
//
// Verifies: REQ-FND-010, REQ-SWIFT-006
func TestOSVAsksSwiftURLForSwiftPackages(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Queries []struct {
				Package struct{ Name, Ecosystem string } `json:"package"`
				Version string                           `json:"version"`
			} `json:"queries"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		out := struct {
			Results []batchResult `json:"results"`
		}{}
		for _, q := range body.Queries {
			asked = append(asked, q.Package.Ecosystem+" "+q.Package.Name+" "+q.Version)
			out.Results = append(out.Results, batchResult{})
		}
		json.NewEncoder(w).Encode(out)
	}))
	defer srv.Close()
	o := &OSV{http: srv.Client(), cache: store.New(t.TempDir(), time.Hour), API: srv.URL}
	o.Query(context.Background(), []Package{{Ecosystem: "swiftpm", Name: "github.com/apple/swift-nio", Version: "2.64.0"}})
	if want := []string{"SwiftURL github.com/apple/swift-nio 2.64.0"}; !reflect.DeepEqual(asked, want) {
		t.Errorf("asked %q, want %q", asked, want)
	}
}

// A pub package is asked about in OSV's Pub ecosystem, by the version pubspec.lock
// pins.
//
// Verifies: REQ-FND-010, REQ-DART-008
func TestOSVAsksPubForDartPackages(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Queries []struct {
				Package struct{ Name, Ecosystem string } `json:"package"`
				Version string                           `json:"version"`
			} `json:"queries"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		out := struct {
			Results []batchResult `json:"results"`
		}{}
		for _, q := range body.Queries {
			asked = append(asked, q.Package.Ecosystem+" "+q.Package.Name+" "+q.Version)
			out.Results = append(out.Results, batchResult{})
		}
		json.NewEncoder(w).Encode(out)
	}))
	defer srv.Close()
	o := &OSV{http: srv.Client(), cache: store.New(t.TempDir(), time.Hour), API: srv.URL}
	o.Query(context.Background(), []Package{{Ecosystem: "pub", Name: "http", Version: "1.2.1"}})
	if want := []string{"Pub http 1.2.1"}; !reflect.DeepEqual(asked, want) {
		t.Errorf("asked %q, want %q", asked, want)
	}
}

// A Hex package is asked about in OSV's Hex ecosystem, by the version mix.lock or
// rebar.lock pins.
//
// Verifies: REQ-FND-010, REQ-BEAM-011
func TestOSVAsksHexForBeamPackages(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Queries []struct {
				Package struct{ Name, Ecosystem string } `json:"package"`
				Version string                           `json:"version"`
			} `json:"queries"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		out := struct {
			Results []batchResult `json:"results"`
		}{}
		for _, q := range body.Queries {
			asked = append(asked, q.Package.Ecosystem+" "+q.Package.Name+" "+q.Version)
			out.Results = append(out.Results, batchResult{})
		}
		json.NewEncoder(w).Encode(out)
	}))
	defer srv.Close()
	o := &OSV{http: srv.Client(), cache: store.New(t.TempDir(), time.Hour), API: srv.URL}
	o.Query(context.Background(), []Package{{Ecosystem: "hex", Name: "plug", Version: "1.16.0"}})
	if want := []string{"Hex plug 1.16.0"}; !reflect.DeepEqual(asked, want) {
		t.Errorf("asked %q, want %q", asked, want)
	}
}

// An R package is asked about in OSV's CRAN ecosystem, a Bioconductor package in
// OSV's Bioconductor ecosystem, each by the version renv.lock pins.
//
// Verifies: REQ-FND-010, REQ-R-007, REQ-R-009
func TestOSVAsksCRANAndBioconductorForRPackages(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Queries []struct {
				Package struct{ Name, Ecosystem string } `json:"package"`
				Version string                           `json:"version"`
			} `json:"queries"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		out := struct {
			Results []batchResult `json:"results"`
		}{}
		for _, q := range body.Queries {
			asked = append(asked, q.Package.Ecosystem+" "+q.Package.Name+" "+q.Version)
			out.Results = append(out.Results, batchResult{})
		}
		json.NewEncoder(w).Encode(out)
	}))
	defer srv.Close()
	o := &OSV{http: srv.Client(), cache: store.New(t.TempDir(), time.Hour), API: srv.URL}
	o.Query(context.Background(), []Package{
		{Ecosystem: "cran", Name: "readxl", Version: "1.4.3"},
		{Ecosystem: "bioconductor", Name: "DESeq2", Version: "1.44.0"},
	})
	sort.Strings(asked)
	if want := []string{"Bioconductor DESeq2 1.44.0", "CRAN readxl 1.4.3"}; !reflect.DeepEqual(asked, want) {
		t.Errorf("asked %q, want %q", asked, want)
	}
}

// A Haskell package is asked about in OSV's Hackage ecosystem by the version the
// build plan, freeze file or lock pins.
//
// Verifies: REQ-FND-010, REQ-HASKELL-010
func TestOSVAsksHackageForHaskellPackages(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Queries []struct {
				Package struct{ Name, Ecosystem string } `json:"package"`
				Version string                           `json:"version"`
			} `json:"queries"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		out := struct {
			Results []batchResult `json:"results"`
		}{}
		for _, q := range body.Queries {
			asked = append(asked, q.Package.Ecosystem+" "+q.Package.Name+" "+q.Version)
			out.Results = append(out.Results, batchResult{})
		}
		json.NewEncoder(w).Encode(out)
	}))
	defer srv.Close()
	o := &OSV{http: srv.Client(), cache: store.New(t.TempDir(), time.Hour), API: srv.URL}
	o.Query(context.Background(), []Package{{Ecosystem: "hackage", Name: "aeson", Version: "2.2.3.0"}})
	if want := []string{"Hackage aeson 2.2.3.0"}; !reflect.DeepEqual(asked, want) {
		t.Errorf("asked %q, want %q", asked, want)
	}
}

// An OCaml package is asked about in OSV's opam ecosystem by the version a lock or
// an exact constraint pins; Jane Street's leading v is part of opam's version.
//
// Verifies: REQ-FND-010, REQ-OCAML-008
func TestOSVAsksOpamForOCamlPackages(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Queries []struct {
				Package struct{ Name, Ecosystem string } `json:"package"`
				Version string                           `json:"version"`
			} `json:"queries"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		out := struct {
			Results []batchResult `json:"results"`
		}{}
		for _, q := range body.Queries {
			asked = append(asked, q.Package.Ecosystem+" "+q.Package.Name+" "+q.Version)
			out.Results = append(out.Results, batchResult{})
		}
		json.NewEncoder(w).Encode(out)
	}))
	defer srv.Close()
	o := &OSV{http: srv.Client(), cache: store.New(t.TempDir(), time.Hour), API: srv.URL}
	o.Query(context.Background(), []Package{{Ecosystem: "opam", Name: "lwt", Version: "5.7.0"}, {Ecosystem: "opam", Name: "base", Version: "v0.16.3"}})
	if want := []string{"opam base v0.16.3", "opam lwt 5.7.0"}; !reflect.DeepEqual(asked, want) {
		t.Errorf("asked %q, want %q", asked, want)
	}
}

// A Julia package is asked about in OSV's Julia ecosystem by its name and the version
// its manifest pins.
//
// Verifies: REQ-FND-010, REQ-JULIA-008
func TestOSVAsksJuliaForJuliaPackages(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Queries []struct {
				Package struct{ Name, Ecosystem string } `json:"package"`
				Version string                           `json:"version"`
			} `json:"queries"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		out := struct {
			Results []batchResult `json:"results"`
		}{}
		for _, q := range body.Queries {
			asked = append(asked, q.Package.Ecosystem+" "+q.Package.Name+" "+q.Version)
			out.Results = append(out.Results, batchResult{})
		}
		json.NewEncoder(w).Encode(out)
	}))
	defer srv.Close()
	o := &OSV{http: srv.Client(), cache: store.New(t.TempDir(), time.Hour), API: srv.URL}
	o.Query(context.Background(), []Package{{Ecosystem: "julia", Name: "HTTP", Version: "1.10.8"}, {Ecosystem: "julia-std", Name: "Dates", Version: "1.11.0"}})
	if want := []string{"Julia HTTP 1.10.8"}; !reflect.DeepEqual(asked, want) {
		t.Errorf("asked %q, want %q", asked, want)
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
//
// Verifies: REQ-FND-012
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

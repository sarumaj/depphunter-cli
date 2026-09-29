package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/findings"
	"github.com/sarumaj/depphunter-cli/internal/graph"
	"github.com/sarumaj/depphunter-cli/internal/scope"
)

// TestLockFileGitDependenciesAskedByCommit maps each of the JavaScript plugin's
// testdata/gitlock projects (one install written in every npm, Yarn, pnpm and Bun
// lock format) and asks a stand-in for api.osv.dev about what the map pins. The git
// dependencies on public forges, pub (GitHub) and the transitive tr (GitLab), are
// asked about by their commits alone; corp, on a company host, is not asked about
// at all; neither is by its name and version; the registry package dep is asked
// about as npm. The commit's advisory is reported on pub. With a private pattern
// naming pub's repository, pub's commit is not sent either.
//
// Verifies: REQ-JS-019, REQ-FND-026, REQ-FND-010, REQ-SUP-040
func TestLockFileGitDependenciesAskedByCommit(t *testing.T) {
	const (
		pub = "3b8f1e6a9d2c4b7e0f5a8c1d6e9b2f4a7c0d3e5f"
		tr  = "6c1a9f4e2d7b0c5a8e3f6d1b9c4a7e2f5d0b8c3a"
	)
	var mu sync.Mutex
	var asked []string
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/querybatch", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Queries []struct {
				Commit  string
				Version string
				Package struct{ Name, Ecosystem string }
			}
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("batch body: %v", err)
		}
		type result struct {
			Vulns []struct {
				ID string `json:"id"`
			} `json:"vulns"`
		}
		out := struct {
			Results []result `json:"results"`
		}{}
		mu.Lock()
		defer mu.Unlock()
		for _, q := range body.Queries {
			var answer result
			if q.Commit != "" {
				asked = append(asked, "commit "+q.Commit)
				if q.Commit == pub {
					answer.Vulns = append(answer.Vulns, struct {
						ID string `json:"id"`
					}{"OSV-2026-1"})
				}
			} else {
				asked = append(asked, q.Package.Ecosystem+" "+q.Package.Name+" "+q.Version)
			}
			out.Results = append(out.Results, answer)
		}
		json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("GET /v1/vulns/OSV-2026-1", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":"OSV-2026-1","summary":"pub","affected":[{"ranges":[{"type":"GIT",` +
			`"repo":"https://github.com/acme-oss/pub","events":[{"introduced":"0"}]}]}]}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	t.Setenv("DEPPHUNTER_CACHE", "false")
	for _, format := range []string{"npm-v3", "npm-v1", "yarn-classic", "berry", "pnpm-v9", "pnpm-v6", "bun"} {
		t.Run(format, func(t *testing.T) {
			root := filepath.Join("..", "..", "internal", "lang", "javascript", "testdata", "gitlock", format)
			out := filepath.Join(t.TempDir(), "g.json")
			if _, err := execute(t, "--no-history", "--no-links", "--resolve-depth", "-1", "--export", "json", "-o", out, root); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			var g graph.Graph
			if err := json.Unmarshal(data, &g); err != nil {
				t.Fatal(err)
			}
			for _, c := range []struct {
				private []string
				want    []string
			}{
				{nil, []string{"commit " + pub, "commit " + tr, "npm dep 1.0.0"}},
				{[]string{"github.com/acme-oss/*"}, []string{"commit " + tr, "npm dep 1.0.0"}},
			} {
				mu.Lock()
				asked = nil
				mu.Unlock()
				o := findings.NewOSV("", time.Hour, 10*time.Second)
				o.API = server.URL
				found, _ := o.Query(context.Background(), pinned(&g, scope.New(c.private).Match))
				mu.Lock()
				got := append([]string(nil), asked...)
				mu.Unlock()
				sort.Strings(got)
				if !reflect.DeepEqual(got, c.want) {
					t.Errorf("private %q: asked\n%q\nwant\n%q", c.private, got, c.want)
				}
				var on []string
				for _, f := range found {
					on = append(on, f.Source+" "+f.Package+" "+f.Version)
				}
				want := []string{findings.SourceCommit + " pub " + pub}
				if c.private != nil {
					want = nil
				}
				if !reflect.DeepEqual(on, want) {
					t.Errorf("private %q: found %q, want %q", c.private, on, want)
				}
			}
		})
	}
}

package findings

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/store"
)

const (
	commitSwift  = "4aae40bf6fff5286e0e1672329d17824ce16e081" // swift-markdown, also asked by name
	commitZig    = "dc0a228a5544988d4a920cfb40be9cd28db41423" // libvaxis: a zig package
	commitBundle = "0123456789abcdef0123456789abcdef01234567" // devise from GitHub: private, by commit only
)

// commitServer answers like api.osv.dev: SwiftURL swift-markdown at its commit has
// GHSA-swift (alias CVE-2024-1), and its commit matches the CVE's own entry and
// OSV-2024-9; libvaxis's commit matches OSV-2024-7, whose GIT range is fixed at
// "f00d…" in its repository and at "beef…" in a fork; devise's commit matches
// GHSA-devise. It records every query it was sent.
func commitServer(t *testing.T) (*httptest.Server, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var asked []string
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/querybatch", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Queries []map[string]json.RawMessage `json:"queries"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("batch body: %v", err)
		}
		out := struct {
			Results []batchResult `json:"results"`
		}{}
		for _, q := range body.Queries {
			keys := make([]string, 0, len(q))
			for k := range q {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			var res batchResult
			var ids []string
			switch {
			case reflect.DeepEqual(keys, []string{"commit"}):
				var c string
				json.Unmarshal(q["commit"], &c)
				mu.Lock()
				asked = append(asked, "commit "+c)
				mu.Unlock()
				ids = map[string][]string{
					commitSwift:  {"CVE-2024-1", "OSV-2024-9"},
					commitZig:    {"OSV-2024-7"},
					commitBundle: {"GHSA-devise"},
				}[c]
			case reflect.DeepEqual(keys, []string{"package", "version"}):
				var p struct{ Name, Ecosystem string }
				var v string
				json.Unmarshal(q["package"], &p)
				json.Unmarshal(q["version"], &v)
				mu.Lock()
				asked = append(asked, p.Ecosystem+" "+p.Name+" "+v)
				mu.Unlock()
				if p.Name == "github.com/swiftlang/swift-markdown" {
					ids = []string{"GHSA-swift"}
				}
			default:
				t.Errorf("query with fields %v", keys)
			}
			for _, id := range ids {
				res.Vulns = append(res.Vulns, struct {
					ID string `json:"id"`
				}{id})
			}
			out.Results = append(out.Results, res)
		}
		json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("GET /v1/vulns/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		// cSpell: ignore beefbeefbeefbeefbeefbeefbeefbeefbeefbeef
		entry := map[string]string{
			"GHSA-swift":  `{"id":"GHSA-swift","summary":"swift","aliases":["CVE-2024-1"]}`,
			"CVE-2024-1":  `{"id":"CVE-2024-1","summary":"the same advisory, from the NVD"}`,
			"OSV-2024-9":  `{"id":"OSV-2024-9","summary":"found by fuzzing"}`,
			"GHSA-devise": `{"id":"GHSA-devise","summary":"devise"}`,
			"OSV-2024-7": `{"id":"OSV-2024-7","summary":"vaxis","affected":[{"ranges":[
				{"type":"GIT","repo":"https://github.com/someone/libvaxis-fork","events":[{"introduced":"0"},{"fixed":"beefbeefbeefbeefbeefbeefbeefbeefbeefbeef"}]},
				{"type":"GIT","repo":"https://github.com/rockorager/libvaxis.git","events":[{"introduced":"0"},{"fixed":"f00df00df00df00df00df00df00df00df00df00d"}]}]}]}`,
		}[id]
		if entry == "" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(entry))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		out := slices.Clone(asked)
		sort.Strings(out)
		return out
	}
}

func commitPackages() []Package {
	return []Package{
		// Asked by name and version (SwiftURL) and by its commit.
		{Ecosystem: "swiftpm", Name: "github.com/swiftlang/swift-markdown", Version: commitSwift, Commit: commitSwift, Repo: "github.com/swiftlang/swift-markdown"},
		// No OSV ecosystem: by its commit only.
		{Ecosystem: "zig", Name: "github.com/rockorager/libvaxis", Version: commitZig, Commit: commitZig, Repo: "github.com/rockorager/libvaxis"},
		// Private for having come from GitHub rather than rubygems.org: by its commit
		// only, never by its name and version.
		{Ecosystem: "rubygems", Name: "devise", Version: "4.9.3", Commit: commitBundle, Repo: "github.com/heartcombo/devise", CommitOnly: true},
		// A shortened commit is not a pin anybody can be asked about; the version is.
		{Ecosystem: "npm", Name: "forge-std", Version: "1.0.0", Commit: "1eea5ba"},
		// No commit: asked about by name and version, as before.
		{Ecosystem: "go", Name: "golang.org/x/text", Version: "v0.14.0"},
	}
}

// A package pinned to a full git commit is asked about by that commit, alone, in the
// same batch as the packages asked about by name; a shortened commit and a private
// package's name and version are not sent. What the commit matched is placed on the
// package under its own source, with the fix its repository's GIT range names, and
// an advisory its name and version already returned - by id or by alias - is not
// repeated. A second run asks nothing.
//
// Verifies: REQ-FND-026, REQ-FND-010, REQ-FND-011, REQ-FND-012
func TestOSVAsksAboutGitCommits(t *testing.T) {
	srv, asked := commitServer(t)
	dir := t.TempDir()
	o := &OSV{http: srv.Client(), cache: store.New(dir, time.Hour), API: srv.URL}
	found, partial := o.Query(context.Background(), commitPackages())
	if partial {
		t.Error("a complete answer was reported as partial")
	}
	want := []string{
		"Go golang.org/x/text 0.14.0",
		"SwiftURL github.com/swiftlang/swift-markdown " + commitSwift,
		"commit " + commitBundle,
		"commit " + commitSwift,
		"commit " + commitZig,
		"npm forge-std 1.0.0", // by its name and version only
	}
	if got := asked(); !reflect.DeepEqual(got, want) {
		t.Errorf("asked\n%q\nwant\n%q", got, want)
	}

	var got []string
	for _, f := range found {
		got = append(got, f.Source+" "+f.Ecosystem+" "+f.Package+" "+f.Ref+" fixed="+f.Fixed)
	}
	sort.Strings(got)
	wantFound := []string{
		"OSV (git commit) rubygems devise GHSA-devise fixed=",
		"OSV (git commit) swiftpm github.com/swiftlang/swift-markdown OSV-2024-9 fixed=",
		"OSV (git commit) zig github.com/rockorager/libvaxis OSV-2024-7 fixed=f00df00df00df00df00df00df00df00df00df00d",
		"osv swiftpm github.com/swiftlang/swift-markdown GHSA-swift fixed=",
	}
	if !reflect.DeepEqual(got, wantFound) {
		t.Errorf("found\n%q\nwant\n%q", got, wantFound)
	}
	for _, f := range found {
		if f.Package == "devise" && f.Version != "4.9.3" {
			t.Errorf("devise's finding is at %q, want the version the map shows", f.Version)
		}
		if f.Package == "devise" && f.Detail != "Matched by its git commit "+commitBundle+" of github.com/heartcombo/devise." {
			t.Errorf("devise's finding does not say which commit matched: %q", f.Detail)
		}
	}

	n := len(asked())
	again := &OSV{http: srv.Client(), cache: store.New(dir, time.Hour), API: srv.URL}
	if f, _ := again.Query(context.Background(), commitPackages()); len(f) != len(found) {
		t.Errorf("cached run: %d findings, want %d", len(f), len(found))
	}
	if len(asked()) != n {
		t.Errorf("cached run asked again: %q", asked()[n:])
	}
}

// Two packages at one commit (a Bazel module and the repository it fetches) are one
// question, and the answer is placed on both.
//
// Verifies: REQ-FND-026, REQ-FND-011
func TestOSVAsksAboutACommitOnce(t *testing.T) {
	srv, asked := commitServer(t)
	o := &OSV{http: srv.Client(), API: srv.URL}
	found, _ := o.Query(context.Background(), []Package{
		{Ecosystem: "bazel", Name: "libvaxis", Version: commitZig, Commit: commitZig, CommitOnly: true},
		{Ecosystem: "bazel-repo", Name: "github.com/rockorager/libvaxis", Version: commitZig, Commit: commitZig},
	})
	if got := asked(); !reflect.DeepEqual(got, []string{"commit " + commitZig}) {
		t.Errorf("asked %q", got)
	}
	var on []string
	for _, f := range found {
		on = append(on, f.Ecosystem+" "+f.Ref)
	}
	sort.Strings(on)
	if want := []string{"bazel OSV-2024-7", "bazel-repo OSV-2024-7"}; !reflect.DeepEqual(on, want) {
		t.Errorf("found on %q, want %q", on, want)
	}
}

// The set names the commit source whenever commits were asked about, so the panel
// can say what ran, and keeps it apart from the answers by name and version.
//
// Verifies: REQ-FND-026, REQ-FND-020
func TestCollectReportsCommitFindingsUnderTheirOwnSource(t *testing.T) {
	srv, _ := commitServer(t)
	set := Collect(context.Background(), Options{
		Root:     t.TempDir(),
		OSV:      &OSV{http: srv.Client(), API: srv.URL},
		Packages: commitPackages(),
	})
	if want := []string{SourceCommit, "osv"}; !reflect.DeepEqual(set.Sources, want) {
		t.Errorf("sources %q, want %q", set.Sources, want)
	}
	if len(set.Findings) != 4 {
		t.Errorf("%d findings, want 4", len(set.Findings))
	}

	set = Collect(context.Background(), Options{
		Root:     t.TempDir(),
		OSV:      &OSV{http: srv.Client(), API: srv.URL},
		Packages: []Package{{Ecosystem: "go", Name: "golang.org/x/text", Version: "v0.14.0"}},
	})
	if want := []string{"osv"}; !reflect.DeepEqual(set.Sources, want) {
		t.Errorf("without commits: sources %q, want %q", set.Sources, want)
	}
}

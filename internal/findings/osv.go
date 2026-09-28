package findings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/semver"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/store"
)

// The OSV database: the one question a repository's own files cannot answer, which is
// whether the versions it pins are known to be vulnerable. Asked only with --online.
const (
	osvAPI    = "https://api.osv.dev"
	osvBatch  = 500 // queries per batch request; the API allows 1000
	osvDetail = 6   // concurrent detail fetches
)

// osvEcosystems maps this project's ecosystem ids onto OSV's names. An ecosystem that
// is not here is not asked about by name and version: a PowerShell Gallery module or a
// CI runner image has no OSV counterpart, and guessing one would invent findings. (A
// package of any ecosystem pinned to a git commit on a public forge is asked about by
// that commit - see Package.Commit - which is how Carthage, Zig, shards, Paket's
// GitHub files, submodules, jsonnet-bundler and the rest below get an answer.) Neither CocoaPods nor
// Carthage is an OSV ecosystem, so pods are not asked about; nor are LuaRocks and
// Wally, so rocks and Wally packages are not either, nor CPAN (OSV's ecosystem list
// has none for Perl), so CPAN distributions are not, nor Zig packages, nor Bazel
// modules and WORKSPACE downloads (the Maven, PyPI, Go, npm and crates.io packages
// Bazel's module extensions install are asked about under their own ecosystems),
// nor flake inputs and nixpkgs packages (OSV has no Nix ecosystem), nor Elm
// or PureScript packages (OSV has no Elm or PureScript ecosystem), nor Crystal
// shards (OSV has no Crystal ecosystem), nor the files and repositories Paket
// fetches from GitHub, git servers and HTTP (F# and C# NuGet packages, Paket's
// included, are asked about as NuGet), nor dub packages (OSV has no D ecosystem)
// nor fpm packages (no Fortran ecosystem either) nor haxelib libraries (nor a
// Haxe one) nor Alire crates (nor an Ada one) nor Racket packages (nor a
// Racket one) nor Quicklisp projects (nor a Common Lisp one) nor Soldeer
// packages and git submodules (no Solidity ecosystem; the npm packages of
// Hardhat projects are asked about as npm) nor nimble packages (no Nim
// ecosystem) nor jsonnet-bundler packages and CUE modules (neither has an
// ecosystem; Go packages CUE was generated from are asked about as Go) nor
// Dhall packages and Puppet modules (OSV has neither a Dhall nor a Puppet
// Forge ecosystem).
//
// Implements: REQ-FND-010
var osvEcosystems = map[string]string{
	"go":       "Go",
	"npm":      "npm",
	"pypi":     "PyPI",
	"crates":   "crates.io",
	"maven":    "Maven",
	"nuget":    "NuGet",
	"actions":  "GitHub Actions",
	"conan":    "ConanCenter", // vcpkg has no OSV ecosystem
	"composer": "Packagist",
	"rubygems": "RubyGems",
	"swiftpm":  "SwiftURL", // packages are named by their URL, as OSV names them
	"pub":      "Pub",
	"hex":      "Hex",
	"cran":     "CRAN",
	// A Bioconductor package is its own ecosystem in OSV: the same name on CRAN
	// would be another package.
	"bioconductor": "Bioconductor",
	// Haskell packages are Hackage's; GHC's own libraries (haskell-std) would be OSV's
	// GHC ecosystem, but standard libraries are not asked about.
	"hackage": "Hackage",
	// OCaml packages are opam-repository's; the compiler's own libraries (ocaml-std)
	// are not asked about.
	"opam": "opam",
	// Julia packages are the General registry's (by name, as OSV's Julia ecosystem
	// names them); Julia's standard libraries (julia-std) are not asked about.
	"julia": "Julia",
}

// Package is one thing to ask the database about: a dependency pinned to a version,
// to a git commit, or to both.
type Package struct {
	Ecosystem string
	Name      string
	Version   string
	// Commit is the full git commit the package was built from (lang.GitPin). It is
	// asked about on its own - OSV matches a commit against the repositories and
	// commit ranges its advisories record, whatever the ecosystem - so a package of
	// an ecosystem OSV does not know can still be answered for.
	//
	// Implements: REQ-FND-026
	Commit string
	// Repository is the repository the commit is from, as lang.RepositoryName spells it. It is
	// not sent; it chooses which of an advisory's fixed commits to suggest.
	Repository string
	// CommitOnly sends the commit and nothing else: the package itself is private
	// (it came from outside every index), although its repository is public.
	CommitOnly bool
}

// SourceCommit is the source a finding answered for a package's git commit is
// reported under, apart from "osv" for its name and version.
//
// Implements: REQ-FND-026
const SourceCommit = "OSV (git commit)"

// OSV asks api.osv.dev which of the given packages are known to be vulnerable.
type OSV struct {
	http  *http.Client
	cache *store.Store
	// API overrides the database's address; tests point it at their own server.
	API string
	// Logf reports what could not be asked; nil is silent.
	Logf func(string, ...any)
}

// Implements: REQ-FND-012
func NewOSV(directory string, ttl, timeout time.Duration) *OSV {
	return &OSV{http: &http.Client{Timeout: timeout}, cache: store.New(directory, ttl)}
}

// question is one query of a batch: a package at a version, or a commit alone.
type question struct {
	askedPackage Package // the name and the version asked about; zero for a commit
	commit       string
}

// key is where a question's answer is kept.
func (q question) key() string {
	if q.commit != "" {
		return "osv-commit|" + q.commit
	}
	return queryKey(q.askedPackage)
}

// Query returns a finding per (package, vulnerability) pair, and whether anything was
// left unasked. A database that will not answer is not an error the map can use: the
// set is marked partial and what did arrive is kept.
//
// A package pinned to a git commit is asked about by that commit too. An advisory
// both questions return is reported once, from the package's name and version: the
// same id, or one listed among the other's aliases (a GHSA and the CVE it names),
// is the same advisory.
//
// Implements: REQ-FND-010, REQ-FND-016, REQ-FND-026
func (o *OSV) Query(ctx context.Context, packages []Package) ([]*Finding, bool) {
	var queries []Package
	byCommit := map[string][]Package{}
	seen := map[string]bool{}
	for _, p := range packages {
		if c := strings.ToLower(p.Commit); lang.Commit(c) && p.Name != "" {
			p.Commit = c
			if k := "commit|" + p.Ecosystem + "|" + p.Name + "|" + c; !seen[k] {
				seen[k] = true
				byCommit[c] = append(byCommit[c], p)
			}
		}
		ecosystem, ok := osvEcosystems[p.Ecosystem]
		if !ok || p.CommitOnly || p.Name == "" || p.Version == "" {
			continue
		}
		p.Version = osvVersion(p.Ecosystem, p.Version)
		key := ecosystem + "|" + p.Name + "|" + p.Version
		if seen[key] {
			continue
		}
		seen[key] = true
		queries = append(queries, p)
	}
	if len(queries) == 0 && len(byCommit) == 0 {
		return nil, false
	}
	sort.Slice(queries, func(i, j int) bool {
		a, b := queries[i], queries[j]
		return a.Ecosystem+a.Name+a.Version < b.Ecosystem+b.Name+b.Version
	})
	var asked []question
	for _, p := range queries {
		asked = append(asked, question{askedPackage: Package{Ecosystem: p.Ecosystem, Name: p.Name, Version: p.Version}})
	}
	for _, c := range slices.Sorted(maps.Keys(byCommit)) {
		asked = append(asked, question{commit: c})
	}

	ids, partial := o.ids(ctx, asked)
	entries, missed := o.entries(ctx, ids)
	partial = partial || missed

	var out []*Finding
	// known is every id and alias already reported per package, so the answer to a
	// commit does not repeat what its name and version said.
	known := map[string]bool{}
	for _, p := range queries {
		for _, id := range ids[question{askedPackage: Package{Ecosystem: p.Ecosystem, Name: p.Name, Version: p.Version}}] {
			e, ok := entries[id]
			if !ok {
				continue
			}
			for _, a := range append([]string{e.ID}, e.Aliases...) {
				known[p.Ecosystem+"|"+p.Name+"|"+a] = true
			}
			out = append(out, e.finding(p))
		}
	}
	for _, c := range slices.Sorted(maps.Keys(byCommit)) {
		for _, p := range byCommit[c] {
			for _, id := range ids[question{commit: c}] {
				e, ok := entries[id]
				if !ok || known[p.Ecosystem+"|"+p.Name+"|"+e.ID] || slices.ContainsFunc(e.Aliases, func(a string) bool {
					return known[p.Ecosystem+"|"+p.Name+"|"+a]
				}) {
					continue
				}
				out = append(out, e.commitFinding(p))
			}
		}
	}
	return out, partial
}

// queryKey is where one package's answer is kept.
func queryKey(p Package) string { return "osv-query|" + p.Ecosystem + "|" + p.Name + "|" + p.Version }

// ids asks which vulnerabilities each question matches. The answer is only a list of
// ids, which is what makes one batch request enough for a whole dependency tree.
//
// Implements: REQ-FND-011, REQ-FND-012
func (o *OSV) ids(ctx context.Context, qs []question) (map[question][]string, bool) {
	out := map[question][]string{}
	ask := qs[:0:0]
	for _, q := range qs {
		if ids, ok := store.Get[[]string](o.cache, q.key()); ok {
			if len(ids) > 0 {
				out[q] = ids
			}
			continue
		}
		ask = append(ask, q)
	}
	partial := false
	for start := 0; start < len(ask); start += osvBatch {
		end := min(start+osvBatch, len(ask))
		batch := ask[start:end]
		result, err := o.batch(ctx, batch)
		if err == nil && len(result) != len(batch) {
			// Answers are matched to questions by position: a short answer cannot be
			// matched, and caching the missing ones as clean would hide them for the
			// whole time to live.
			err = fmt.Errorf("%d answers to %d questions", len(result), len(batch))
		}
		if err != nil {
			o.logFormat("osv.dev: %v", err)
			partial = true
			continue
		}
		for i, q := range batch {
			var ids []string
			for _, v := range result[i].Vulnerabilities {
				ids = append(ids, v.ID)
			}
			if result[i].NextPageToken != "" {
				// More than one page of advisories: what came is shown, but it is not
				// the whole answer and is not cached as one.
				partial = true
			} else {
				o.cache.Put(q.key(), ids)
			}
			if len(ids) > 0 {
				out[q] = ids
			}
		}
	}
	return out, partial
}

type batchResult struct {
	Vulnerabilities []struct {
		ID string `json:"id"`
	} `json:"vulns"`
	NextPageToken string `json:"next_page_token"`
}

// batch sends one /v1/querybatch request. A package question names the package and
// its version; a commit question is {"commit": "<sha>"} and nothing else, which is
// how OSV's API asks about a commit.
func (o *OSV) batch(ctx context.Context, qs []question) ([]batchResult, error) {
	type osvPackage struct {
		Name      string `json:"name"`
		Ecosystem string `json:"ecosystem"`
	}
	type query struct {
		Commit  string      `json:"commit,omitempty"`
		Package *osvPackage `json:"package,omitempty"`
		Version string      `json:"version,omitempty"`
	}
	body := struct {
		Queries []query `json:"queries"`
	}{}
	for _, q := range qs {
		if q.commit != "" {
			body.Queries = append(body.Queries, query{Commit: q.commit})
			continue
		}
		p := q.askedPackage
		body.Queries = append(body.Queries, query{Package: &osvPackage{Name: p.Name, Ecosystem: osvEcosystems[p.Ecosystem]}, Version: p.Version})
	}
	buffer, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, o.base()+"/v1/querybatch", bytes.NewReader(buffer))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	var result struct {
		Results []batchResult `json:"results"`
	}
	if err := o.do(request, &result); err != nil {
		return nil, err
	}
	return result.Results, nil
}

// entries fetches the vulnerabilities themselves. Only the few ids a query matched are
// fetched, and each one only once however many packages it affects.
//
// Implements: REQ-FND-011
func (o *OSV) entries(ctx context.Context, ids map[question][]string) (map[string]*osvEntry, bool) {
	want := map[string]bool{}
	for _, list := range ids {
		for _, id := range list {
			want[id] = true
		}
	}
	out := map[string]*osvEntry{}
	var todo []string
	for id := range want {
		if e, ok := store.Get[osvEntry](o.cache, "osv-vuln|"+id); ok {
			out[id] = &e
			continue
		}
		todo = append(todo, id)
	}
	sort.Strings(todo)

	var mu sync.Mutex
	var wg sync.WaitGroup
	partial := false
	sem := make(chan struct{}, osvDetail)
	for _, id := range todo {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			e, err := o.vulnerability(ctx, id)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				o.logFormat("osv.dev %s: %v", id, err)
				partial = true
				return
			}
			out[id] = e
		}(id)
	}
	wg.Wait()
	return out, partial
}

func (o *OSV) vulnerability(ctx context.Context, id string) (*osvEntry, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, o.base()+"/v1/vulns/"+id, nil)
	if err != nil {
		return nil, err
	}
	var e osvEntry
	if err := o.do(request, &e); err != nil {
		return nil, err
	}
	o.cache.Put("osv-vuln|"+id, e)
	return &e, nil
}

func (o *OSV) do(request *http.Request, into any) error {
	response, err := o.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		io.Copy(io.Discard, io.LimitReader(response.Body, 4<<10))
		return fmt.Errorf("%s: %s", request.URL.Path, response.Status)
	}
	return json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(into)
}

func (o *OSV) logFormat(format string, arguments ...any) {
	if o.Logf != nil {
		o.Logf(format, arguments...)
	}
}

// base is the API root; tests point it elsewhere.
func (o *OSV) base() string {
	if o.API != "" {
		return o.API
	}
	return osvAPI
}

// osvVersion spells a version the way the database does. A Composer lock keeps a
// tag's "v" (symfony/http-foundation v6.4.2); Packagist advisories name 6.4.2.
//
// Implements: REQ-FND-010
func osvVersion(ecosystem, v string) string {
	switch ecosystem {
	case "go":
		v = strings.TrimSuffix(v, "+incompatible")
		v = strings.TrimPrefix(v, "v")
	case "composer":
		if len(v) > 1 && v[0] == 'v' && v[1] >= '0' && v[1] <= '9' {
			v = v[1:]
		}
	}
	return v
}

// osvEntry is one vulnerability as the database (and govulncheck, and osv-scanner)
// records it. Only the fields the map shows are read.
type osvEntry struct {
	ID       string   `json:"id"`
	Summary  string   `json:"summary"`
	Details  string   `json:"details"`
	Aliases  []string `json:"aliases"`
	Severity []struct {
		Type  string `json:"type"`
		Score string `json:"score"`
	} `json:"severity"`
	Affected []struct {
		Package struct {
			Ecosystem string `json:"ecosystem"`
			Name      string `json:"name"`
		} `json:"package"`
		Ranges []struct {
			Type       string `json:"type"`
			Repository string `json:"repo"` // a GIT range's repository
			Events     []struct {
				Introduced string `json:"introduced"`
				Fixed      string `json:"fixed"`
			} `json:"events"`
		} `json:"ranges"`
		DatabaseSpecific map[string]any `json:"database_specific"`
	} `json:"affected"`
	References []struct {
		Type string `json:"type"`
		URL  string `json:"url"`
	} `json:"references"`
	DatabaseSpecific map[string]any `json:"database_specific"`
}

// finding places the vulnerability on the package it affects.
//
// Implements: REQ-FND-020
func (e *osvEntry) finding(p Package) *Finding {
	f := &Finding{
		Kind:      KindVulnerability,
		Source:    "osv",
		Reference: e.ID,
		Severity:  e.severity(),
		Title:     e.title(),
		Detail:    e.detail(),
		URL:       e.url(),
		Ecosystem: p.Ecosystem,
		Package:   p.Name,
		Version:   p.Version,
		Fixed:     e.fixed(p.Name, p.Version),
	}
	return f
}

// commitFinding places the vulnerability on a package its git commit matched. The
// fix is the commit an affected GIT range of the package's own repository names.
//
// Implements: REQ-FND-026
func (e *osvEntry) commitFinding(p Package) *Finding {
	f := e.finding(Package{Ecosystem: p.Ecosystem, Name: p.Name, Version: p.Version})
	f.Source = SourceCommit
	f.Fixed = e.fixedCommit(p.Repository)
	// Which commit it was: the version the map shows may be a release or a shortened
	// commit, and an advisory matched by commit says nothing about releases.
	how := "Matched by its git commit " + p.Commit
	if p.Repository != "" {
		how += " of " + p.Repository
	}
	if f.Detail != "" {
		how = f.Detail + "\n\n" + how
	}
	f.Detail = how + "."
	if f.Version == "" {
		f.Version = p.Commit
	}
	return f
}

// fixedCommit is the last fixed commit of a GIT range in repo ("" matches any
// repository), "" when the advisory names none.
func (e *osvEntry) fixedCommit(repository string) string {
	for _, a := range e.Affected {
		for _, r := range a.Ranges {
			if r.Type != "GIT" || repository != "" && !within(repository, lang.RepositoryName(r.Repository)) {
				continue
			}
			for i := len(r.Events) - 1; i >= 0; i-- {
				if fix := r.Events[i].Fixed; fix != "" {
					return fix
				}
			}
		}
	}
	return ""
}

// within reports whether repository, which may name a directory inside a repository
// (github.com/grafana/jsonnet-libs/ksonnet-util), is in the repository r.
func within(repository, r string) bool {
	repository, r = strings.ToLower(repository), strings.ToLower(r)
	return r != "" && (repository == r || strings.HasPrefix(repository, r+"/"))
}

func (e *osvEntry) title() string {
	if s := strings.TrimSpace(e.Summary); s != "" {
		return trim(s, 200)
	}
	line, _, _ := strings.Cut(strings.TrimSpace(e.Details), "\n")
	if line != "" {
		return trim(line, 200)
	}
	return e.ID
}

func (e *osvEntry) detail() string {
	d := trim(e.Details, 1200)
	if aliases := e.otherIDs(); len(aliases) > 0 {
		alias := "Also known as " + strings.Join(aliases, ", ") + "."
		if d == "" {
			return alias
		}
		d += "\n\n" + alias
	}
	return d
}

func (e *osvEntry) otherIDs() []string {
	var out []string
	for _, a := range e.Aliases {
		if a != e.ID {
			out = append(out, a)
		}
	}
	return out
}

// severity prefers the CVSS vector, which says what an advisory's severity word only
// summarizes; the word is the fallback, and some databases give neither.
//
// Implements: REQ-FND-018
func (e *osvEntry) severity() Severity {
	for _, s := range e.Severity {
		if !strings.HasPrefix(s.Type, "CVSS_V3") && s.Type != "" && !strings.HasPrefix(s.Score, "CVSS:3") {
			continue
		}
		if score, ok := scoreCVSS(s.Score); ok {
			return severityOfScore(score)
		}
	}
	if s, ok := e.DatabaseSpecific["severity"].(string); ok {
		if level := severity(s); level != Unknown {
			return level
		}
	}
	for _, a := range e.Affected {
		if s, ok := a.DatabaseSpecific["severity"].(string); ok {
			if level := severity(s); level != Unknown {
				return level
			}
		}
	}
	return Unknown
}

func (e *osvEntry) url() string {
	if u, ok := e.DatabaseSpecific["url"].(string); ok && u != "" {
		return u
	}
	for _, r := range e.References {
		if r.Type == "ADVISORY" && r.URL != "" {
			return r.URL
		}
	}
	return "https://osv.dev/vulnerability/" + e.ID
}

// fixed is the version of the package to move to, when the advisory names one.
//
// An advisory often fixes each release line separately - 2.9.x in one release, 2.12.x
// in another - so the answer is the lowest fix above the version in use; a fix on an
// older line would be a downgrade that is still vulnerable. When the versions cannot
// be compared, the answer is the last fix of the first affected range.
func (e *osvEntry) fixed(packageName, version string) string {
	current := semverOf(version)
	first, best := "", ""
	for _, a := range e.Affected {
		if packageName != "" && a.Package.Name != "" && !strings.EqualFold(a.Package.Name, packageName) {
			continue
		}
		for _, r := range a.Ranges {
			for i := len(r.Events) - 1; i >= 0; i-- {
				fix := r.Events[i].Fixed
				if fix == "" {
					continue
				}
				if first == "" {
					first = fix
				}
				v := semverOf(fix)
				if current == "" || v == "" || semver.Compare(v, current) <= 0 {
					continue
				}
				if best == "" || semver.Compare(v, semverOf(best)) < 0 {
					best = fix
				}
			}
		}
	}
	if best != "" {
		return best
	}
	return first
}

// semverOf is version as golang.org/x/mod/semver reads it, "" when it cannot.
func semverOf(version string) string {
	v := "v" + strings.TrimPrefix(strings.TrimSpace(version), "v")
	if !semver.IsValid(v) {
		return ""
	}
	return v
}

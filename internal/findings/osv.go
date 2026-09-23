package findings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/semver"

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
// is not here is not asked about: a PowerShell Gallery module or a CI runner image has
// no OSV counterpart, and guessing one would invent findings.
var osvEcosystems = map[string]string{
	"go":      "Go",
	"npm":     "npm",
	"pypi":    "PyPI",
	"crates":  "crates.io",
	"maven":   "Maven",
	"nuget":   "NuGet",
	"actions": "GitHub Actions",
}

// Package is one thing to ask the database about: a dependency pinned to a version.
type Package struct {
	Ecosystem string
	Name      string
	Version   string
}

// OSV asks api.osv.dev which of the given packages are known to be vulnerable.
type OSV struct {
	http  *http.Client
	cache *store.Store
	// API overrides the database's address; tests point it at their own server.
	API string
	// Logf reports what could not be asked; nil is silent.
	Logf func(string, ...any)
}

func NewOSV(dir string, ttl, timeout time.Duration) *OSV {
	return &OSV{http: &http.Client{Timeout: timeout}, cache: store.New(dir, ttl)}
}

// Query returns a finding per (package, vulnerability) pair, and whether anything was
// left unasked. A database that will not answer is not an error the map can use: the
// set is marked partial and what did arrive is kept.
func (o *OSV) Query(ctx context.Context, pkgs []Package) ([]*Finding, bool) {
	queries := make([]Package, 0, len(pkgs))
	seen := map[string]bool{}
	for _, p := range pkgs {
		eco, ok := osvEcosystems[p.Ecosystem]
		if !ok || p.Name == "" || p.Version == "" {
			continue
		}
		p.Version = osvVersion(p.Ecosystem, p.Version)
		key := eco + "|" + p.Name + "|" + p.Version
		if seen[key] {
			continue
		}
		seen[key] = true
		queries = append(queries, p)
	}
	if len(queries) == 0 {
		return nil, false
	}
	sort.Slice(queries, func(i, j int) bool {
		a, b := queries[i], queries[j]
		return a.Ecosystem+a.Name+a.Version < b.Ecosystem+b.Name+b.Version
	})

	ids, partial := o.ids(ctx, queries)
	entries, missed := o.entries(ctx, ids)
	partial = partial || missed

	var out []*Finding
	for _, p := range queries {
		for _, id := range ids[p] {
			e, ok := entries[id]
			if !ok {
				continue
			}
			out = append(out, e.finding(p))
		}
	}
	return out, partial
}

// queryKey is where one package's answer is kept.
func queryKey(p Package) string { return "osv-query|" + p.Ecosystem + "|" + p.Name + "|" + p.Version }

// ids asks which vulnerabilities affect each package. The answer is only a list of
// ids, which is what makes one batch request enough for a whole dependency tree.
func (o *OSV) ids(ctx context.Context, pkgs []Package) (map[Package][]string, bool) {
	out := map[Package][]string{}
	ask := pkgs[:0:0]
	for _, p := range pkgs {
		if ids, ok := store.Get[[]string](o.cache, queryKey(p)); ok {
			if len(ids) > 0 {
				out[p] = ids
			}
			continue
		}
		ask = append(ask, p)
	}
	partial := false
	for start := 0; start < len(ask); start += osvBatch {
		end := min(start+osvBatch, len(ask))
		batch := ask[start:end]
		res, err := o.batch(ctx, batch)
		if err == nil && len(res) != len(batch) {
			// Answers are matched to questions by position: a short answer cannot be
			// matched, and caching the missing ones as clean would hide them for the
			// whole time to live.
			err = fmt.Errorf("%d answers to %d questions", len(res), len(batch))
		}
		if err != nil {
			o.logFormat("osv.dev: %v", err)
			partial = true
			continue
		}
		for i, p := range batch {
			var ids []string
			for _, v := range res[i].Vulns {
				ids = append(ids, v.ID)
			}
			if res[i].NextPageToken != "" {
				// More than one page of advisories: what came is shown, but it is not
				// the whole answer and is not cached as one.
				partial = true
			} else {
				o.cache.Put(queryKey(p), ids)
			}
			if len(ids) > 0 {
				out[p] = ids
			}
		}
	}
	return out, partial
}

type batchResult struct {
	Vulns []struct {
		ID string `json:"id"`
	} `json:"vulns"`
	NextPageToken string `json:"next_page_token"`
}

func (o *OSV) batch(ctx context.Context, pkgs []Package) ([]batchResult, error) {
	type query struct {
		Package struct {
			Name      string `json:"name"`
			Ecosystem string `json:"ecosystem"`
		} `json:"package"`
		Version string `json:"version"`
	}
	body := struct {
		Queries []query `json:"queries"`
	}{}
	for _, p := range pkgs {
		var q query
		q.Package.Name, q.Package.Ecosystem, q.Version = p.Name, osvEcosystems[p.Ecosystem], p.Version
		body.Queries = append(body.Queries, q)
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.base()+"/v1/querybatch", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	var res struct {
		Results []batchResult `json:"results"`
	}
	if err := o.do(req, &res); err != nil {
		return nil, err
	}
	return res.Results, nil
}

// entries fetches the vulnerabilities themselves. Only the few ids a query matched are
// fetched, and each one only once however many packages it affects.
func (o *OSV) entries(ctx context.Context, ids map[Package][]string) (map[string]*osvEntry, bool) {
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
			e, err := o.vuln(ctx, id)
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

func (o *OSV) vuln(ctx context.Context, id string) (*osvEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.base()+"/v1/vulns/"+id, nil)
	if err != nil {
		return nil, err
	}
	var e osvEntry
	if err := o.do(req, &e); err != nil {
		return nil, err
	}
	o.cache.Put("osv-vuln|"+id, e)
	return &e, nil
}

func (o *OSV) do(req *http.Request, into any) error {
	res, err := o.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		io.Copy(io.Discard, io.LimitReader(res.Body, 4<<10))
		return fmt.Errorf("%s: %s", req.URL.Path, res.Status)
	}
	return json.NewDecoder(io.LimitReader(res.Body, 8<<20)).Decode(into)
}

func (o *OSV) logFormat(format string, args ...any) {
	if o.Logf != nil {
		o.Logf(format, args...)
	}
}

// base is the API root; tests point it elsewhere.
func (o *OSV) base() string {
	if o.API != "" {
		return o.API
	}
	return osvAPI
}

// osvVersion spells a version the way the database does.
func osvVersion(ecosystem, v string) string {
	if ecosystem == "go" {
		v = strings.TrimSuffix(v, "+incompatible")
		v = strings.TrimPrefix(v, "v")
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
			Type   string `json:"type"`
			Events []struct {
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
func (e *osvEntry) finding(p Package) *Finding {
	f := &Finding{
		Kind:      KindVulnerability,
		Source:    "osv",
		Ref:       e.ID,
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
		if sev := severity(s); sev != Unknown {
			return sev
		}
	}
	for _, a := range e.Affected {
		if s, ok := a.DatabaseSpecific["severity"].(string); ok {
			if sev := severity(s); sev != Unknown {
				return sev
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
// in another - so the answer is the lowest fix above the version in use: the first
// one found could be a fix on an older line, which is a downgrade that is still
// vulnerable. Versions that cannot be compared fall back to the last fix listed.
func (e *osvEntry) fixed(pkg, version string) string {
	current := semverOf(version)
	first, best := "", ""
	for _, a := range e.Affected {
		if pkg != "" && a.Package.Name != "" && !strings.EqualFold(a.Package.Name, pkg) {
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

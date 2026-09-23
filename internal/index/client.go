package index

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"

	"github.com/sarumaj/depphunter-cli/internal/auth"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scope"
	"github.com/sarumaj/depphunter-cli/internal/store"
	"github.com/sarumaj/depphunter-cli/internal/trace"
)

// Client asks package indexes what a package depends on, for the ecosystems whose
// answer is not already in the repository. It is used only with --online, and only
// against indexes this machine's own configuration names: an index a repository
// points at is never fetched from, however plainly it asks.
type Client struct {
	cfg     *Config
	http    *http.Client
	cache   *store.Store
	auth    *auth.Store
	private *scope.Private
	timeout time.Duration

	mu   sync.Mutex
	seen map[string][]lang.Target // answers already given, including empty ones
	// failed is when each question the index would not answer was last asked. It is
	// asked again once failRetry has passed: one client serves every re-analysis in
	// --watch, and a timeout remembered as an answer would leave a package with no
	// dependencies until the process is restarted.
	failed map[string]time.Time
	// feeds is what a NuGet service index resolved to: the same answer for every
	// package on that feed, and one request rather than one per package.
	feeds map[string]string
	// rep is the report this run is writing, if anybody is reading it. It is set per
	// analysis - one client serves every re-analysis in --watch - so it is guarded
	// like the rest.
	rep *trace.Report
}

// Trace points the client at the report of the analysis now running. Passing nil
// stops it recording. internal/analyze calls this; nothing else needs to.
func (c *Client) Trace(r *trace.Report) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rep = r
}

// report records one question, if anybody is listening.
func (c *Client) report(l trace.Lookup) {
	c.mu.Lock()
	r := c.rep
	c.mu.Unlock()
	r.Add(l)
}

// NewClient prepares the client. dir holds the cached answers; ttl is how long one
// stays usable; credentials are what this machine holds for its indexes, and private
// names the packages that must not be asked of a public index.
func NewClient(cfg *Config, dir string, ttl, timeout time.Duration,
	credentials *auth.Store, private *scope.Private) *Client {
	return &Client{
		cfg:     cfg,
		http:    &http.Client{Timeout: timeout},
		cache:   store.New(dir, ttl),
		auth:    credentials,
		private: private,
		timeout: timeout,
		seen:    map[string][]lang.Target{},
		failed:  map[string]time.Time{},
		feeds:   map[string]string{},
	}
}

// Dependencies implements lang.Transitive against the indexes. Every way out is
// recorded (internal/trace), because a question declined for what asking would
// disclose is indistinguishable on the map from a dependency that genuinely has none.
func (c *Client) Dependencies(t lang.Target) []lang.Target {
	if c == nil || t.Package == "" {
		return nil
	}
	l := trace.Lookup{Ecosystem: t.Ecosystem, Package: t.Package, Version: t.Version, Answer: trace.NoAnswer}
	index, known := c.cfg.For(t.Ecosystem, t.Package)
	l.Index = index
	if !known {
		// An index only the repository asks for is not fetched from.
		l.Reason = trace.ReasonUntrusted
		if index == "" {
			l.Reason = trace.ReasonNoIndex
		}
		c.report(l)
		return nil
	}
	// An organization's own package is not named to the world: asking the public
	// index about corp.example/billing would not answer anyway, and the request
	// itself is the disclosure. A private registry this machine configures is asked
	// as usual.
	if c.private.Match(t.Ecosystem, t.Package) && c.cfg.Public(t.Ecosystem, index) {
		l.Reason = trace.ReasonPrivate
		c.report(l)
		return nil
	}
	key := t.Ecosystem + " " + t.Package + "@" + t.Version
	c.mu.Lock()
	cached, ok := c.seen[key]
	if at, failed := c.failed[key]; !ok && failed && time.Since(at) < failRetry {
		ok = true // not asked again so soon; the lookup that failed was reported
	}
	c.mu.Unlock()
	if ok {
		l.Answer, l.Deps = trace.FromMemo, len(cached)
		c.report(l)
		return cached
	}

	start := time.Now()
	a, err := c.lookup(t, index)
	if err != nil {
		// An index that will not answer is not an error the map can use - but it is
		// the whole of what the report has to say about this package.
		a = answer{source: trace.NoAnswer, reason: err.Error(), requests: a.requests}
	}
	l.Answer, l.Deps, l.Reason = a.source, len(a.deps), a.reason
	l.Requests, l.Millis = a.requests, time.Since(start).Milliseconds()
	c.report(l)
	c.mu.Lock()
	if err != nil {
		c.failed[key] = time.Now()
	} else {
		delete(c.failed, key)
		c.seen[key] = a.deps
	}
	c.mu.Unlock()
	return a.deps
}

// failRetry is how long a question an index did not answer is left before it is
// asked again.
const failRetry = 5 * time.Minute

// answer is what one question came to: the dependencies, who provided them, and -
// when nobody did - why, with whatever went over the network on the way.
type answer struct {
	deps     []lang.Target
	source   trace.Answer
	reason   string
	requests []trace.Request
}

// lookup answers from the cache when it can, and from the index when it must.
func (c *Client) lookup(t lang.Target, index string) (answer, error) {
	key := t.Ecosystem + "|" + index + "|" + t.Package + "|" + t.Version
	if deps, ok := store.Get[[]dep](c.cache, key); ok {
		return answer{deps: c.targets(t.Ecosystem, deps), source: trace.FromCache}, nil
	}
	if t.Ecosystem == Go && t.Version == "" {
		// A module proxy serves a go.mod for one version; without one there is no
		// document to ask for. Said here rather than deeper down so the report can
		// say it, instead of recording an empty answer that looks like "no
		// dependencies".
		return answer{source: trace.NoAnswer, reason: trace.ReasonNoVersion}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	// Every request this question makes is collected, so the report can say which of
	// an image's three round trips was the one that failed.
	made := &requestLog{}
	ctx = context.WithValue(ctx, requestLogKey{}, made)

	var deps []dep
	var err error
	switch t.Ecosystem {
	case Go:
		deps, err = c.goModule(ctx, index, t)
	case NPM:
		deps, err = c.npmPackage(ctx, index, t)
	case PyPI:
		deps, err = c.pypiDistribution(ctx, index, t)
	case Cargo:
		deps, err = c.cargoCrate(ctx, index, t)
	case NuGet:
		deps, err = c.nugetPackage(ctx, index, t)
	case OCI:
		deps, err = c.ociBase(ctx, index, t)
	default:
		// Maven is the one that cannot be asked. A POM is addressed by group *and*
		// artifact, and the Java plugin puts only the group on the map (an import
		// names a package, and a package does not say which artifact ships it), so
		// there is no document to request. Saying nothing beats guessing an artifact.
		return answer{source: trace.NoAnswer, reason: trace.ReasonUnsupported}, nil
	}
	if err != nil {
		return answer{requests: made.taken()}, err
	}
	c.cache.Put(key, deps)
	return answer{deps: c.targets(t.Ecosystem, deps), source: trace.FromIndex, requests: made.taken()}, nil
}

// requestLog collects what one question sent, in the order it sent it. The ecosystem
// functions know nothing about it: it rides on the context they already carry, and
// do writes to it.
type requestLog struct {
	mu sync.Mutex
	at []trace.Request
}

type requestLogKey struct{}

func (l *requestLog) add(r trace.Request) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.at = append(l.at, r)
}

func (l *requestLog) taken() []trace.Request {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.at
}

// targets turns an index's answer into what the graph takes. The version travels with
// the name: without it the next level cannot be asked for at all - a module proxy
// serves a go.mod for a version, not for a module.
func (c *Client) targets(eco string, deps []dep) []lang.Target {
	out := make([]lang.Target, 0, len(deps))
	for _, d := range deps {
		out = append(out, lang.Target{
			Ecosystem: eco, Package: d.Name, Version: d.Version,
			Pinned: d.Pinned(eco),
		})
	}
	return out
}

// userAgent identifies depphunter to the indexes. crates.io refuses a request without
// one, and an index that is rate-limiting is owed a name to complain about.
const userAgent = "depphunter (+https://github.com/sarumaj/depphunter-cli)"

// maxBody is as much of an answer as any of this needs to read.
const maxBody = 8 << 20

// get performs one request, with whatever credentials this machine has for the host.
func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	return c.accept(ctx, url, "application/json")
}

// accept is get for the answers that are not JSON: a nuspec, a sparse index line, an
// image manifest that has to name the media types it will take.
func (c *Client) accept(ctx context.Context, url, media string) ([]byte, error) {
	resp, err := c.do(ctx, url, media, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return readLimited(resp)
}

// readLimited reads an answer, and no more of it than any of this has any use for.
func readLimited(resp *http.Response) ([]byte, error) {
	return io.ReadAll(io.LimitReader(resp.Body, maxBody))
}

// do makes one request and hands back the response unread. bearer, when given, is
// sent instead of this machine's own credentials - it is the token a registry handed
// out for this one pull (see ociToken).
func (c *Client) do(ctx context.Context, url, media, bearer string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", media)
	req.Header.Set("User-Agent", userAgent)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	} else {
		c.auth.Apply(req)
	}
	made, _ := ctx.Value(requestLogKey{}).(*requestLog)
	start := time.Now()
	resp, err := c.http.Do(req)
	status := "ok"
	switch {
	case err != nil:
		status = err.Error()
	case resp != nil:
		status = resp.Status
	}
	made.add(trace.Request{URL: url, Status: status, Millis: time.Since(start).Milliseconds()})
	return resp, err
}

// goModule reads a module's own requirements from its go.mod, which a module proxy
// serves on its own - no archive, no checkout. A version is guaranteed: lookup turns
// a module without one away, so that the report can say why rather than record an
// empty answer.
func (c *Client) goModule(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	escaped, err := module.EscapePath(t.Package)
	if err != nil {
		return nil, err
	}
	version, err := module.EscapeVersion(t.Version)
	if err != nil {
		return nil, err
	}
	body, err := c.get(ctx, fmt.Sprintf("%s/%s/@v/%s.mod", index, escaped, version))
	if err != nil {
		return nil, err
	}
	// Lax parsing: this is someone else's go.mod, and one line this version of the
	// parser dislikes should not cost the whole module's requirements.
	f, err := modfile.ParseLax("go.mod", body, nil)
	if err != nil {
		return nil, err
	}
	var out []dep
	for _, r := range f.Require {
		if !r.Indirect { // the proxy's go.mod lists what this module itself needs
			out = append(out, dep{Name: r.Mod.Path, Version: r.Mod.Version})
		}
	}
	return out, nil
}

// npmPackage reads a version's dependencies from the registry.
func (c *Client) npmPackage(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	version := t.Version
	if !lang.PinnedSemver(version) {
		version = "latest" // a range names no document to ask for
	}
	body, err := c.get(ctx, fmt.Sprintf("%s/%s/%s", index, strings.ReplaceAll(t.Package, "/", "%2f"), version))
	if err != nil {
		return nil, err
	}
	var doc struct {
		Dependencies map[string]string `json:"dependencies"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	out := make([]dep, 0, len(doc.Dependencies))
	for name, constraint := range doc.Dependencies {
		// A range is what the package asked for; it is not a version, and the next
		// request falls back to the current one.
		out = append(out, dep{Name: name, Version: constraint})
	}
	return out, nil
}

// pypiDistribution reads requires-dist from the JSON API. A simple index (PEP 503)
// serves file listings and no metadata, so this asks the host that has both.
func (c *Client) pypiDistribution(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	host := strings.TrimSuffix(strings.TrimSuffix(index, "/simple"), "/simple/")
	url := fmt.Sprintf("%s/pypi/%s/json", host, t.Package)
	if lang.Pinned(t.Version) {
		url = fmt.Sprintf("%s/pypi/%s/%s/json", host, t.Package, t.Version)
	}
	body, err := c.get(ctx, url)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Info struct {
			RequiresDist []string `json:"requires_dist"`
		} `json:"info"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	var out []dep
	for _, req := range doc.Info.RequiresDist {
		// "certifi (>=2017.4.17)" is a dependency; anything guarded by an extra is
		// installed only when that extra is asked for.
		if strings.Contains(req, "extra ==") {
			continue
		}
		if name := requirementName(req); name != "" {
			out = append(out, dep{Name: name, Version: requirementConstraint(req, name)})
		}
	}
	return out, nil
}

// requirementConstraint is what a requirement asks for, without its name, markers or
// brackets: "certifi (>=2017.4.17)" asks for ">=2017.4.17".
func requirementConstraint(req, name string) string {
	rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(req), name))
	if i := strings.Index(rest, ";"); i >= 0 {
		rest = rest[:i]
	}
	rest = strings.TrimSpace(rest)
	rest = strings.TrimPrefix(rest, "(")
	rest = strings.TrimSuffix(rest, ")")
	if strings.HasPrefix(rest, "[") { // an extras list, not a version
		return ""
	}
	return strings.TrimSpace(rest)
}

// dep is one dependency an index reported, with whatever it said about its version.
type dep struct {
	Name    string `json:"n"`
	Version string `json:"v,omitempty"`
}

// Pinned reports whether the version this index gave fixes one release. A Go module
// proxy answers with the exact version the build selects; npm and PyPI answer with
// the constraint the package asked for, which usually does not.
func (d dep) Pinned(eco string) bool {
	if eco == NPM {
		return lang.PinnedSemver(d.Version)
	}
	return lang.Pinned(d.Version)
}

// requirementName takes the distribution name off the front of a requirement.
func requirementName(req string) string {
	if i := strings.IndexAny(req, " <>=!~[;("); i >= 0 {
		req = req[:i]
	}
	return strings.TrimSpace(req)
}

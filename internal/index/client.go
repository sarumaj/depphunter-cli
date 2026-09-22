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

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scope"
)

// Client asks package indexes what a package depends on, for the ecosystems whose
// answer is not already in the repository. It is used only with --online, and only
// against indexes this machine's own configuration names: an index a repository
// points at is never fetched from, however plainly it asks.
type Client struct {
	cfg     *Config
	http    *http.Client
	cache   *store
	auth    *credentials
	private *scope.Private
	timeout time.Duration

	mu   sync.Mutex
	seen map[string][]lang.Target // answers already given, including empty ones
	// feeds is what a NuGet service index resolved to: the same answer for every
	// package on that feed, and one request rather than one per package.
	feeds map[string]string
}

// NewClient prepares the client. dir holds the cached answers; ttl is how long one
// stays usable; private names the packages that must not be asked of a public index.
func NewClient(cfg *Config, dir string, ttl, timeout time.Duration, home string, private *scope.Private) *Client {
	return &Client{
		cfg:     cfg,
		http:    &http.Client{Timeout: timeout},
		cache:   newStore(dir, ttl),
		auth:    readCredentials(home),
		private: private,
		timeout: timeout,
		seen:    map[string][]lang.Target{},
		feeds:   map[string]string{},
	}
}

// Dependencies implements lang.Transitive against the indexes.
func (c *Client) Dependencies(t lang.Target) []lang.Target {
	if c == nil || t.Package == "" {
		return nil
	}
	index, known := c.cfg.For(t.Ecosystem, t.Package)
	if !known {
		return nil // an index only the repository asks for is not fetched from
	}
	// An organization's own package is not named to the world. Asking the public
	// index about corp.example/billing would not answer anyway, and the asking is
	// itself the disclosure: the request says the package exists, and to whom. A
	// private registry that this machine is configured for is a different matter, and
	// is asked as usual.
	if c.private.Match(t.Ecosystem, t.Package) && c.cfg.Public(t.Ecosystem, index) {
		return nil
	}
	key := t.Ecosystem + " " + t.Package + "@" + t.Version
	c.mu.Lock()
	cached, ok := c.seen[key]
	c.mu.Unlock()
	if ok {
		return cached
	}

	deps, err := c.lookup(t, index)
	if err != nil {
		deps = nil // an index that will not answer is not an error the map can use
	}
	c.mu.Lock()
	c.seen[key] = deps
	c.mu.Unlock()
	return deps
}

// lookup answers from the cache when it can, and from the index when it must.
func (c *Client) lookup(t lang.Target, index string) ([]lang.Target, error) {
	key := t.Ecosystem + "|" + index + "|" + t.Package + "|" + t.Version
	if deps, ok := c.cache.get(key); ok {
		return c.targets(t.Ecosystem, deps), nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

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
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.cache.put(key, deps)
	return c.targets(t.Ecosystem, deps), nil
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
		c.auth.apply(req)
	}
	return c.http.Do(req)
}

// goModule reads a module's own requirements from its go.mod, which a module proxy
// serves on its own - no archive, no checkout.
func (c *Client) goModule(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	if t.Version == "" {
		return nil, nil // the proxy needs a version to serve a go.mod
	}
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

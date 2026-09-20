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
	timeout time.Duration

	mu   sync.Mutex
	seen map[string][]lang.Target // answers already given, including empty ones
}

// NewClient prepares the client. dir holds the cached answers; ttl is how long one
// stays usable.
func NewClient(cfg *Config, dir string, ttl, timeout time.Duration, home string) *Client {
	return &Client{
		cfg:     cfg,
		http:    &http.Client{Timeout: timeout},
		cache:   newStore(dir, ttl),
		auth:    readCredentials(home),
		timeout: timeout,
		seen:    map[string][]lang.Target{},
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
	if names, ok := c.cache.get(key); ok {
		return c.targets(t.Ecosystem, names), nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	var names []string
	var err error
	switch t.Ecosystem {
	case Go:
		names, err = c.goModule(ctx, index, t)
	case NPM:
		names, err = c.npmPackage(ctx, index, t)
	case PyPI:
		names, err = c.pypiDistribution(ctx, index, t)
	default:
		// The other ecosystems need a different request per index flavour; until
		// that is written, saying nothing is better than guessing.
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.cache.put(key, names)
	return c.targets(t.Ecosystem, names), nil
}

func (c *Client) targets(eco string, names []string) []lang.Target {
	out := make([]lang.Target, 0, len(names))
	for _, n := range names {
		// A version would be a second request per dependency; the version a package
		// resolves to is what the project's own files decide anyway.
		out = append(out, lang.Target{Ecosystem: eco, Package: n})
	}
	return out
}

// get performs one request, with whatever credentials this machine has for the host.
func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	c.auth.apply(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
}

// goModule reads a module's own requirements from its go.mod, which a module proxy
// serves on its own - no archive, no checkout.
func (c *Client) goModule(ctx context.Context, index string, t lang.Target) ([]string, error) {
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
	var out []string
	for _, r := range f.Require {
		if !r.Indirect { // the proxy's go.mod lists what this module itself needs
			out = append(out, r.Mod.Path)
		}
	}
	return out, nil
}

// npmPackage reads a version's dependencies from the registry.
func (c *Client) npmPackage(ctx context.Context, index string, t lang.Target) ([]string, error) {
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
	out := make([]string, 0, len(doc.Dependencies))
	for name := range doc.Dependencies {
		out = append(out, name)
	}
	return out, nil
}

// pypiDistribution reads requires-dist from the JSON API. A simple index (PEP 503)
// serves file listings and no metadata, so this asks the host that has both.
func (c *Client) pypiDistribution(ctx context.Context, index string, t lang.Target) ([]string, error) {
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
	var out []string
	for _, req := range doc.Info.RequiresDist {
		// "certifi (>=2017.4.17)" is a dependency; anything guarded by an extra is
		// installed only when that extra is asked for.
		if strings.Contains(req, "extra ==") {
			continue
		}
		if name := requirementName(req); name != "" {
			out = append(out, name)
		}
	}
	return out, nil
}

// requirementName takes the distribution name off the front of a requirement.
func requirementName(req string) string {
	if i := strings.IndexAny(req, " <>=!~[;("); i >= 0 {
		req = req[:i]
	}
	return strings.TrimSpace(req)
}

package index

import (
	"cmp"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// The ecosystems whose transitive dependencies are read from a registry rather than
// from the repository: crates.io and its mirrors, NuGet feeds, and OCI registries.
// Each speaks its own protocol; ociBase, below, explains the odd one out.

// ---------------------------------------------------------------- crates.io

// cargoCrate reads a crate's own dependencies from a sparse index (RFC 2789): one
// line of JSON per published version, at a path derived from the crate's name. Every
// modern registry speaks it, but crates.io serves its index from a different host
// than the registry Cargo is configured with, so the one is turned into the other
// here; a mirror already names its index and is used as given.
//
// Implements: REQ-SUP-024
func (c *Client) cargoCrate(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	return c.cargoSparse(ctx, cargoIndex(index), t)
}

// cargoIndex is the sparse index for a configured Cargo source. Cargo writes the
// protocol into the URL ("sparse+https://…"), and crates.io keeps its index on a host
// of its own.
func cargoIndex(index string) string {
	index = strings.TrimRight(strings.TrimPrefix(index, "sparse+"), "/")
	if host := Host(index); host == "" || host == "crates.io" {
		return "https://index.crates.io"
	}
	return index
}

// cargoSparse reads the last line of a crate's sparse-index file, which is its newest
// published version, or the line for the version asked for.
func (c *Client) cargoSparse(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	body, err := c.accept(ctx, index+"/"+sparsePath(t.Package), "text/plain, */*")
	if err != nil {
		return nil, err
	}
	type line struct {
		Vers string `json:"vers"`
		Deps []struct {
			Name     string `json:"name"`
			Req      string `json:"req"`
			Kind     string `json:"kind"`
			Optional bool   `json:"optional"`
		} `json:"deps"`
		Yanked bool `json:"yanked"`
	}
	var best *line
	for _, raw := range strings.Split(string(body), "\n") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		var l line
		if json.Unmarshal([]byte(raw), &l) != nil || l.Yanked {
			continue
		}
		if t.Version != "" && l.Vers == strings.TrimPrefix(t.Version, "v") {
			best = &l
			break
		}
		best = &l // in published order, so the last one standing is the newest
	}
	if best == nil {
		return nil, nil
	}
	var out []dep
	for _, d := range best.Deps {
		if d.Kind != "" && d.Kind != "normal" || d.Optional {
			continue
		}
		out = append(out, dep{Name: d.Name, Version: d.Req})
	}
	return out, nil
}

// sparsePath is where a sparse index keeps a crate: one directory per name length up
// to three, and two letters at a time past that. Names are matched in lower case.
func sparsePath(name string) string {
	n := strings.ToLower(name)
	switch {
	case len(n) <= 2:
		return fmt.Sprintf("%d/%s", len(n), n)
	case len(n) == 3:
		return fmt.Sprintf("3/%s/%s", n[:1], n)
	default:
		return fmt.Sprintf("%s/%s/%s", n[:2], n[2:4], n)
	}
}

// ---------------------------------------------------------------- NuGet

// nugetPackage reads a package's dependencies from its nuspec.
//
// A NuGet feed is a service index naming the resources it offers, so the address of
// the packages themselves has to be asked for before anything can be fetched from it.
// That answer is the same for every package on the feed and is kept for the run.
//
// Implements: REQ-SUP-025
func (c *Client) nugetPackage(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	base, err := c.nugetBase(ctx, index)
	if err != nil || base == "" {
		return nil, err
	}
	id := strings.ToLower(t.Package)
	version, err := c.nugetVersion(ctx, base, id, t.Version)
	if err != nil || version == "" {
		return nil, err
	}
	body, err := c.accept(ctx, fmt.Sprintf("%s/%s/%s/%s.nuspec", base, id, version, id), "application/xml, */*")
	if err != nil {
		return nil, err
	}
	var doc struct {
		Metadata struct {
			Dependencies struct {
				// A nuspec lists dependencies either flat or grouped by target
				// framework, and plenty list both.
				Direct []nuspecDep `xml:"dependency"`
				Groups []struct {
					Dependency []nuspecDep `xml:"dependency"`
				} `xml:"group"`
			} `xml:"dependencies"`
		} `xml:"metadata"`
	}
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []dep
	add := func(list []nuspecDep) {
		for _, d := range list {
			if d.ID == "" || seen[strings.ToLower(d.ID)] {
				continue
			}
			seen[strings.ToLower(d.ID)] = true
			out = append(out, dep{Name: d.ID, Version: d.Version})
		}
	}
	add(doc.Metadata.Dependencies.Direct)
	for _, g := range doc.Metadata.Dependencies.Groups {
		add(g.Dependency)
	}
	return out, nil
}

type nuspecDep struct {
	ID      string `xml:"id,attr"`
	Version string `xml:"version,attr"`
}

// nugetBase resolves a feed's service index to the flat container that serves nuspecs
// and version listings, and remembers the answer.
func (c *Client) nugetBase(ctx context.Context, index string) (string, error) {
	c.mu.Lock()
	base, ok := c.feeds[index]
	c.mu.Unlock()
	if ok {
		return base, nil
	}
	base, err := c.readNuGetIndex(ctx, index)
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	if c.feeds == nil {
		c.feeds = map[string]string{}
	}
	c.feeds[index] = base
	c.mu.Unlock()
	return base, nil
}

func (c *Client) readNuGetIndex(ctx context.Context, index string) (string, error) {
	address := index
	if !strings.HasSuffix(address, ".json") {
		address += "/index.json" // a feed named by its root rather than its document
	}
	body, err := c.get(ctx, address)
	if err != nil {
		return "", err
	}
	var doc struct {
		Resources []struct {
			ID   string `json:"@id"`
			Type string `json:"@type"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", err
	}
	for _, r := range doc.Resources {
		if strings.HasPrefix(r.Type, "PackageBaseAddress/3.0") {
			return strings.TrimRight(r.ID, "/"), nil
		}
	}
	return "", nil
}

// nugetVersion is the version to fetch: the one asked for when it names one, and the
// newest release on the feed when it does not.
func (c *Client) nugetVersion(ctx context.Context, base, id, want string) (string, error) {
	if lang.Pinned(want) {
		return strings.ToLower(strings.Trim(strings.TrimPrefix(strings.TrimSpace(want), "["), "[]")), nil
	}
	body, err := c.accept(ctx, fmt.Sprintf("%s/%s/index.json", base, id), "application/json")
	if err != nil {
		return "", err
	}
	var doc struct {
		Versions []string `json:"versions"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", err
	}
	newest := ""
	for _, v := range doc.Versions {
		if strings.ContainsAny(v, "-") {
			continue // a pre-release is not what a project gets by default
		}
		newest = v
	}
	if newest == "" && len(doc.Versions) > 0 {
		newest = doc.Versions[len(doc.Versions)-1]
	}
	return strings.ToLower(newest), nil
}

// ---------------------------------------------------------------- Composer

// composerPackage reads a package's requirements from a Composer repository's
// metadata (Composer 2's /p2/<vendor>/<name>.json): every tagged version with its
// require, newest first. Packagist's own address is known; any other repository
// (Private Packagist, Satis, a proxy) names it as "metadata-url" in its
// packages.json, which is asked once per run.
//
// Implements: REQ-SUP-044
func (c *Client) composerPackage(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	pattern, err := c.composerMetadataURL(ctx, index)
	if err != nil || pattern == "" {
		return nil, err
	}
	name := strings.ToLower(t.Package)
	body, err := c.get(ctx, strings.ReplaceAll(pattern, "%package%", name))
	if err != nil {
		return nil, err
	}
	var doc struct {
		Packages map[string][]map[string]json.RawMessage `json:"packages"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	versions := expandComposer(doc.Packages[name])
	if len(versions) == 0 {
		return nil, nil
	}
	chosen := versions[0] // the newest: what an unpinned requirement installs today
	want := strings.TrimPrefix(strings.TrimSpace(t.Version), "v")
	for _, v := range versions {
		var version string
		if json.Unmarshal(v["version"], &version) == nil && want != "" && strings.TrimPrefix(version, "v") == want {
			chosen = v
			break
		}
	}
	var require map[string]string
	if raw, ok := chosen["require"]; ok {
		_ = json.Unmarshal(raw, &require) // "__unset" or a broken entry: nothing required
	}
	out := make([]dep, 0, len(require))
	for name, constraint := range require {
		if strings.Contains(name, "/") { // php, ext-*, lib-* are the platform
			out = append(out, dep{Name: name, Version: constraint})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// expandComposer undoes Composer 2's minified metadata: each version lists only what
// differs from the one before it, and "__unset" removes a key.
func expandComposer(versions []map[string]json.RawMessage) []map[string]json.RawMessage {
	out := make([]map[string]json.RawMessage, 0, len(versions))
	current := map[string]json.RawMessage{}
	for _, v := range versions {
		next := make(map[string]json.RawMessage, len(current)+len(v))
		for k, val := range current {
			next[k] = val
		}
		for k, val := range v {
			if string(val) == `"__unset"` {
				delete(next, k)
			} else {
				next[k] = val
			}
		}
		out = append(out, next)
		current = next
	}
	return out
}

// composerMetadataURL is where a Composer repository serves one package's metadata,
// with %package% for the name, and remembers the answer.
func (c *Client) composerMetadataURL(ctx context.Context, index string) (string, error) {
	if index == public[Composer] {
		return index + "/p2/%package%.json", nil
	}
	key := Composer + " " + index
	c.mu.Lock()
	pattern, ok := c.feeds[key]
	c.mu.Unlock()
	if ok {
		return pattern, nil
	}
	body, err := c.get(ctx, index+"/packages.json")
	if err != nil {
		return "", err
	}
	var doc struct {
		MetadataURL string `json:"metadata-url"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", err
	}
	// Relative to the repository's host, as Composer reads it. Not url.Parse: the
	// placeholder is not a valid escape.
	switch m := doc.MetadataURL; {
	case m == "":
	case strings.HasPrefix(m, "https://"), strings.HasPrefix(m, "http://"):
		pattern = m
	case strings.HasPrefix(m, "/"):
		if u, err := url.Parse(index); err == nil {
			pattern = u.Scheme + "://" + u.Host + m
		}
	default:
		pattern = index + "/" + m
	}
	c.mu.Lock()
	if c.feeds == nil {
		c.feeds = map[string]string{}
	}
	c.feeds[key] = pattern
	c.mu.Unlock()
	return pattern, nil
}

// ---------------------------------------------------------------- RubyGems

// rubygemsPackage reads a gem's runtime dependencies from the compact index Bundler
// itself reads (/info/<name>): one line per published version and platform, "1.2.3
// dep:>= 1&< 2,other:~> 3|checksum:...". rubygems.org serves it, and so do the
// servers Bundler is pointed at (Gemfury, Artifactory, Gemstash). The line of the
// version asked for answers, the platform-independent one first; without a version,
// the newest release (the last line that is neither a pre-release nor for one
// platform only) does.
//
// Implements: REQ-SUP-045
func (c *Client) rubygemsPackage(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	body, err := c.accept(ctx, strings.TrimRight(index, "/")+"/info/"+t.Package, "text/plain, */*")
	if err != nil {
		return nil, err
	}
	want := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(t.Version), "="))
	var newest, exact, exactPlatform string
	for _, line := range strings.Split(string(body), "\n") {
		version, rest, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok || line == "---" {
			continue
		}
		plain, platform, _ := strings.Cut(version, "-")
		switch {
		case plain == want && platform == "":
			exact = rest
		case plain == want && exactPlatform == "":
			exactPlatform = rest
		}
		if platform == "" && !strings.ContainsAny(plain, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ") {
			newest = rest
		}
	}
	chosen := exact
	if chosen == "" {
		chosen = exactPlatform
	}
	if chosen == "" {
		chosen = newest
	}
	deps, _, _ := strings.Cut(chosen, "|")
	var out []dep
	for _, d := range strings.Split(deps, ",") {
		name, req, ok := strings.Cut(strings.TrimSpace(d), ":")
		if !ok || name == "" {
			continue
		}
		reqs := strings.Split(req, "&")
		for i := range reqs {
			reqs[i] = strings.TrimSpace(reqs[i])
		}
		version := strings.Join(reqs, ", ")
		if exact, ok := strings.CutPrefix(version, "= "); ok && !strings.Contains(exact, ",") {
			version = exact // one version: pinned, as lang.Pinned reads it
		}
		out = append(out, dep{Name: name, Version: version})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ---------------------------------------------------------------- pub

// pubPackage reads a package's dependencies from a pub server's package API
// (<server>/api/packages/<name>), which pub.dev and every server implementing the
// hosted-repository protocol serve: each version with its pubspec. The version asked
// for answers, else the latest. Its `dependencies` count - not dev_dependencies - each
// with its constraint; an SDK package (flutter) comes from no server, and a git or
// path dependency of a published package cannot occur.
//
// Implements: REQ-SUP-046
func (c *Client) pubPackage(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	body, err := c.accept(ctx, strings.TrimRight(index, "/")+"/api/packages/"+t.Package, "application/vnd.pub.v2+json")
	if err != nil {
		return nil, err
	}
	type version struct {
		Version string `json:"version"`
		Pubspec struct {
			Dependencies map[string]any `json:"dependencies"`
		} `json:"pubspec"`
	}
	var doc struct {
		Latest   version   `json:"latest"`
		Versions []version `json:"versions"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	chosen := doc.Latest
	for _, v := range doc.Versions {
		if v.Version == strings.TrimSpace(t.Version) {
			chosen = v
		}
	}
	var out []dep
	for name, spec := range chosen.Pubspec.Dependencies {
		switch s := spec.(type) {
		case nil:
			out = append(out, dep{Name: name})
		case string:
			out = append(out, dep{Name: name, Version: s})
		case map[string]any:
			if _, sdk := s["sdk"]; sdk {
				continue
			}
			v, _ := s["version"].(string)
			out = append(out, dep{Name: name, Version: v})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ---------------------------------------------------------------- Hex

// hexPackage reads a Hex package's dependencies from the Hex API: the package
// (<api>/packages/<name>) says which releases exist and which is the latest stable
// one, and the release asked for - else that one - lists its requirements
// (<api>/packages/<name>/releases/<version>), keyed by package name. Optional
// requirements are left out: they are installed only when something else asks.
//
// Implements: REQ-SUP-047
func (c *Client) hexPackage(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	base := strings.TrimRight(index, "/") + "/packages/" + url.PathEscape(t.Package)
	body, err := c.accept(ctx, base, "application/json")
	if err != nil {
		return nil, err
	}
	var pkg struct {
		Latest    string `json:"latest_stable_version"`
		LatestAny string `json:"latest_version"`
		Releases  []struct {
			Version string `json:"version"`
		} `json:"releases"`
	}
	if err := json.Unmarshal(body, &pkg); err != nil {
		return nil, err
	}
	version := cmp.Or(pkg.Latest, pkg.LatestAny)
	for _, r := range pkg.Releases {
		if r.Version == strings.TrimSpace(t.Version) {
			version = r.Version
		}
	}
	if version == "" {
		return nil, nil
	}
	body, err = c.accept(ctx, base+"/releases/"+url.PathEscape(version), "application/json")
	if err != nil {
		return nil, err
	}
	var rel struct {
		Requirements map[string]struct {
			Requirement string `json:"requirement"`
			Optional    bool   `json:"optional"`
		} `json:"requirements"`
	}
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, err
	}
	var out []dep
	for name, r := range rel.Requirements {
		if r.Optional {
			continue
		}
		v := strings.TrimSpace(r.Requirement)
		if exact, ok := strings.CutPrefix(v, "=="); ok && !strings.ContainsAny(strings.TrimSpace(exact), " ") {
			v = strings.TrimSpace(exact) // one version: pinned, as lang.Pinned reads it
		}
		out = append(out, dep{Name: name, Version: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ---------------------------------------------------------------- CRAN

// crandbAPI serves CRAN's package metadata as JSON: <api>/<name> for the current
// release, <api>/<name>/<version> for any earlier one. CRAN itself publishes only
// the PACKAGES file of every package's current release.
var crandbAPI = "https://crandb.r-pkg.org"

// rBase are the packages of priority "base": part of R itself, never installed from
// a repository, so not dependencies to follow.
var rBase = map[string]bool{
	"R": true, "base": true, "compiler": true, "datasets": true, "grDevices": true, "graphics": true,
	"grid": true, "methods": true, "parallel": true, "splines": true, "stats": true, "stats4": true,
	"tcltk": true, "tools": true, "utils": true,
}

// cranPackage reads an R package's dependencies - Depends, Imports and LinkingTo,
// without R and its base packages - from crandb when the index is CRAN or a mirror
// of it (the version asked for when it names one release, else the current one), and
// from the repository's src/contrib/PACKAGES file otherwise: the index every
// CRAN-like repository (drat, r-universe, an internal one) serves, read once.
//
// Implements: REQ-SUP-048
func (c *Client) cranPackage(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	if !CRANMirror(index) {
		pkgs, err := c.cranRepo(ctx, index)
		if err != nil {
			return nil, err
		}
		return pkgs[t.Package], nil
	}
	base := strings.TrimRight(crandbAPI, "/") + "/" + url.PathEscape(t.Package)
	var body []byte
	var err error
	if v := strings.TrimSpace(t.Version); lang.Pinned(v) {
		body, err = c.get(ctx, base+"/"+url.PathEscape(v))
	}
	if body == nil {
		// No release named, or crandb does not have it: the current one answers.
		if body, err = c.get(ctx, base); err != nil {
			return nil, err
		}
	}
	var doc struct {
		Depends, Imports, LinkingTo map[string]string
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	var out []dep
	seen := map[string]bool{}
	for _, field := range []map[string]string{doc.Depends, doc.Imports, doc.LinkingTo} {
		for name, req := range field {
			if rBase[name] || seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, dep{Name: name, Version: rRequirement(req)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// rRequirement writes an R version requirement as the R plugin does: "*" is none,
// "== 1.2" the bare version (pinned), anything else ">= 1.2".
func rRequirement(req string) string {
	req = strings.Join(strings.Fields(strings.Trim(strings.TrimSpace(req), "()")), " ")
	if req == "*" {
		return ""
	}
	if v, ok := strings.CutPrefix(req, "=="); ok {
		return strings.TrimSpace(v)
	}
	return req
}

// cranRepo reads a CRAN-like repository's src/contrib/PACKAGES: one DCF record per
// package, with its Depends, Imports and LinkingTo.
func (c *Client) cranRepo(ctx context.Context, index string) (map[string][]dep, error) {
	c.mu.Lock()
	pkgs, ok := c.repos[index]
	c.mu.Unlock()
	if ok {
		return pkgs, nil
	}
	body, err := c.accept(ctx, strings.TrimRight(index, "/")+"/src/contrib/PACKAGES", "text/plain")
	if err != nil {
		return nil, err
	}
	pkgs = map[string][]dep{}
	for _, rec := range strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n\n") {
		fields := map[string]string{}
		last := ""
		for _, line := range strings.Split(rec, "\n") {
			if line == "" {
				continue
			}
			if line[0] == ' ' || line[0] == '\t' {
				fields[last] += " " + strings.TrimSpace(line)
				continue
			}
			if k, v, ok := strings.Cut(line, ":"); ok {
				last = k
				fields[k] = strings.TrimSpace(v)
			}
		}
		name := fields["Package"]
		if name == "" {
			continue
		}
		var out []dep
		seen := map[string]bool{}
		for _, f := range []string{"Depends", "Imports", "LinkingTo"} {
			for _, entry := range strings.Split(fields[f], ",") {
				n, req, _ := strings.Cut(strings.TrimSpace(entry), "(")
				n = strings.TrimSpace(n)
				if n == "" || rBase[n] || seen[n] {
					continue
				}
				seen[n] = true
				out = append(out, dep{Name: n, Version: rRequirement(req)})
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
		pkgs[name] = out
	}
	c.mu.Lock()
	c.repos[index] = pkgs
	c.mu.Unlock()
	return pkgs, nil
}

// ---------------------------------------------------------------- Hackage

// hackageStd are the packages that come with GHC: the compiler's own library and
// runtime, not dependencies to follow (the Haskell plugin's haskell-std island).
var hackageStd = map[string]bool{
	"base": true, "ghc-prim": true, "ghc": true, "template-haskell": true, "integer-gmp": true,
	"ghc-bignum": true, "ghc-boot": true, "ghc-boot-th": true, "ghc-heap": true, "rts": true,
	"ghc-internal": true, "ghc-experimental": true, "integer-simple": true,
}

// hackagePackage reads a Haskell package's dependencies from a Hackage server: the
// package's preferred versions (<server>/package/<name>/preferred, JSON) say which
// releases exist, and the release asked for - else the newest normal one - has its
// package description read (<server>/package/<name>-<version>/<name>.cabal, the
// latest revision). The dependencies are the build-depends of its libraries and of
// the common stanzas they import, conditional blocks included, without GHC's own
// packages and the package's own sublibraries.
//
// Implements: REQ-SUP-049
func (c *Client) hackagePackage(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	base := strings.TrimRight(index, "/") + "/package/"
	body, err := c.accept(ctx, base+url.PathEscape(t.Package)+"/preferred", "application/json")
	if err != nil {
		return nil, err
	}
	var pref struct {
		Normal []string `json:"normal-version"`
	}
	if err := json.Unmarshal(body, &pref); err != nil {
		return nil, err
	}
	version := ""
	for _, v := range pref.Normal {
		if v == strings.TrimSpace(t.Version) {
			version = v
			break
		}
		if version == "" || compareVersions(v, version) > 0 {
			version = v
		}
	}
	if version == "" {
		return nil, nil
	}
	id := url.PathEscape(t.Package + "-" + version)
	body, err = c.accept(ctx, base+id+"/"+url.PathEscape(t.Package)+".cabal", "text/plain")
	if err != nil {
		return nil, err
	}
	return cabalLibraryDepends(body, t.Package), nil
}

// compareVersions orders two dotted numeric versions (1.10 after 1.9).
func compareVersions(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var x, y int
		if i < len(as) {
			x, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			y, _ = strconv.Atoi(bs[i])
		}
		if x != y {
			return x - y
		}
	}
	return 0
}

// sublibs is a dependency's list of sublibraries: pkg:{a, b}.
var sublibs = regexp.MustCompile(`:\s*\{[^}]*\}`)

// cabalLibraryDepends reads the build-depends of a .cabal file's library stanzas
// (named sublibraries too) and of the common stanzas they import. A requirement
// written ==1.2.3 is the bare version, as the plugin writes a pin.
func cabalLibraryDepends(src []byte, self string) []dep {
	type stanza struct {
		kind, name string
		deps       []string
		imports    []string
	}
	var stanzas []*stanza
	var cur *stanza
	field, fieldIndent := "", 0
	for _, line := range strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if indent == 0 {
			kind, name, _ := strings.Cut(strings.ToLower(trimmed), " ")
			cur = &stanza{kind: kind, name: strings.TrimSpace(name)}
			stanzas = append(stanzas, cur)
			field = ""
			continue
		}
		if cur == nil {
			continue
		}
		if field != "" && indent > fieldIndent {
			if field == "build-depends" {
				cur.deps = append(cur.deps, trimmed)
			}
			continue
		}
		field = ""
		k, v, ok := strings.Cut(trimmed, ":")
		if !ok || strings.ContainsAny(k, " (") {
			continue // an if/else line
		}
		switch k = strings.ToLower(strings.TrimSpace(k)); k {
		case "build-depends":
			field, fieldIndent = k, indent
			cur.deps = append(cur.deps, v)
		case "import":
			cur.imports = append(cur.imports, strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' })...)
		}
	}
	commons := map[string]*stanza{}
	for _, s := range stanzas {
		if s.kind == "common" {
			commons[s.name] = s
		}
	}
	var out []dep
	seen := map[string]bool{}
	var add func(s *stanza, via map[string]bool)
	add = func(s *stanza, via map[string]bool) {
		for _, entry := range strings.Split(sublibs.ReplaceAllString(strings.Join(s.deps, ","), ""), ",") {
			entry = strings.Join(strings.Fields(entry), " ")
			i := strings.IndexAny(entry, " <>=^:")
			name, req := entry, ""
			if i >= 0 {
				name, req = entry[:i], strings.TrimSpace(entry[i:])
			}
			if strings.HasPrefix(req, ":") { // pkg:sublib or pkg:{a, b}
				req = strings.TrimSpace(strings.TrimLeft(req[strings.IndexAny(req+" ", " <>=^"):], " "))
			}
			if name == "" || name == self || hackageStd[name] || seen[name] {
				continue
			}
			seen[name] = true
			if v, ok := strings.CutPrefix(req, "=="); ok && lang.Pinned(strings.TrimSpace(v)) {
				req = strings.TrimSpace(v)
			}
			if req == "-any" || req == ">=0" {
				req = ""
			}
			out = append(out, dep{Name: name, Version: req})
		}
		for _, name := range s.imports {
			if cm := commons[name]; cm != nil && !via[name] {
				via[name] = true
				add(cm, via)
			}
		}
	}
	for _, s := range stanzas {
		if s.kind == "library" {
			add(s, map[string]bool{})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ---------------------------------------------------------------- Terraform Registry

// terraformModule reads what a registry module requires from the Terraform module
// registry protocol: <modules.v1>/<namespace>/<name>/<provider>/versions lists every
// published version with the providers and modules its root module (and each
// submodule) declares. The version asked for is taken when the target names one,
// else the newest release its constraint allows. The modules.v1 path is found by
// service discovery (/.well-known/terraform.json) on any registry but the public one.
// Providers depend on nothing, so only modules are asked about.
//
// Implements: REQ-SUP-050
func (c *Client) terraformModule(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	addr, sub, _ := strings.Cut(t.Package, "//")
	parts := strings.Split(addr, "/")
	if len(parts) == 4 {
		parts = parts[1:]
	}
	if len(parts) != 3 {
		return nil, fmt.Errorf("%s: not a registry module address", t.Package)
	}
	base, err := c.terraformService(ctx, index, "modules.v1")
	if err != nil {
		return nil, err
	}
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	body, err := c.get(ctx, base+strings.Join(parts, "/")+"/versions")
	if err != nil {
		return nil, err
	}
	type module struct {
		Path      string `json:"path"`
		Providers []struct {
			Name, Namespace, Source, Version string
		} `json:"providers"`
		Dependencies []struct {
			Name, Source, Version string
		} `json:"dependencies"`
	}
	var doc struct {
		Modules []struct {
			Versions []struct {
				Version    string   `json:"version"`
				Root       module   `json:"root"`
				Submodules []module `json:"submodules"`
			} `json:"versions"`
		} `json:"modules"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	if len(doc.Modules) == 0 {
		return nil, nil
	}
	versions := doc.Modules[0].Versions
	want := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(t.Version), "="))
	chosen := -1
	for i, v := range versions {
		if v.Version == want {
			chosen = i
			break
		}
		if strings.Contains(v.Version, "-") || !terraformAllows(t.Version, v.Version) {
			continue // a pre-release is only taken by name
		}
		if chosen < 0 || compareVersions(v.Version, versions[chosen].Version) > 0 {
			chosen = i
		}
	}
	if chosen < 0 {
		return nil, nil
	}
	m := versions[chosen].Root
	if sub = strings.Trim(sub, "/"); sub != "" {
		m = module{}
		for _, s := range versions[chosen].Submodules {
			if strings.Trim(s.Path, "/") == sub {
				m = s
			}
		}
	}
	var out []dep
	seen := map[string]bool{}
	for _, p := range m.Providers {
		src := p.Source
		if src == "" {
			ns := p.Namespace
			if ns == "" {
				ns = "hashicorp"
			}
			src = ns + "/" + p.Name
		}
		src = terraformName(src)
		if src == "" || strings.HasPrefix(src, "terraform.io/builtin/") || seen["p "+src] {
			continue
		}
		seen["p "+src] = true
		out = append(out, dep{Name: src, Version: terraformVersion(p.Version), Eco: "terraform-provider"})
	}
	for _, d := range m.Dependencies {
		name := terraformName(d.Source)
		if name == "" || seen["m "+name] || strings.Count(strings.SplitN(name, "//", 2)[0], "/") < 2 {
			continue // local paths and git sources name no registry module
		}
		seen["m "+name] = true
		out = append(out, dep{Name: name, Version: terraformVersion(d.Version)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Eco+out[i].Name < out[j].Eco+out[j].Name })
	return out, nil
}

// terraformService finds where a registry serves one of its services, as Terraform
// does: the public registry's are known, any other host's are read from its
// /.well-known/terraform.json (a path relative to the host, or a URL). The answer
// is remembered per registry.
func (c *Client) terraformService(ctx context.Context, index, service string) (string, error) {
	index = strings.TrimRight(index, "/")
	if index == public[TerraformModule] {
		return index + "/v1/modules/", nil
	}
	key := "terraform " + index + " " + service
	c.mu.Lock()
	base, ok := c.feeds[key]
	c.mu.Unlock()
	if ok {
		return base, nil
	}
	body, err := c.get(ctx, index+"/.well-known/terraform.json")
	if err != nil {
		return "", err
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", err
	}
	s, _ := doc[service].(string)
	if s == "" {
		return "", fmt.Errorf("%s does not serve %s", index, service)
	}
	ref, err := url.Parse(s)
	if err != nil {
		return "", err
	}
	root, err := url.Parse(index + "/")
	if err != nil {
		return "", err
	}
	base = root.ResolveReference(ref).String()
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	c.mu.Lock()
	c.feeds[key] = base
	c.mu.Unlock()
	return base, nil
}

// terraformName normalizes a registry address the way the Terraform plugin names
// it: lower case, the public Terraform and OpenTofu registries' hosts dropped. Local
// paths and addresses go-getter fetches are not registry addresses: "".
func terraformName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" || strings.Contains(s, "::") || strings.Contains(s, "://") || strings.HasPrefix(s, ".") ||
		strings.HasPrefix(s, "github.com/") || strings.HasPrefix(s, "bitbucket.org/") || strings.Contains(s, "?") {
		return ""
	}
	for _, host := range []string{"registry.terraform.io/", "registry.opentofu.org/"} {
		s = strings.TrimPrefix(s, host)
	}
	return s
}

// terraformVersion writes a constraint allowing one version as that version, as
// the plugin does ("= 1.2.3" is 1.2.3).
func terraformVersion(c string) string {
	v := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(c), "="))
	if lang.Pinned(v) && !strings.ContainsAny(v, ", ") {
		return v
	}
	return strings.TrimSpace(c)
}

// terraformAllows reports whether version v meets a Terraform version constraint:
// comma-separated =, !=, >, >=, <, <= and ~> (the rightmost given part may grow:
// "~> 1.2" is >= 1.2, < 2.0; "~> 1.2.0" is >= 1.2.0, < 1.3.0). No constraint allows
// every version.
func terraformAllows(constraint, v string) bool {
	for _, c := range strings.Split(constraint, ",") {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		op := ""
		for _, o := range []string{"~>", ">=", "<=", "!=", ">", "<", "="} {
			if strings.HasPrefix(c, o) {
				op, c = o, strings.TrimSpace(c[len(o):])
				break
			}
		}
		cmp := compareVersions(v, c)
		ok := true
		switch op {
		case "", "=":
			ok = cmp == 0
		case "!=":
			ok = cmp != 0
		case ">":
			ok = cmp > 0
		case ">=":
			ok = cmp >= 0
		case "<":
			ok = cmp < 0
		case "<=":
			ok = cmp <= 0
		case "~>":
			parts := strings.Split(c, ".")
			if len(parts) > 1 {
				parts = parts[:len(parts)-1]
			}
			n, _ := strconv.Atoi(parts[len(parts)-1])
			parts[len(parts)-1] = strconv.Itoa(n + 1)
			ok = cmp >= 0 && compareVersions(v, strings.Join(parts, ".")) < 0
		}
		if !ok {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------- OCI

// The media types a registry may answer a manifest request with: the OCI ones and
// Docker's older equivalents, single images and the indexes that point at them.
const ociAccept = "application/vnd.oci.image.manifest.v1+json," +
	"application/vnd.oci.image.index.v1+json," +
	"application/vnd.docker.distribution.manifest.v2+json," +
	"application/vnd.docker.distribution.manifest.list.v2+json"

// ociBase resolves the image an image was built on.
//
// A container image has no dependency list. What it has, when whoever built it said
// so, is a base image - an annotation on the manifest or a label in the config blob -
// and that is the image whose vulnerabilities this one inherits, which is what makes
// an image on the map more than a leaf. Two or three requests, no layers: the
// manifest, one more for a multi-platform index, and the config blob.
//
// Implements: REQ-SUP-026
func (c *Client) ociBase(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	repo := ociRepository(t.Package)
	ref := t.Version
	if ref == "" {
		ref = "latest"
	}
	manifest, err := c.ociFetch(ctx, index, repo, ref, ociAccept)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Config struct {
			Digest string `json:"digest"`
		} `json:"config"`
		Annotations map[string]string `json:"annotations"`
		Manifests   []struct {
			Digest   string `json:"digest"`
			Platform struct {
				OS   string `json:"os"`
				Arch string `json:"architecture"`
			} `json:"platform"`
		} `json:"manifests"`
	}
	if err := json.Unmarshal(manifest, &doc); err != nil {
		return nil, err
	}
	if base := ociBaseOf(doc.Annotations); base.Name != "" {
		return []dep{base}, nil
	}
	// A multi-platform index points at one manifest per platform, and the base image
	// is the same in all of them: whichever comes first will do, with linux/amd64
	// preferred only because it is the one most likely to exist.
	if doc.Config.Digest == "" && len(doc.Manifests) > 0 {
		pick := doc.Manifests[0].Digest
		for _, m := range doc.Manifests {
			if m.Platform.OS == "linux" && m.Platform.Arch == "amd64" {
				pick = m.Digest
				break
			}
		}
		if manifest, err = c.ociFetch(ctx, index, repo, pick, ociAccept); err != nil {
			return nil, err
		}
		doc.Config.Digest, doc.Annotations, doc.Manifests = "", nil, nil
		if err := json.Unmarshal(manifest, &doc); err != nil {
			return nil, err
		}
		if base := ociBaseOf(doc.Annotations); base.Name != "" {
			return []dep{base}, nil
		}
	}
	if doc.Config.Digest == "" {
		return nil, nil
	}
	blob, err := c.ociBlob(ctx, index, repo, doc.Config.Digest)
	if err != nil {
		return nil, err
	}
	var config struct {
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"config"`
	}
	if err := json.Unmarshal(blob, &config); err != nil {
		return nil, err
	}
	if base := ociBaseOf(config.Config.Labels); base.Name != "" {
		return []dep{base}, nil
	}
	return nil, nil
}

// ociBaseOf reads the standard base-image keys out of annotations or labels. The
// digest is preferred over the tag where both are given: a tag moves, a digest is the
// image that was actually built on.
func ociBaseOf(kv map[string]string) dep {
	name := strings.TrimSpace(kv["org.opencontainers.image.base.name"])
	if name == "" {
		return dep{}
	}
	image, tag := name, ""
	if i := strings.LastIndex(name, "@"); i > 0 {
		image, tag = name[:i], name[i+1:]
	} else if i := strings.LastIndex(name, ":"); i > 0 && !strings.Contains(name[i:], "/") {
		image, tag = name[:i], name[i+1:]
	}
	if digest := strings.TrimSpace(kv["org.opencontainers.image.base.digest"]); digest != "" {
		tag = digest
	}
	if tag == "" {
		tag = "latest"
	}
	return dep{Name: image, Version: tag}
}

// ociRepository is the path part of an image reference, as a registry's API wants it:
// a Docker Hub image with no namespace lives under "library".
func ociRepository(image string) string {
	first, rest, ok := strings.Cut(image, "/")
	if !ok {
		return "library/" + image
	}
	if strings.Contains(first, ".") || strings.Contains(first, ":") || first == "localhost" {
		return rest // the first segment was the registry
	}
	return image
}

func (c *Client) ociFetch(ctx context.Context, index, repo, ref, accept string) ([]byte, error) {
	return c.ociGet(ctx, index, repo, fmt.Sprintf("%s/v2/%s/manifests/%s", index, repo, ref), accept)
}

func (c *Client) ociBlob(ctx context.Context, index, repo, digest string) ([]byte, error) {
	return c.ociGet(ctx, index, repo, fmt.Sprintf("%s/v2/%s/blobs/%s", index, repo, digest), "application/json, */*")
}

// ociGet performs one registry request, answering the pull-token challenge a registry
// sends when it will not serve anonymously without one. Only the token the registry
// itself points at is asked for, and it is used for this request alone.
func (c *Client) ociGet(ctx context.Context, index, repo, address, accept string) ([]byte, error) {
	resp, err := c.do(ctx, address, accept, "")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		challenge := resp.Header.Get("Www-Authenticate")
		resp.Body.Close()
		token, err := c.ociToken(ctx, index, repo, challenge)
		if err != nil {
			return nil, err
		}
		if token == "" {
			// No challenge this client can answer: the registry refused.
			return nil, fmt.Errorf("%s: %s", address, http.StatusText(http.StatusUnauthorized))
		}
		if resp, err = c.do(ctx, address, accept, token); err != nil {
			return nil, err
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", address, resp.Status)
	}
	return readLimited(resp)
}

// ociToken asks for the pull token a registry's challenge describes.
//
// A challenge is a header, and a header names wherever it likes, so the realm is
// checked before it is followed: either https, or the registry this request was
// already going to - which keeps a registry on a private plain-http network working
// without letting a plain-http challenge redirect the request elsewhere. Credentials
// are chosen by the realm's own host (credentials.apply), so a redirected realm gets
// none of the registry's.
//
// Implements: REQ-SUP-027
func (c *Client) ociToken(ctx context.Context, index, repo, challenge string) (string, error) {
	scheme, params, ok := strings.Cut(challenge, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", nil
	}
	fields := map[string]string{}
	for _, part := range splitChallenge(params) {
		if k, v, ok := strings.Cut(part, "="); ok {
			fields[strings.ToLower(strings.TrimSpace(k))] = strings.Trim(strings.TrimSpace(v), `"`)
		}
	}
	realm := fields["realm"]
	if realm == "" {
		return "", nil
	}
	u, err := url.Parse(realm)
	if err != nil {
		return "", nil
	}
	if registry, err := url.Parse(index); u.Scheme != "https" && (err != nil || u.Host != registry.Host) {
		return "", nil
	}
	q := u.Query()
	if s := fields["service"]; s != "" {
		q.Set("service", s)
	}
	scope := fields["scope"]
	if scope == "" {
		scope = "repository:" + repo + ":pull"
	}
	q.Set("scope", scope)
	u.RawQuery = q.Encode()
	body, err := c.get(ctx, u.String())
	if err != nil {
		return "", err
	}
	var doc struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", err
	}
	if doc.Token != "" {
		return doc.Token, nil
	}
	return doc.AccessToken, nil
}

// splitChallenge splits a challenge's comma-separated parameters, leaving the commas
// that are inside a quoted scope where they are.
func splitChallenge(params string) []string {
	var out []string
	quoted, start := false, 0
	for i, r := range params {
		switch {
		case r == '"':
			quoted = !quoted
		case r == ',' && !quoted:
			out = append(out, params[start:i])
			start = i + 1
		}
	}
	return append(out, params[start:])
}

// ---------------------------------------------------------------- CocoaPods

// cocoapodsPod reads a pod's dependencies from a CocoaPods CDN (the trunk spec
// repository served as files, https://cdn.cocoapods.org). Pods are sharded by the
// MD5 of their name: all_pods_versions_<a>_<b>_<c>.txt lists "Name/1.0/1.1" per
// pod, and Specs/<a>/<b>/<c>/<Name>/<version>/<Name>.podspec.json is one release's
// specification. The release asked for is read when the version is exact, else the
// newest one that is not a pre-release and that the requirement allows (else the
// newest at all). Its dependencies are the root spec's and
// those of its default subspecs (all subspecs when none is named), without test
// specs and the pod's own subspecs; "= 1.2.3" is the bare, pinned version.
//
// Implements: REQ-SUP-051
func (c *Client) cocoapodsPod(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	sum := md5.Sum([]byte(t.Package))
	h := hex.EncodeToString(sum[:])
	shard := []string{h[0:1], h[1:2], h[2:3]}
	base := strings.TrimRight(index, "/")
	version := strings.TrimSpace(t.Version)
	if !lang.Pinned(version) {
		body, err := c.accept(ctx, base+"/all_pods_versions_"+strings.Join(shard, "_")+".txt", "text/plain")
		if err != nil {
			return nil, err
		}
		constraint := version
		version = ""
		newest := ""
		for _, line := range strings.Split(string(body), "\n") {
			fields := strings.Split(strings.TrimSpace(line), "/")
			if fields[0] != t.Package {
				continue
			}
			for _, v := range fields[1:] {
				if strings.Contains(v, "-") {
					continue // a pre-release
				}
				if newest == "" || compareVersions(v, newest) > 0 {
					newest = v
				}
				// CocoaPods' operators are Terraform's: ~>, >=, <, =, !=.
				if terraformAllows(constraint, v) && (version == "" || compareVersions(v, version) > 0) {
					version = v
				}
			}
		}
		if version == "" {
			version = newest
		}
		if version == "" {
			return nil, nil
		}
	}
	esc := url.PathEscape(t.Package)
	body, err := c.get(ctx, base+"/Specs/"+strings.Join(shard, "/")+"/"+esc+"/"+url.PathEscape(version)+"/"+esc+".podspec.json")
	if err != nil {
		return nil, err
	}
	var spec podSpec
	if err := json.Unmarshal(body, &spec); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []dep
	var walk func(s podSpec)
	walk = func(s podSpec) {
		for _, name := range slices.Sorted(maps.Keys(s.Dependencies)) {
			root, _, _ := strings.Cut(name, "/")
			if root == t.Package || seen[root] {
				continue
			}
			seen[root] = true
			req := strings.Join(s.Dependencies[name], ", ")
			if v, ok := strings.CutPrefix(req, "= "); ok && lang.Pinned(v) {
				req = v
			}
			out = append(out, dep{Name: root, Version: req})
		}
	}
	walk(spec)
	defaults := map[string]bool{}
	for _, d := range spec.defaults() {
		defaults[d] = true
	}
	for _, sub := range spec.Subspecs {
		if len(defaults) == 0 || defaults[sub.Name] {
			walk(sub)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// podSpec is the part of a podspec.json read for dependencies.
type podSpec struct {
	Name         string              `json:"name"`
	Dependencies map[string][]string `json:"dependencies"`
	Subspecs     []podSpec           `json:"subspecs"`
	// DefaultSubspecs is a name or a list of names.
	DefaultSubspecs json.RawMessage `json:"default_subspecs"`
}

func (s podSpec) defaults() []string {
	var list []string
	if json.Unmarshal(s.DefaultSubspecs, &list) == nil {
		return list
	}
	var one string
	if json.Unmarshal(s.DefaultSubspecs, &one) == nil && one != "" {
		return []string{one}
	}
	return nil
}

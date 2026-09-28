package index

import (
	"archive/zip"
	"bytes"
	"cmp"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/ada"
	"github.com/sarumaj/depphunter-cli/internal/lang/juliapkg"
	"github.com/sarumaj/depphunter-cli/internal/lang/luarocks"
	"github.com/sarumaj/depphunter-cli/internal/lang/opam"
	"github.com/sarumaj/depphunter-cli/internal/lang/starlark"
	"github.com/sarumaj/depphunter-cli/internal/store"
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
	if v := strings.TrimSpace(t.Version); lang.Pinned(v) {
		// A failure here falls back to the current release below.
		body, _ = c.get(ctx, base+"/"+url.PathEscape(v))
	}
	if body == nil {
		// No release named, or crandb does not have it: the current one answers.
		var err error
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
	walk := func(s podSpec) {
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

// ---------------------------------------------------------------- LuaRocks

// luarocksRock reads a rock's dependencies from a rocks server, as the luarocks
// client does: the server's manifest (manifest-5.1.zip, else manifest-5.1, read once
// per server) lists every rock's versions, and <server>/<rock>-<version>.rockspec is
// one release's rockspec. A locked version with its revision ("1.14.0-3") is fetched
// as it is; otherwise the newest release the constraint allows (an exact version
// being == that version) is taken from the manifest. The dependencies are the
// rockspec's run-time ones, every platform's, without lua itself; "== 1.2" is the
// bare, pinned version.
//
// Implements: REQ-SUP-052
func (c *Client) luarocksRock(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	base := strings.TrimRight(index, "/")
	version := strings.TrimSpace(t.Version)
	if !t.Pinned || !strings.Contains(version, "-") {
		versions, err := c.rocksManifest(ctx, base)
		if err != nil {
			return nil, err
		}
		constraint := version
		if t.Pinned {
			constraint = "== " + version
		}
		if version = luarocks.Newest(versions[strings.ToLower(t.Package)], constraint); version == "" {
			return nil, nil // not on this server, or nothing it serves is allowed
		}
	}
	body, err := c.accept(ctx, base+"/"+url.PathEscape(strings.ToLower(t.Package)+"-"+version)+".rockspec", "text/plain, */*")
	if err != nil {
		return nil, err
	}
	var out []dep
	seen := map[string]bool{}
	for _, d := range luarocks.ReadRockspec(body).Deps {
		if d.Section != "dependencies" || d.Name == "lua" || seen[d.Name] {
			continue
		}
		seen[d.Name] = true
		v := d.Constraint
		if exact, ok := luarocks.Exact(v); ok {
			v = exact
		}
		out = append(out, dep{Name: d.Name, Version: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// maxManifest bounds a rocks server manifest once unzipped; luarocks.org's is a few
// megabytes.
const maxManifest = 64 << 20

// rocksManifest reads a rocks server's manifest once: the zipped one the luarocks
// client prefers, else the plain one.
func (c *Client) rocksManifest(ctx context.Context, base string) (map[string][]string, error) {
	c.mu.Lock()
	m, ok := c.rocks[base]
	c.mu.Unlock()
	if ok {
		return m, nil
	}
	var src []byte
	if zipped, err := c.accept(ctx, base+"/manifest-5.1.zip", "application/zip, */*"); err == nil {
		if zr, err := zip.NewReader(bytes.NewReader(zipped), int64(len(zipped))); err == nil {
			for _, f := range zr.File {
				if f.Name != "manifest-5.1" {
					continue
				}
				if rc, err := f.Open(); err == nil {
					src, _ = io.ReadAll(io.LimitReader(rc, maxManifest))
					rc.Close()
				}
			}
		}
	}
	if src == nil {
		plain, err := c.accept(ctx, base+"/manifest-5.1", "text/plain, */*")
		if err != nil {
			return nil, err
		}
		src = plain
	}
	m = luarocks.ReadManifest(src)
	c.mu.Lock()
	c.rocks[base] = m
	c.mu.Unlock()
	return m, nil
}

// ---------------------------------------------------------------- CPAN

// cpanDistribution reads a distribution's dependencies from the MetaCPAN API:
// <api>/v1/release/<distribution> is its latest release, whose `dependency` list
// names modules by phase and relationship. The run-time requirements are kept,
// without perl itself; each module becomes the distribution
// <api>/v1/module/<module> says provides it (asked once per module and cached),
// and a module only perl provides is left out. A version is a minimum, shown as
// ">= 1.2" and never pinned.
//
// Implements: REQ-SUP-053
func (c *Client) cpanDistribution(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	base := strings.TrimRight(index, "/")
	body, err := c.get(ctx, base+"/v1/release/"+url.PathEscape(t.Package))
	if err != nil {
		return nil, err
	}
	var rel struct {
		Distribution string `json:"distribution"`
		Dependency   []struct {
			Module       string `json:"module"`
			Version      any    `json:"version"`
			Phase        string `json:"phase"`
			Relationship string `json:"relationship"`
		} `json:"dependency"`
	}
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, err
	}
	var out []dep
	seen := map[string]bool{t.Package: true, rel.Distribution: true}
	for _, d := range rel.Dependency {
		if d.Phase != "runtime" || d.Relationship != "requires" || d.Module == "perl" {
			continue
		}
		dist, err := c.cpanModule(ctx, base, d.Module)
		if err != nil {
			return nil, err
		}
		if dist == "" || dist == "perl" || seen[dist] {
			continue // perl's own module, or one already answered
		}
		seen[dist] = true
		v := strings.TrimSpace(fmt.Sprint(d.Version))
		if d.Version == nil || strings.Trim(v, "0.") == "" {
			v = ""
		} else if v[0] >= '0' && v[0] <= '9' || v[0] == 'v' {
			v = ">= " + v
		}
		out = append(out, dep{Name: dist, Version: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// cpanModule is the distribution providing a module, as MetaCPAN's module endpoint
// says; "" for a module it does not know.
func (c *Client) cpanModule(ctx context.Context, base, module string) (string, error) {
	key := "cpan-module|" + base + "|" + module
	c.mu.Lock()
	dist, ok := c.cpanModules[key]
	c.mu.Unlock()
	if ok {
		return dist, nil
	}
	if d, ok := store.Get[string](c.cache, key); ok {
		return d, nil
	}
	resp, err := c.do(ctx, base+"/v1/module/"+url.PathEscape(module), "application/json", "")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		body, err := readLimited(resp)
		if err != nil {
			return "", err
		}
		var m struct {
			Distribution string `json:"distribution"`
		}
		if err := json.Unmarshal(body, &m); err != nil {
			return "", err
		}
		dist = m.Distribution
	case http.StatusNotFound:
	default:
		return "", fmt.Errorf("%s/v1/module/%s: %s", base, module, resp.Status)
	}
	c.mu.Lock()
	c.cpanModules[key] = dist
	c.mu.Unlock()
	c.cache.Put(key, dist)
	return dist, nil
}

// ---------------------------------------------------------------- opam

// opamPackage reads a package's dependencies from an opam repository laid out as
// files: <repository>/packages/<name>/<name>.<version>/opam, which needs the
// version (a pinned one; a range is not asked about). The dependencies are its
// depends without the compiler and what only tests, documentation or development
// need; a dependency written {= "1.2"} is that version, else its constraint as
// written.
//
// Implements: REQ-SUP-054
func (c *Client) opamPackage(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	name, version := url.PathEscape(t.Package), url.PathEscape(strings.TrimSpace(t.Version))
	body, err := c.accept(ctx, strings.TrimRight(index, "/")+"/packages/"+name+"/"+name+"."+version+"/opam", "text/plain")
	if err != nil {
		return nil, err
	}
	var out []dep
	seen := map[string]bool{t.Package: true}
	for _, d := range opam.Read(body).Depends {
		if seen[d.Name] || opam.Compiler(d.Name) || slices.ContainsFunc(d.Flags, func(f string) bool {
			return f == "with-test" || f == "with-doc" || f == "with-dev-setup" || f == "dev"
		}) {
			continue
		}
		seen[d.Name] = true
		v := d.Exact
		if v == "" {
			v = d.Constraint
		}
		out = append(out, dep{Name: d.Name, Version: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ---------------------------------------------------------------- julia

// juliaPackage reads a package's dependencies from a Julia registry's files:
// <registry>/<dir>/Versions.toml for the version (the one pinned, else the newest a
// [compat] range admits, else the newest not yanked), then Deps.toml and Compat.toml,
// whose sections are keyed by the version ranges they hold for ("0.21.2 - 0"). A
// dependency's version is its compat range for that release, in the registry's
// notation, where a whole "1.2.3" is that release; julia itself is not a dependency,
// and Julia's standard libraries are julia-std.
//
// Implements: REQ-SUP-055
func (c *Client) juliaPackage(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	base := strings.TrimRight(index, "/")
	dir, err := c.juliaDir(ctx, base, t.Package)
	if err != nil || dir == "" {
		return nil, err
	}
	dir = base + "/" + dir + "/"
	body, err := c.accept(ctx, dir+"Versions.toml", "text/plain")
	if err != nil {
		return nil, err
	}
	var versions map[string]struct {
		Yanked bool `toml:"yanked"`
	}
	if _, err := toml.Decode(string(body), &versions); err != nil {
		return nil, err
	}
	var listed []string
	for v, meta := range versions {
		if !meta.Yanked {
			listed = append(listed, v)
		}
	}
	version := strings.TrimSpace(t.Version)
	if _, ok := versions[version]; !ok {
		ranges, _ := juliapkg.CompatRanges(version)
		if version = juliapkg.Newest(listed, ranges); version == "" {
			version = juliapkg.Newest(listed, nil)
		}
	}
	v, ok := juliapkg.ParseVersion(version)
	if !ok {
		return nil, nil
	}
	body, err = c.accept(ctx, dir+"Deps.toml", "text/plain")
	if err != nil {
		return nil, nil // a package without dependencies has no Deps.toml
	}
	deps := juliaSections(body, v)
	compat := map[string]string{}
	if body, err := c.accept(ctx, dir+"Compat.toml", "text/plain"); err == nil {
		for name, value := range juliaSections(body, v) {
			compat[name] = juliaCompat(value)
		}
	}
	var out []dep
	for name := range deps {
		if name == "julia" {
			continue
		}
		d := dep{Name: name, Version: compat[name]}
		if juliapkg.Stdlib(name) {
			d = dep{Name: name, Eco: "julia-std"}
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// juliaSections merges the sections of a Deps.toml or Compat.toml whose range key
// holds for a version.
func juliaSections(body []byte, v juliapkg.Version) map[string]any {
	var doc map[string]map[string]any
	if _, err := toml.Decode(string(body), &doc); err != nil {
		return nil
	}
	out := map[string]any{}
	for key, section := range doc {
		if r, ok := juliapkg.RegistryRange(key); ok && r.Contains(v) {
			for name, value := range section {
				out[name] = value
			}
		}
	}
	return out
}

// juliaCompat writes a Compat.toml value - a range or a list of them, "0.1-0.3"
// compressed - as one string, hyphen ranges spaced so that no range reads as a
// pre-release: ["0.1-0.3", "1"] is "0.1 - 0.3, 1".
func juliaCompat(value any) string {
	var parts []string
	switch v := value.(type) {
	case string:
		parts = []string{v}
	case []any:
		for _, x := range v {
			if s, ok := x.(string); ok {
				parts = append(parts, s)
			}
		}
	}
	for i, p := range parts {
		if lo, hi, ok := strings.Cut(p, "-"); ok && !strings.Contains(p, " - ") {
			parts[i] = strings.TrimSpace(lo) + " - " + strings.TrimSpace(hi)
		}
	}
	return strings.Join(parts, ", ")
}

// juliaDir is where a registry keeps a package's files. The General registry's
// layout is known (J/JSON); another registry's Registry.toml lists each package's
// path, read once per registry.
func (c *Client) juliaDir(ctx context.Context, base, name string) (string, error) {
	if base == public[Julia] {
		return juliapkg.RegistryDir(name), nil
	}
	c.mu.Lock()
	dirs, ok := c.juliaDirs[base]
	c.mu.Unlock()
	if !ok {
		body, err := c.accept(ctx, base+"/Registry.toml", "text/plain")
		if err != nil {
			return "", err
		}
		dirs = juliaRegistryPaths(body)
		c.mu.Lock()
		c.juliaDirs[base] = dirs
		c.mu.Unlock()
	}
	return dirs[name], nil
}

// juliaRegistry is what a Registry.toml says: the registry's name and repository,
// and each package's name and directory, keyed by UUID.
type juliaRegistry struct {
	Name     string `toml:"name"`
	Repo     string `toml:"repo"`
	Packages map[string]struct {
		Name string `toml:"name"`
		Path string `toml:"path"`
	} `toml:"packages"`
}

// juliaRegistryPaths maps each package of a Registry.toml to its directory.
func juliaRegistryPaths(body []byte) map[string]string {
	var reg juliaRegistry
	out := map[string]string{}
	if _, err := toml.Decode(string(body), &reg); err != nil {
		return out
	}
	for _, p := range reg.Packages {
		out[p.Name] = p.Path
	}
	return out
}

// ---------------------------------------------------------------- Maven

// mavenPOM is what a POM says about a release's dependencies: its own, its parent's
// (inherited), the versions dependencyManagement fixes and the properties both use.
type mavenPOM struct {
	GroupID string `xml:"groupId"`
	Version string `xml:"version"`
	Parent  struct {
		GroupID    string `xml:"groupId"`
		ArtifactID string `xml:"artifactId"`
		Version    string `xml:"version"`
	} `xml:"parent"`
	Properties struct {
		Items []struct {
			XMLName xml.Name
			Value   string `xml:",chardata"`
		} `xml:",any"`
	} `xml:"properties"`
	Deps    []mavenDep `xml:"dependencies>dependency"`
	Managed []mavenDep `xml:"dependencyManagement>dependencies>dependency"`
}

type mavenDep struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Version    string `xml:"version"`
	Scope      string `xml:"scope"`
	Optional   string `xml:"optional"`
}

var mavenProperty = regexp.MustCompile(`\$\{([^}]+)\}`)

// mavenArtifact reads what a Maven artifact depends on from its POM. It is asked only
// of packages named group:artifact - which is how the Java, Kotlin, Scala, Clojure
// and Bazel plugins name them - as a POM is addressed by both. The version is the one pinned, else the release
// maven-metadata.xml names. The POM's dependencies in the compile and runtime scopes
// that are not optional are the answer, with those its parent POMs declare (read up
// to four levels), versions filled from dependencyManagement and properties along
// that chain; a version still unknown is left empty. When the index is Maven Central
// and the repository has a Clojure manifest, Clojars is asked after it, as Clojure's
// tools do.
//
// Implements: REQ-SUP-056, REQ-JAVA-010
func (c *Client) mavenArtifact(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	group, artifact, _ := strings.Cut(t.Package, ":")
	repos := []string{index}
	if index == public[Maven] && c.cfg.clojure {
		repos = append(repos, clojarsURL)
	}
	fetch := func(g, a, file string) ([]byte, string, error) {
		var last error
		for _, r := range repos {
			body, err := c.accept(ctx, fmt.Sprintf("%s/%s/%s/%s", r, strings.ReplaceAll(g, ".", "/"), a, file), "application/xml")
			if err == nil {
				return body, r, nil
			}
			last = err
		}
		return nil, "", last
	}
	version := t.Version
	if !lang.PinnedMaven(version) {
		body, _, err := fetch(group, artifact, "maven-metadata.xml")
		if err != nil {
			return nil, err
		}
		var meta struct {
			Versioning struct {
				Release  string   `xml:"release"`
				Latest   string   `xml:"latest"`
				Versions []string `xml:"versions>version"`
			} `xml:"versioning"`
		}
		if err := lang.UnmarshalXML(body, &meta); err != nil {
			return nil, err
		}
		v := meta.Versioning
		version = cmp.Or(v.Release, v.Latest)
		if version == "" && len(v.Versions) > 0 {
			version = v.Versions[len(v.Versions)-1]
		}
		if version == "" {
			return nil, fmt.Errorf("%s: no release in maven-metadata.xml", t.Package)
		}
	}
	props := map[string]string{}
	managed := map[string]string{}
	var deps []mavenDep
	g, a, v := group, artifact, version
	for level := 0; level < 5 && a != ""; level++ {
		body, repo, err := fetch(g, a, fmt.Sprintf("%s/%s-%s.pom", v, a, v))
		if err != nil {
			if level == 0 {
				return nil, err
			}
			break // a parent nobody serves: what the child says is still the answer
		}
		repos = []string{repo} // a parent lives where its child does
		var pom mavenPOM
		if err := lang.UnmarshalXML(body, &pom); err != nil {
			if level == 0 {
				return nil, err
			}
			break
		}
		setDefault := func(k, val string) {
			if _, ok := props[k]; !ok && val != "" {
				props[k] = val
			}
		}
		for _, p := range pom.Properties.Items {
			setDefault(p.XMLName.Local, strings.TrimSpace(p.Value))
		}
		if level == 0 {
			setDefault("project.version", cmp.Or(pom.Version, pom.Parent.Version))
			setDefault("project.groupId", cmp.Or(pom.GroupID, pom.Parent.GroupID))
			setDefault("version", cmp.Or(pom.Version, pom.Parent.Version))
		}
		setDefault("project.parent.version", pom.Parent.Version)
		for _, d := range pom.Managed {
			if k := d.GroupID + ":" + d.ArtifactID; managed[k] == "" && d.Scope != "import" {
				managed[k] = d.Version
			}
		}
		deps = append(deps, pom.Deps...)
		g, a, v = pom.Parent.GroupID, pom.Parent.ArtifactID, pom.Parent.Version
	}
	expand := func(s string) string {
		for i := 0; i < 5 && strings.Contains(s, "${"); i++ {
			s = mavenProperty.ReplaceAllStringFunc(s, func(m string) string {
				if val, ok := props[m[2:len(m)-1]]; ok {
					return val
				}
				return m
			})
		}
		if strings.Contains(s, "${") {
			return ""
		}
		return strings.TrimSpace(s)
	}
	var out []dep
	seen := map[string]bool{}
	for _, d := range deps {
		switch strings.TrimSpace(d.Scope) {
		case "", "compile", "runtime":
		default:
			continue // test, provided, system, import
		}
		name := expand(d.GroupID) + ":" + expand(d.ArtifactID)
		if strings.TrimSpace(d.Optional) == "true" || seen[name] || strings.HasPrefix(name, ":") || strings.HasSuffix(name, ":") {
			continue
		}
		seen[name] = true
		ver := d.Version
		if ver == "" {
			ver = managed[d.GroupID+":"+d.ArtifactID]
		}
		out = append(out, dep{Name: name, Version: expand(ver)})
	}
	return out, nil
}

// ---------------------------------------------------------------- Bazel

// bazelModule reads a module's dependencies from a Bazel registry laid out as files
// (the Bazel Central Registry, or one a .bazelrc names with --registry):
// <registry>/modules/<name>/<version>/MODULE.bazel, the version the target names,
// else the newest metadata.json lists that is not yanked. Its bazel_dep calls are
// the dependencies, without dev_dependency ones, each at the version it names:
// minimal version selection makes that the version the module needs at least.
//
// Implements: REQ-SUP-057
func (c *Client) bazelModule(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	base := strings.TrimRight(index, "/") + "/modules/" + url.PathEscape(t.Package) + "/"
	version := strings.TrimSpace(t.Version)
	if version == "" {
		body, err := c.get(ctx, base+"metadata.json")
		if err != nil {
			return nil, err
		}
		var meta struct {
			Versions       []string          `json:"versions"`
			YankedVersions map[string]string `json:"yanked_versions"`
		}
		if err := json.Unmarshal(body, &meta); err != nil {
			return nil, err
		}
		for i := len(meta.Versions) - 1; i >= 0; i-- { // listed oldest first
			if _, yanked := meta.YankedVersions[meta.Versions[i]]; !yanked {
				version = meta.Versions[i]
				break
			}
		}
		if version == "" {
			return nil, nil
		}
	}
	body, err := c.accept(ctx, base+url.PathEscape(version)+"/MODULE.bazel", "text/plain")
	if err != nil {
		return nil, err
	}
	var out []dep
	seen := map[string]bool{}
	for _, st := range starlark.Parse(body).Stmts {
		n := st.X
		if st.Def != "" || n.Callee() != "bazel_dep" {
			continue
		}
		name := n.KwStr("name")
		if dev := n.Kw("dev_dependency"); name == "" || seen[name] || dev != nil && dev.Name() == "True" {
			continue
		}
		seen[name] = true
		out = append(out, dep{Name: name, Version: n.KwStr("version")})
	}
	return out, nil
}

// ---------------------------------------------------------------- Elm

// elmPackage reads an Elm package's dependencies from package.elm-lang.org (or a
// server laid out like it): <server>/packages/<author>/<name>/<version>/elm.json
// for the version the target names, else the newest that releases.json lists and
// the target's range ("1.0.0 <= v < 2.0.0") admits. The package's dependencies
// are ranges, and are returned as written; test-dependencies are left out.
//
// Implements: REQ-SUP-058
func (c *Client) elmPackage(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	author, name, _ := strings.Cut(t.Package, "/")
	if !elmName.MatchString(t.Package) || strings.Contains(t.Package, "..") {
		return nil, fmt.Errorf("not an Elm package name: %q", t.Package)
	}
	base := strings.TrimRight(index, "/") + "/packages/" + url.PathEscape(author) + "/" + url.PathEscape(name) + "/"
	version := strings.TrimSpace(t.Version)
	if _, exact := elmVersion(version); !exact {
		body, err := c.get(ctx, base+"releases.json")
		if err != nil {
			return nil, err
		}
		var releases map[string]int64 // version -> publication time
		if err := json.Unmarshal(body, &releases); err != nil {
			return nil, err
		}
		lo, hi, ranged := elmRange(version)
		best, bestV := "", [3]int{}
		for v := range releases {
			parsed, ok := elmVersion(v)
			if ok && (!ranged || elmAdmits(lo, hi, parsed)) && (best == "" || elmLess(bestV, parsed)) {
				best, bestV = v, parsed
			}
		}
		if best == "" {
			return nil, nil
		}
		version = best
	}
	body, err := c.get(ctx, base+url.PathEscape(version)+"/elm.json")
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
	for n, v := range doc.Dependencies {
		out = append(out, dep{Name: n, Version: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// elmName is an Elm package name: author/name.
var elmName = regexp.MustCompile(`^[A-Za-z0-9_-]+/[A-Za-z0-9_.-]+$`)

// elmVersion parses an Elm package version, always major.minor.patch.
func elmVersion(s string) ([3]int, bool) {
	var v [3]int
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || p == "" || p[0] == '+' {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

func elmLess(a, b [3]int) bool { return slices.Compare(a[:], b[:]) < 0 }

// elmBound is one side of an Elm constraint; strict for `<`.
type elmBound struct {
	v      [3]int
	strict bool
}

// elmRange parses an Elm constraint, "1.0.0 <= v < 2.0.0" (either side < or <=).
func elmRange(s string) (lo, hi elmBound, ok bool) {
	f := strings.Fields(s)
	if len(f) != 5 || f[2] != "v" || f[1] != "<" && f[1] != "<=" || f[3] != "<" && f[3] != "<=" {
		return lo, hi, false
	}
	a, ok1 := elmVersion(f[0])
	b, ok2 := elmVersion(f[4])
	return elmBound{a, f[1] == "<"}, elmBound{b, f[3] == "<"}, ok1 && ok2
}

func elmAdmits(lo, hi elmBound, v [3]int) bool {
	if elmLess(v, lo.v) || lo.strict && v == lo.v {
		return false
	}
	return elmLess(v, hi.v) || !hi.strict && v == hi.v
}

// ---------------------------------------------------------------- PureScript

// purescriptPackage reads a PureScript registry package's dependencies from the
// registry's repositories as files: the manifest of the version the target
// names from registry-index/main/<shard>/<name> (one JSON manifest per line,
// sharded like crates.io's index: 1/, 2/, 3/<first letter>/, else the first two
// letters and the next two), else of the newest version published in
// registry/main/metadata/<name>.json that the target's range (">=6.0.0 <7.0.0")
// admits - the newest of all when it names a package set rather than a
// version. The dependencies are ranges, returned as written.
//
// Implements: REQ-SUP-059
func (c *Client) purescriptPackage(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	name := t.Package
	if !purescriptName.MatchString(name) {
		return nil, fmt.Errorf("not a PureScript registry package name: %q", name)
	}
	base := strings.TrimRight(index, "/")
	version := strings.TrimPrefix(strings.TrimSpace(t.Version), "v")
	if _, exact := purescriptVersion(version); !exact {
		body, err := c.get(ctx, base+"/registry/main/metadata/"+name+".json")
		if err != nil {
			return nil, err
		}
		var meta struct {
			Published map[string]json.RawMessage `json:"published"`
		}
		if err := json.Unmarshal(body, &meta); err != nil {
			return nil, err
		}
		best, bestV := "", [3]int{}
		for v := range meta.Published {
			parsed, ok := purescriptVersion(v)
			if ok && purescriptAdmits(t.Version, parsed) && (best == "" || elmLess(bestV, parsed)) {
				best, bestV = v, parsed
			}
		}
		if best == "" {
			return nil, nil
		}
		version = best
	}
	body, err := c.get(ctx, base+"/registry-index/main/"+purescriptShard(name)+"/"+name)
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(body), "\n") {
		var m struct {
			Version      string            `json:"version"`
			Dependencies map[string]string `json:"dependencies"`
		}
		if json.Unmarshal([]byte(line), &m) != nil || m.Version != version {
			continue
		}
		out := make([]dep, 0, len(m.Dependencies))
		for n, v := range m.Dependencies {
			out = append(out, dep{Name: n, Version: v})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
		return out, nil
	}
	return nil, nil
}

// purescriptName is a registry package name: lower-case letters, digits and
// single dashes.
var purescriptName = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// purescriptShard is the directory of a package's file in the registry index.
func purescriptShard(name string) string {
	switch len(name) {
	case 1, 2:
		return strconv.Itoa(len(name))
	case 3:
		return "3/" + name[:1]
	}
	return name[:2] + "/" + name[2:4]
}

// purescriptVersion parses a registry version, always major.minor.patch.
func purescriptVersion(s string) ([3]int, bool) { return elmVersion(s) }

// purescriptAdmits reports whether a version satisfies a registry range
// (">=6.0.0 <7.0.0"); anything that is not a range (a package set's name, no
// version) admits every version.
func purescriptAdmits(rng string, v [3]int) bool {
	f := strings.Fields(rng)
	if len(f) == 0 || !strings.ContainsAny(f[0], "<>=") {
		return true
	}
	for _, c := range f {
		op := strings.TrimRight(c, "0123456789.")
		b, ok := purescriptVersion(strings.TrimPrefix(c, op))
		if !ok {
			return true
		}
		n := slices.Compare(v[:], b[:])
		switch op {
		case ">=":
			ok = n >= 0
		case ">":
			ok = n > 0
		case "<":
			ok = n < 0
		case "<=":
			ok = n <= 0
		case "==", "=":
			ok = n == 0
		default:
			ok = true
		}
		if !ok {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------- dub

// dubPackage reads a dub package's dependencies from a dub registry's API
// (code.dlang.org by default): <registry>/api/packages/<name>/<version>/info for
// the version the target names, else <registry>/api/packages/<name>/info and the
// newest release the target's specification (~>0.9.5, ^1.2.0, >=1.0.0 <2.0.0,
// *) admits. A version's info is its recipe: the dependencies of the package, of
// its sub-packages and of its first (default) configuration are returned as the
// specifications they are, under their base package's name; optional ones,
// path dependencies and the package's own sub-packages are left out.
//
// Implements: REQ-SUP-060
func (c *Client) dubPackage(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	name := t.Package
	if !dubName.MatchString(name) {
		return nil, fmt.Errorf("not a dub package name: %q", name)
	}
	base := strings.TrimRight(index, "/") + "/api/packages/" + url.PathEscape(name)
	version := strings.TrimPrefix(strings.TrimSpace(t.Version), "==")
	var info map[string]json.RawMessage
	if _, exact := dubVersion(version); exact && !strings.Contains(version, "-") {
		body, err := c.get(ctx, base+"/"+url.PathEscape(version)+"/info")
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(body, &info); err != nil {
			return nil, err
		}
	} else {
		body, err := c.get(ctx, base+"/info")
		if err != nil {
			return nil, err
		}
		var pkg struct {
			Versions []map[string]json.RawMessage `json:"versions"`
		}
		if err := json.Unmarshal(body, &pkg); err != nil {
			return nil, err
		}
		best, bestV := -1, [3]int{}
		for i, v := range pkg.Versions {
			var s string
			json.Unmarshal(v["version"], &s)
			parsed, ok := dubVersion(s)
			if !ok || strings.ContainsAny(s, "-+") && s != version || !dubAdmits(version, parsed) {
				continue
			}
			if best < 0 || elmLess(bestV, parsed) {
				best, bestV = i, parsed
			}
		}
		if best < 0 {
			return nil, nil
		}
		info = pkg.Versions[best]
	}
	seen := map[string]bool{name: true}
	var out []dep
	var collect func(recipe map[string]json.RawMessage, depth int)
	collect = func(recipe map[string]json.RawMessage, depth int) {
		var deps map[string]json.RawMessage
		json.Unmarshal(recipe["dependencies"], &deps)
		for _, n := range slices.Sorted(maps.Keys(deps)) {
			b, _, _ := strings.Cut(n, ":")
			if b == "" || seen[b] {
				continue // the package's own sub-packages
			}
			var spec string
			if json.Unmarshal(deps[n], &spec) != nil {
				var o struct {
					Version  string `json:"version"`
					Path     string `json:"path"`
					Optional bool   `json:"optional"`
				}
				if json.Unmarshal(deps[n], &o) != nil || o.Optional || o.Path != "" {
					continue
				}
				spec = o.Version
			}
			seen[b] = true
			out = append(out, dep{Name: b, Version: strings.TrimSpace(spec)})
		}
		if depth > 0 {
			return
		}
		var subs []json.RawMessage
		json.Unmarshal(recipe["subPackages"], &subs)
		for _, raw := range subs {
			var sub map[string]json.RawMessage
			if json.Unmarshal(raw, &sub) == nil {
				collect(sub, depth+1)
			}
		}
		var configs []map[string]json.RawMessage
		if json.Unmarshal(recipe["configurations"], &configs) == nil && len(configs) > 0 {
			collect(configs[0], depth+1)
		}
	}
	collect(info, 0)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// dubName is a dub package name: lower-case letters, digits, - and _.
var dubName = regexp.MustCompile(`^[a-z0-9_][a-z0-9_.-]*$`)

// dubVersion parses a release's major.minor.patch, ignoring a pre-release or
// build suffix.
func dubVersion(s string) ([3]int, bool) {
	s = strings.TrimPrefix(s, "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	return elmVersion(s)
}

// dubAdmits reports whether a release satisfies a dub version specification:
// ~>1.2.3 (>=1.2.3 <1.3.0), ~>1.2 (>=1.2.0 <2.0.0), ^1.2.3, ==1.2.3 or a bare
// 1.2.3, comparisons joined by spaces (>=1.0.0 <2.0.0), and * or nothing (any
// release). A branch (~master) admits no release.
func dubAdmits(spec string, v [3]int) bool {
	spec = strings.TrimSpace(spec)
	if spec == "" || spec == "*" {
		return true
	}
	bound := func(s string) ([3]int, int, bool) {
		parts := strings.Split(strings.SplitN(strings.SplitN(s, "-", 2)[0], "+", 2)[0], ".")
		var b [3]int
		if len(parts) > 3 {
			return b, 0, false
		}
		for i, p := range parts {
			n, err := strconv.Atoi(p)
			if err != nil || n < 0 {
				return b, 0, false
			}
			b[i] = n
		}
		return b, len(parts), true
	}
	cmp := func(a, b [3]int) int { return slices.Compare(a[:], b[:]) }
	switch {
	case strings.HasPrefix(spec, "~>"):
		lo, n, ok := bound(strings.TrimSpace(spec[2:]))
		if !ok {
			return false
		}
		hi := lo
		switch n {
		case 1:
			hi = [3]int{lo[0] + 1, 0, 0}
		case 2:
			hi = [3]int{lo[0] + 1, 0, 0}
		default:
			hi = [3]int{lo[0], lo[1] + 1, 0}
		}
		return cmp(v, lo) >= 0 && cmp(v, hi) < 0
	case strings.HasPrefix(spec, "^"):
		lo, _, ok := bound(strings.TrimSpace(spec[1:]))
		if !ok {
			return false
		}
		hi := [3]int{lo[0] + 1, 0, 0}
		if lo[0] == 0 {
			hi = [3]int{0, lo[1] + 1, 0}
		}
		return cmp(v, lo) >= 0 && cmp(v, hi) < 0
	case strings.HasPrefix(spec, "~"):
		return false
	}
	for _, c := range strings.Fields(spec) {
		op := strings.TrimRight(c, "0123456789.-+abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
		b, _, ok := bound(strings.TrimPrefix(c, op))
		if !ok {
			return false
		}
		n := cmp(v, b)
		switch op {
		case ">=":
			ok = n >= 0
		case ">":
			ok = n > 0
		case "<":
			ok = n < 0
		case "<=":
			ok = n <= 0
		case "==", "":
			ok = n == 0
		default:
			ok = false
		}
		if !ok {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------- alire

// alireCrate reads an Alire crate's dependencies from the community index,
// a git repository of release manifests served as files:
// <index>/index/<first two letters>/<crate>/<crate>-<version>.toml. A file
// server cannot list a crate's releases, so only an exact version (a lock
// file's, an =1.2.3 constraint's) is asked about. The dependencies are the
// release's depends-on, every alternative of a case(...) expression counted,
// an exact constraint shown as its version and a range as written.
//
// Implements: REQ-SUP-061
func (c *Client) alireCrate(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	name := t.Package
	if !alireName.MatchString(name) {
		return nil, fmt.Errorf("not an Alire crate name: %q", name)
	}
	version, _ := ada.ExactVersion(t.Version)
	body, err := c.accept(ctx, strings.TrimRight(index, "/")+"/index/"+name[:2]+"/"+name+"/"+name+"-"+url.PathEscape(version)+".toml", "text/plain")
	if err != nil {
		return nil, err
	}
	var out []dep
	for n, constraint := range ada.Dependencies(body) {
		if n == name {
			continue
		}
		if v, ok := ada.ExactVersion(constraint); ok {
			constraint = v
		}
		out = append(out, dep{Name: n, Version: constraint})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// alireName is an Alire crate name: lower-case letters, digits and _, at
// least three characters, starting with a letter.
var alireName = regexp.MustCompile(`^[a-z][a-z0-9_]{2,63}$`)

// alireExact reports whether an Alire target names one release the index has
// a manifest for.
func alireExact(v string) bool {
	_, ok := ada.ExactVersion(v)
	return ok
}

// ---------------------------------------------------------------- quicklisp

// qlIndex is a Quicklisp dist's system index: the systems of each project
// and the project releasing each system.
type qlIndex struct {
	projects map[string][]qlSystem
	project  map[string]string
}

type qlSystem struct {
	name, file string
	deps       []string
}

// quicklispProject reads a Quicklisp project's dependencies from its dist's
// system index. The dist's distinfo (the index URL itself, or for a dated
// version <dist>/<version>/distinfo.txt beside it) names the system index,
// systems.txt, whose lines are `project system-file system-name dependency
// ...`; it is read once per dist version. The project's own systems (the one
// named like the project, else each .asd file's primary system, test systems
// left out) give the dependencies, each named by the project releasing it
// from the same index; ASDF, UIOP and SBCL's contribs are left out. A dated
// version answers with its dependencies at that same dist version.
//
// Implements: REQ-SUP-062
func (c *Client) quicklispProject(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	name := t.Package
	if !quicklispName.MatchString(name) {
		return nil, fmt.Errorf("not a Quicklisp project name: %q", name)
	}
	distinfo := index
	if quicklispDated(t.Version) {
		distinfo = strings.TrimSuffix(index, ".txt") + "/" + t.Version + "/distinfo.txt"
	}
	idx, err := c.quicklispIndex(ctx, distinfo)
	if err != nil {
		return nil, err
	}
	systems := idx.projects[name]
	if len(systems) == 0 {
		return nil, fmt.Errorf("%s: no project %q", distinfo, name)
	}
	var own []qlSystem
	for _, s := range systems {
		if s.name == name {
			own = append(own, s)
		}
	}
	if len(own) == 0 {
		for _, s := range systems {
			if s.name == s.file && !strings.Contains(s.name, "test") {
				own = append(own, s)
			}
		}
	}
	version := ""
	if quicklispDated(t.Version) {
		version = t.Version
	}
	seen := map[string]bool{}
	var out []dep
	for _, s := range own {
		for _, d := range s.deps {
			if d == "asdf" || d == "uiop" || strings.HasPrefix(d, "sb-") {
				continue
			}
			p := idx.project[d]
			if p == "" {
				p = d
			}
			if p == name || seen[p] {
				continue
			}
			seen[p] = true
			out = append(out, dep{Name: p, Version: version})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// quicklispIndex reads a distinfo and the system index it names, once.
func (c *Client) quicklispIndex(ctx context.Context, distinfo string) (*qlIndex, error) {
	c.mu.Lock()
	idx, ok := c.qlSystems[distinfo]
	c.mu.Unlock()
	if ok {
		return idx, nil
	}
	info, err := c.accept(ctx, distinfo, "text/plain")
	if err != nil {
		return nil, err
	}
	systems := ""
	for _, line := range strings.Split(string(info), "\n") {
		if k, v, ok := strings.Cut(line, ":"); ok && strings.TrimSpace(k) == "system-index-url" {
			systems = strings.TrimSpace(v)
		}
	}
	if systems == "" {
		return nil, fmt.Errorf("%s: no system-index-url", distinfo)
	}
	body, err := c.accept(ctx, systems, "text/plain")
	if err != nil {
		return nil, err
	}
	idx = &qlIndex{projects: map[string][]qlSystem{}, project: map[string]string{}}
	for _, line := range strings.Split(string(body), "\n") {
		f := strings.Fields(line)
		if len(f) < 3 || strings.HasPrefix(f[0], "#") {
			continue
		}
		s := qlSystem{file: f[1], name: f[2], deps: f[3:]}
		idx.projects[f[0]] = append(idx.projects[f[0]], s)
		if _, dup := idx.project[s.name]; !dup {
			idx.project[s.name] = f[0]
		}
	}
	c.mu.Lock()
	c.qlSystems[distinfo] = idx
	c.mu.Unlock()
	return idx, nil
}

// quicklispName is a Quicklisp project name: what a dist's directories are
// named (cl-ppcre, cl+ssl's cl-plus-ssl, 3bmd, hu.dwim.stefil).
var quicklispName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._+-]{0,127}$`)

// quicklispDated reports whether a version is a dist version: 2023-10-21.
func quicklispDated(v string) bool {
	if len(v) != 10 || v[4] != '-' || v[7] != '-' {
		return false
	}
	for i, r := range v {
		if i != 4 && i != 7 && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

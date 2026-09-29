package index

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"golang.org/x/mod/module"
	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/cue"
	"github.com/sarumaj/depphunter-cli/internal/lang/racket"
	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// The Puppet Forge, Racket package catalogs, Wally registries, the Buf Schema
// Registry and CUE registries: indexes of the plugins added after the first
// dozen, each read the way its own tool reads it.

// ---------------------------------------------------------------- versions and ranges

// semanticVersion parses major.minor.patch, with an optional leading "v", and
// reports whether a pre-release follows (1.2.0-rc.1). Build metadata is ignored.
func semanticVersion(s string) (v [3]int, prerelease, ok bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	s, _, _ = strings.Cut(s, "+")
	s, pre, hasPre := strings.Cut(s, "-")
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return v, false, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || p == "" || p[0] == '+' {
			return v, false, false
		}
		v[i] = n
	}
	return v, hasPre && pre != "", true
}

// partialVersion parses a version a range names, which may stop short or end in a
// wildcard: 1, 1.2, 1.2.3, 1.x, 1.2.*. given is how many parts it names.
func partialVersion(s string) (v [3]int, given int, ok bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	s, _, _ = strings.Cut(s, "+")
	s, _, _ = strings.Cut(s, "-")
	parts := strings.Split(s, ".")
	if len(parts) > 3 || s == "" {
		return v, 0, false
	}
	for i, p := range parts {
		if p == "x" || p == "X" || p == "*" {
			return v, i, true
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || p[0] == '+' {
			return v, 0, false
		}
		v[i] = n
		given = i + 1
	}
	return v, given, true
}

// above is the first version past everything a partial version covers: 1.2 covers
// 1.2.*, so above is 1.3.0. A version naming nothing covers every version.
func above(v [3]int, given int) ([3]int, bool) {
	switch given {
	case 0:
		return v, false
	case 1:
		return [3]int{v[0] + 1, 0, 0}, true
	case 2:
		return [3]int{v[0], v[1] + 1, 0}, true
	}
	return [3]int{v[0], v[1], v[2] + 1}, true
}

func compareVersion(a, b [3]int) int { return slices.Compare(a[:], b[:]) }

// rangeAdmits reports whether a version range admits a version. It reads the
// range syntaxes of npm, Cargo and SemanticPuppet alike: alternatives separated
// by "||", comparators separated by spaces or commas, each an operator (>=, >,
// <=, <, =, ^, ~, ~>) and a version that may stop short or end in a wildcard
// ("1.x"), and hyphen ranges ("1.0.0 - 2.0.0"). A version without an operator is
// a caret range when bareCaret is set (Wally, as Cargo) and that version (or
// every version it covers) otherwise (the Forge).
func rangeAdmits(requirement string, v [3]int, bareCaret bool) bool {
	requirement = strings.TrimSpace(requirement)
	if requirement == "" || requirement == "*" {
		return true
	}
	for _, alternative := range strings.Split(requirement, "||") {
		if comparatorsAdmit(alternative, v, bareCaret) {
			return true
		}
	}
	return false
}

func comparatorsAdmit(alternative string, v [3]int, bareCaret bool) bool {
	fields := strings.Fields(strings.ReplaceAll(alternative, ",", " "))
	if len(fields) == 0 {
		return false
	}
	if len(fields) == 3 && fields[1] == "-" {
		return comparatorAdmits(">=", fields[0], v, bareCaret) && comparatorAdmits("<=", fields[2], v, bareCaret)
	}
	for i := 0; i < len(fields); i++ {
		operator := fields[i][:len(fields[i])-len(strings.TrimLeft(fields[i], "<>=~^!"))]
		version := fields[i][len(operator):]
		if version == "" && i+1 < len(fields) {
			i++
			version = fields[i]
		}
		if !comparatorAdmits(operator, version, v, bareCaret) {
			return false
		}
	}
	return true
}

func comparatorAdmits(operator, version string, v [3]int, bareCaret bool) bool {
	if version == "*" || version == "x" || version == "X" {
		return operator == "" || operator == "=" || operator == ">=" || operator == "<="
	}
	low, given, ok := partialVersion(version)
	if !ok {
		return false
	}
	high, bounded := above(low, given)
	within := func(high [3]int) bool { return compareVersion(v, low) >= 0 && compareVersion(v, high) < 0 }
	switch operator {
	case ">=":
		return compareVersion(v, low) >= 0
	case ">":
		if given < 3 {
			return !bounded || compareVersion(v, high) >= 0
		}
		return compareVersion(v, low) > 0
	case "<=":
		if given < 3 {
			return !bounded || compareVersion(v, high) < 0
		}
		return compareVersion(v, low) <= 0
	case "<":
		return compareVersion(v, low) < 0
	case "", "=", "==":
		if operator == "" && bareCaret {
			return comparatorAdmits("^", version, v, bareCaret)
		}
		return !bounded || within(high)
	case "^":
		switch {
		case given == 0:
			return true
		case low[0] > 0 || given == 1:
			return within([3]int{low[0] + 1, 0, 0})
		case low[1] > 0 || given == 2:
			return within([3]int{0, low[1] + 1, 0})
		}
		return within([3]int{0, 0, low[2] + 1})
	case "~":
		if given <= 1 {
			return !bounded || within(high)
		}
		return within([3]int{low[0], low[1] + 1, 0})
	case "~>":
		// Pessimistic: the last part given may grow, the ones before it not.
		switch given {
		case 0:
			return true
		case 1, 2:
			return within([3]int{low[0] + 1, 0, 0})
		}
		return within([3]int{low[0], low[1] + 1, 0})
	case "!=":
		return !within(high)
	}
	return false
}

// newestAdmitted is the newest of versions a requirement admits: an exact version
// when the requirement names one that is listed, else the newest release the range
// admits, pre-releases left out. "" when none does.
func newestAdmitted(versions []string, requirement string, bareCaret bool) string {
	requirement = strings.TrimSpace(requirement)
	for _, v := range versions {
		if v == requirement || "v"+v == requirement || v == strings.TrimPrefix(requirement, "=") {
			return v
		}
	}
	best, bestV := "", [3]int{}
	for _, s := range versions {
		v, prerelease, ok := semanticVersion(s)
		if !ok || prerelease || !rangeAdmits(requirement, v, bareCaret) {
			continue
		}
		if best == "" || compareVersion(bestV, v) < 0 {
			best, bestV = s, v
		}
	}
	return best
}

// ---------------------------------------------------------------- Puppet Forge

// forgeSlug is a Forge module's slug, author-name (puppetlabs-stdlib); author/name is
// the form metadata.json and Puppetfiles also write.
var forgeSlug = regexp.MustCompile(`^[a-z0-9]+[-/][a-z][a-z0-9_]*$`)

// forgeRelease is what the Forge's v3 API says of one release.
type forgeRelease struct {
	Version  string `json:"version"`
	Metadata struct {
		Dependencies []struct {
			Name               string `json:"name"`
			VersionRequirement string `json:"version_requirement"`
		} `json:"dependencies"`
	} `json:"metadata"`
}

// forgeModule reads a Forge module's dependencies from the Forge's v3 API (the
// public Forge, or one a Puppetfile's `forge` or r10k.yaml's `forge.baseurl`
// names): the metadata.json of `<api>/v3/releases/<slug>-<version>` for an exact
// version, else of the newest release `<api>/v3/modules/<slug>` lists that the
// requirement (">= 4.13.1 < 10.0.0", "1.x", none for the newest) admits, deleted
// releases and pre-releases left out. The dependencies are named by slug, with
// their version_requirement as written.
//
// Implements: REQ-SUP-069
func (c *Client) forgeModule(ctx context.Context, index string, t lang.Target) ([]dependency, error) {
	slug := strings.ToLower(strings.TrimSpace(t.Package))
	if !forgeSlug.MatchString(slug) {
		return nil, fmt.Errorf("not a Forge module slug: %q", t.Package)
	}
	slug = strings.Replace(slug, "/", "-", 1)
	base := strings.TrimRight(index, "/") + "/v3/"
	version := strings.TrimSpace(t.Version)
	var release forgeRelease
	if _, _, exact := semanticVersion(version); exact && !strings.HasPrefix(version, "v") {
		body, err := c.get(ctx, base+"releases/"+slug+"-"+url.PathEscape(version))
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(body, &release); err != nil {
			return nil, err
		}
	} else {
		body, err := c.get(ctx, base+"modules/"+slug)
		if err != nil {
			return nil, err
		}
		var doc struct {
			CurrentRelease forgeRelease `json:"current_release"`
			Releases       []struct {
				Version   string          `json:"version"`
				DeletedAt json.RawMessage `json:"deleted_at"`
			} `json:"releases"`
		}
		if err := json.Unmarshal(body, &doc); err != nil {
			return nil, err
		}
		var versions []string
		for _, r := range doc.Releases {
			if deleted := string(bytes.TrimSpace(r.DeletedAt)); deleted == "" || deleted == "null" {
				versions = append(versions, r.Version)
			}
		}
		chosen := newestAdmitted(versions, version, false)
		switch {
		case chosen == "":
			return nil, nil
		case chosen == doc.CurrentRelease.Version:
			release = doc.CurrentRelease
		default:
			body, err := c.get(ctx, base+"releases/"+slug+"-"+url.PathEscape(chosen))
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(body, &release); err != nil {
				return nil, err
			}
		}
	}
	var out []dependency
	for _, d := range release.Metadata.Dependencies {
		name := strings.ToLower(strings.TrimSpace(d.Name))
		if forgeSlug.MatchString(name) {
			out = append(out, dependency{Name: strings.Replace(name, "/", "-", 1), Version: strings.TrimSpace(d.VersionRequirement)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ---------------------------------------------------------------- Racket package catalogs

// racketName is a Racket package name: letters, digits, _, - and +.
var racketName = regexp.MustCompile(`^[A-Za-z0-9_+-]+$`)

// racketPackage reads a raco package's dependencies from a package catalog, as raco
// consults one: `<catalog>/pkg/<name>` answers the package's entry, a `read`-able
// hash table whose `dependencies` have the shape of an info.rkt's deps (see
// racket.CatalogDependencies). A catalog records the package's current source only,
// so the answer is the same whatever version the package was asked at. An answer
// that is not a hash table is the catalog not having the package, as raco reads it.
//
// Implements: REQ-SUP-070
func (c *Client) racketPackage(ctx context.Context, index string, t lang.Target) ([]dependency, error) {
	if !racketName.MatchString(t.Package) {
		return nil, fmt.Errorf("not a Racket package name: %q", t.Package)
	}
	body, err := c.accept(ctx, strings.TrimRight(index, "/")+"/pkg/"+t.Package, "*/*")
	if err != nil {
		return nil, err
	}
	targets, ok := racket.CatalogDependencies(body)
	if !ok {
		return nil, errAbsent
	}
	var out []dependency
	for _, d := range targets {
		if d.Package == "" || d.Package == t.Package {
			continue
		}
		ecosystem := d.Ecosystem
		if ecosystem == Racket {
			ecosystem = ""
		}
		out = append(out, dependency{Name: d.Package, Version: d.Version, Ecosystem: ecosystem})
	}
	return out, nil
}

// ---------------------------------------------------------------- Wally

// githubRaw serves the files of GitHub repositories; tests point it at their own
// server.
var githubRaw = "https://raw.githubusercontent.com"

// wallyName is a Wally package name, scope/name, lower case as Wally requires.
var wallyName = regexp.MustCompile(`^[a-z0-9_-]+/[a-z0-9_-]+$`)

// wallyRegistry is a Wally registry's URL as wally.toml and manifests write it,
// compared without a trailing slash or .git and with the host in lower case.
func wallyRegistry(registry string) string {
	registry = strings.TrimSuffix(strings.TrimRight(strings.TrimSpace(registry), "/"), ".git")
	if u, err := url.Parse(registry); err == nil && u.Host != "" {
		u.Host = strings.ToLower(u.Host)
		return u.String()
	}
	return registry
}

// wallyFiles is where a Wally registry's files are served one by one: a registry
// is a git repository, and only GitHub's are read, from its default branch.
func wallyFiles(registry string) (string, error) {
	u, err := url.Parse(registry)
	if err == nil && strings.EqualFold(u.Host, "github.com") {
		if parts := strings.Split(strings.Trim(u.Path, "/"), "/"); len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			return githubRaw + "/" + parts[0] + "/" + parts[1] + "/HEAD", nil
		}
	}
	return "", fmt.Errorf("%s: a Wally registry is read only from GitHub, where its files are served one by one", registry)
}

// wallyManifest is one line of a Wally registry's package file: a version's
// wally.toml as JSON.
type wallyManifest struct {
	Package struct {
		Name     string `json:"name"`
		Version  string `json:"version"`
		Registry string `json:"registry"`
	} `json:"package"`
	Dependencies       map[string]string `json:"dependencies"`
	ServerDependencies map[string]string `json:"server-dependencies"`
}

// wallyPackage reads a Wally package's dependencies from its registry's package
// file, `<scope>/<name>` at the root of the registry's repository: one manifest
// per line, one line per version. The version the target names is taken, else
// the newest a requirement admits (a bare version is a caret range, as in
// wally.toml). Its dependencies and server-dependencies are returned as written
// (`scope/name@>=1.0.0, <2.0.0`), dev-dependencies left out, each coming from the
// registry the manifest names. A registry without the package passes the
// question to the fallback registries its config.json lists, as Wally does.
//
// Implements: REQ-SUP-071
func (c *Client) wallyPackage(ctx context.Context, index string, t lang.Target) ([]dependency, error) {
	name := strings.ToLower(strings.TrimSpace(t.Package))
	if !wallyName.MatchString(name) {
		return nil, fmt.Errorf("not a Wally package name: %q", t.Package)
	}
	registries := []string{wallyRegistry(index)}
	var err error
	for i := 0; i < len(registries) && i < 8; i++ {
		var base string
		if base, err = wallyFiles(registries[i]); err != nil {
			return nil, err
		}
		var body []byte
		body, err = c.accept(ctx, base+"/"+name, "*/*")
		if notFound(err) {
			if configuration, failed := c.wallyConfig(ctx, base); failed == nil {
				for _, fallback := range configuration.FallbackRegistries {
					if fallback = wallyRegistry(fallback); !slices.Contains(registries, fallback) {
						registries = append(registries, fallback)
					}
				}
			}
			continue
		}
		if err != nil {
			return nil, err
		}
		return wallyDependencies(body, t.Version, registries[i])
	}
	return nil, err
}

// wallyConfig is a Wally registry's config.json, read once.
func (c *Client) wallyConfig(ctx context.Context, base string) (*wallyConfiguration, error) {
	return c.wallyConfigs.get(base, func() (*wallyConfiguration, error) {
		body, err := c.accept(ctx, base+"/config.json", "*/*")
		if err != nil {
			return nil, err
		}
		var configuration wallyConfiguration
		if err := json.Unmarshal(body, &configuration); err != nil {
			return nil, err
		}
		return &configuration, nil
	})
}

// wallyConfiguration is what a Wally registry's config.json says beyond its API.
type wallyConfiguration struct {
	FallbackRegistries []string `json:"fallback_registries"`
}

// wallyDependencies picks the version out of a package file and reads its
// dependencies.
func wallyDependencies(body []byte, version, registry string) ([]dependency, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	byVersion := map[string]wallyManifest{}
	var versions []string
	for {
		var m wallyManifest
		if err := decoder.Decode(&m); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if _, seen := byVersion[m.Package.Version]; !seen && m.Package.Version != "" {
			versions = append(versions, m.Package.Version)
		}
		byVersion[m.Package.Version] = m
	}
	chosen := newestAdmitted(versions, version, true)
	if chosen == "" {
		return nil, nil
	}
	m := byVersion[chosen]
	from := wallyRegistry(m.Package.Registry)
	if from == "" {
		from = registry
	}
	var out []dependency
	for _, requirements := range []map[string]string{m.Dependencies, m.ServerDependencies} {
		for _, alias := range slices.Sorted(maps.Keys(requirements)) {
			n, requirement, _ := strings.Cut(requirements[alias], "@")
			if n = strings.ToLower(strings.TrimSpace(n)); wallyName.MatchString(n) {
				out = append(out, dependency{Name: n, Version: strings.TrimSpace(requirement), Registry: from})
			}
		}
	}
	return out, nil
}

// ---------------------------------------------------------------- Buf Schema Registry

// BufPlugin is the lang.Target.Registry the proto plugin gives a remote plugin: the
// registry describes the dependencies of modules, and a plugin is not one.
const BufPlugin = "plugin"

// bufSegment is an owner's or a module's name on a Buf Schema Registry.
var bufSegment = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// bufModule reads a Buf Schema Registry module's direct dependencies through the
// registry's API (buf.registry.module.v1, Connect over HTTP with JSON bodies):
// GraphService/GetGraph for the commit the target names (a commit id or a label;
// none is the default label's), whose edges from that commit lead to its
// dependencies' commits, then ModuleService/GetModules and OwnerService/GetOwners
// for their names. Three requests per module. A dependency on another registry
// (federation) is named by that registry alone, so it is left out. Each
// dependency is pinned by its commit id.
//
// The registry is the module's host; the token sent is the one buf itself would
// send there (auth.Store.BufToken), and no other credential.
//
// Implements: REQ-SUP-072
func (c *Client) bufModule(ctx context.Context, index string, t lang.Target) ([]dependency, error) {
	parts := strings.Split(strings.TrimSpace(t.Package), "/")
	if len(parts) != 3 || !bufSegment.MatchString(parts[1]) || !bufSegment.MatchString(parts[2]) {
		return nil, fmt.Errorf("not a Buf Schema Registry module name: %q", t.Package)
	}
	host := parts[0]
	name := map[string]string{"owner": parts[1], "module": parts[2]}
	if reference := strings.TrimSpace(t.Version); reference != "" {
		name["ref"] = reference
	}
	var graph struct {
		Graph struct {
			Commits []struct {
				ID       string `json:"id"`
				ModuleID string `json:"moduleId"`
			} `json:"commits"`
			Edges []struct {
				FromNode struct {
					CommitID string `json:"commitId"`
				} `json:"fromNode"`
				ToNode struct {
					CommitID string `json:"commitId"`
				} `json:"toNode"`
			} `json:"edges"`
			RegistryCommitIDs []struct {
				CommitID string `json:"commitId"`
			} `json:"registryCommitIds"`
		} `json:"graph"`
	}
	request := map[string]any{"resourceRefs": []any{map[string]any{"name": name}}}
	if err := c.bufCall(ctx, index, host, "buf.registry.module.v1.GraphService/GetGraph", request, &graph); err != nil {
		return nil, err
	}
	// The commit asked about is the one no edge leads to.
	reached := map[string]bool{}
	for _, e := range graph.Graph.Edges {
		reached[bufID(e.ToNode.CommitID)] = true
	}
	root := ""
	for _, commit := range graph.Graph.Commits {
		if !reached[bufID(commit.ID)] {
			root = bufID(commit.ID)
			break
		}
	}
	elsewhere := map[string]bool{}
	for _, r := range graph.Graph.RegistryCommitIDs {
		elsewhere[bufID(r.CommitID)] = true
	}
	moduleOf := map[string]string{}
	for _, commit := range graph.Graph.Commits {
		moduleOf[bufID(commit.ID)] = commit.ModuleID
	}
	var direct []string
	var moduleRefs []any
	for _, e := range graph.Graph.Edges {
		from, to := bufID(e.FromNode.CommitID), bufID(e.ToNode.CommitID)
		if from != root || elsewhere[to] || moduleOf[to] == "" || slices.Contains(direct, to) {
			continue
		}
		direct = append(direct, to)
		moduleRefs = append(moduleRefs, map[string]string{"id": moduleOf[to]})
	}
	if len(direct) == 0 {
		return nil, nil
	}
	var modules struct {
		Modules []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			OwnerID string `json:"ownerId"`
		} `json:"modules"`
	}
	if err := c.bufCall(ctx, index, host, "buf.registry.module.v1.ModuleService/GetModules",
		map[string]any{"moduleRefs": moduleRefs}, &modules); err != nil {
		return nil, err
	}
	var ownerRefs []any
	for _, m := range modules.Modules {
		ownerRefs = append(ownerRefs, map[string]string{"id": m.OwnerID})
	}
	var owners struct {
		Owners []struct {
			User, Organization *struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}
		} `json:"owners"`
	}
	if err := c.bufCall(ctx, index, host, "buf.registry.owner.v1.OwnerService/GetOwners",
		map[string]any{"ownerRefs": ownerRefs}, &owners); err != nil {
		return nil, err
	}
	ownerName, moduleName := map[string]string{}, map[string]string{}
	for _, o := range owners.Owners {
		for _, named := range []*struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}{o.User, o.Organization} {
			if named != nil {
				ownerName[bufID(named.ID)] = named.Name
			}
		}
	}
	for _, m := range modules.Modules {
		if owner := ownerName[bufID(m.OwnerID)]; owner != "" && m.Name != "" {
			moduleName[bufID(m.ID)] = host + "/" + owner + "/" + m.Name
		}
	}
	var out []dependency
	for _, commit := range direct {
		if n := moduleName[bufID(moduleOf[commit])]; n != "" {
			out = append(out, dependency{Name: n, Version: commit})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// bufID is an id as buf.lock writes it: lower-case hex without dashes.
func bufID(id string) string { return strings.ToLower(strings.ReplaceAll(id, "-", "")) }

// bufCall makes one unary Connect call with a JSON body and reads the JSON answer.
// A Connect error comes back with the HTTP status its code maps to - not_found is
// 404 - so a registry without the module reads as one (notFound).
func (c *Client) bufCall(ctx context.Context, index, host, procedure string, request, reply any) error {
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}
	address := strings.TrimRight(index, "/") + "/" + procedure
	call, err := http.NewRequestWithContext(ctx, http.MethodPost, address, bytes.NewReader(body))
	if err != nil {
		return err
	}
	call.Header.Set("Content-Type", "application/json")
	call.Header.Set("Accept", "application/json")
	call.Header.Set("Connect-Protocol-Version", "1")
	if token := c.auth.BufToken(host); token != "" && (call.URL.Scheme == "https" || loopbackHost(call.URL.Hostname())) {
		call.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := c.send(ctx, call, address)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return &statusError{url: address, status: response.Status, code: response.StatusCode}
	}
	data, err := readLimited(response)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, reply)
}

// loopbackHost reports whether a host is this machine, which a token may reach
// over plain http (a registry run locally, a test's).
func loopbackHost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// ---------------------------------------------------------------- CUE registries

// cueModuleFile is the media type of the layer of a CUE module's manifest that
// holds its module.cue.
const cueModuleFile = "application/vnd.cue.modulefile.v1"

// cueModule reads a CUE module's dependencies from a CUE registry, an OCI registry
// laid out as cue lays it out: the module version's manifest is
// `<registry>/v2/<prefix>/<module path>/manifests/<version>` (the prefix is the
// repository part of a CUE_REGISTRY entry), and its module.cue is the blob of the
// layer of media type application/vnd.cue.modulefile.v1 - one small blob, not the
// module's archive. The deps it lists are returned, pinned by their versions. The
// requests go through the OCI client, pull-token challenges and all.
//
// Implements: REQ-SUP-073
func (c *Client) cueModule(ctx context.Context, index string, t lang.Target) ([]dependency, error) {
	if err := module.CheckImportPath(t.Package); err != nil {
		return nil, fmt.Errorf("not a CUE module path: %q", t.Package)
	}
	base, repository := index, t.Package
	if u, err := url.Parse(index); err == nil && u.Host != "" {
		base = u.Scheme + "://" + u.Host
		if prefix := strings.Trim(u.Path, "/"); prefix != "" {
			repository = prefix + "/" + t.Package
		}
	}
	manifest, err := c.ociFetch(ctx, base, repository, t.Version, "application/vnd.oci.image.manifest.v1+json")
	if err != nil {
		return nil, err
	}
	var doc struct {
		Layers []struct {
			MediaType    string `json:"mediaType"`
			ArtifactType string `json:"artifactType"`
			Digest       string `json:"digest"`
		} `json:"layers"`
	}
	if err := json.Unmarshal(manifest, &doc); err != nil {
		return nil, err
	}
	digest := ""
	for _, layer := range doc.Layers {
		if layer.MediaType == cueModuleFile || layer.ArtifactType == cueModuleFile {
			digest = layer.Digest
		}
	}
	if digest == "" {
		return nil, fmt.Errorf("%s/%s:%s: not a CUE module (no module.cue layer)", base, repository, t.Version)
	}
	source, err := c.ociBlob(ctx, base, repository, digest)
	if err != nil {
		return nil, err
	}
	var out []dependency
	for _, d := range cue.ModuleDependencies(source) {
		out = append(out, dependency{Name: d.Package, Version: d.Version})
	}
	return out, nil
}

// ---------------------------------------------------------------- where they are configured

// forgeHosts are the public Forge's hosts, old and new: a Puppetfile naming one
// names the public Forge.
var forgeHosts = map[string]bool{
	"forge.puppet.com": true, "forgeapi.puppet.com": true, "forge.puppetlabs.com": true, "forgeapi.puppetlabs.com": true,
}

// puppetfileForge is a Puppetfile's `forge "<url>"` line.
var puppetfileForge = regexp.MustCompile(`(?m)^[ \t]*forge[ \t(]+['"]([^'"]+)['"]`)

// parsePuppetfileForge reads the Forge a Puppetfile names: librarian-puppet
// installs from it, and r10k does when its configuration lets a Puppetfile
// override its own. The public Forge is not recorded.
//
// Implements: REQ-SUP-069
func parsePuppetfileForge(data []byte, k sink) {
	for _, m := range puppetfileForge.FindAllSubmatch(data, -1) {
		if forge := forgeURL(string(m[1])); forge != "" {
			k.add(PuppetForge, forge, "")
		}
	}
}

// forgeURL is a Forge's API URL ("" for the public Forge): a bare host is https.
func forgeURL(forge string) string {
	forge = strings.TrimSpace(forge)
	if !strings.Contains(forge, "://") {
		forge = "https://" + forge
	}
	u, err := url.Parse(forge)
	if err != nil || u.Host == "" || forgeHosts[strings.ToLower(u.Host)] {
		return ""
	}
	return strings.TrimRight(forge, "/")
}

// machineR10K reads the Forge r10k's configuration names (`forge: baseurl`), from
// the first of its configuration files that exists.
//
// Implements: REQ-SUP-069, REQ-SUP-064
func machineR10K(m userconf.Machine, k sink) {
	for _, name := range m.R10KConfigs() {
		data, err := os.ReadFile(name)
		if err != nil {
			continue
		}
		var doc struct {
			Forge struct {
				BaseURL string `yaml:"baseurl"`
			} `yaml:"forge"`
		}
		if yaml.Unmarshal(data, &doc) == nil {
			k.add(PuppetForge, forgeURL(doc.Forge.BaseURL), "")
		}
		return
	}
}

// parseWallyManifest reads the registry a wally.toml's [package] names, which
// serves the package's dependencies.
//
// Implements: REQ-SUP-071
func parseWallyManifest(data []byte, k sink) {
	var doc struct {
		Package struct {
			Registry string `toml:"registry"`
		} `toml:"package"`
	}
	if _, err := toml.Decode(string(data), &doc); err == nil && strings.TrimSpace(doc.Package.Registry) != "" {
		k.add(Wally, wallyRegistry(doc.Package.Registry), "")
	}
}

// parseCUERegistry reads CUE_REGISTRY as cue does: a comma-separated list of
// registries, each `<registry>` (for every module) or `<module prefix>=<registry>`
// (for the modules under that prefix, the longest matching prefix winning); a
// registry is `<host>[/<repository prefix>]`, over https unless it ends in
// "+insecure" or is this machine, and `none` for no registry at all. The catch-all
// replaces registry.cue.works; `none` as the catch-all switches it off. A
// `file:` or `inline:` configuration (CUE's own configuration language) is not
// read.
//
// Implements: REQ-SUP-073
func parseCUERegistry(value string, k sink) {
	value = strings.TrimSpace(value)
	kind, rest, _ := strings.Cut(value, ":")
	switch kind {
	case "file", "inline":
		return
	case "simple":
		value = rest
	}
	type scoped struct{ prefix, url string }
	var prefixes []scoped
	for _, entry := range strings.Split(value, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		prefix, registry, ok := strings.Cut(entry, "=")
		if !ok {
			prefix, registry = "", entry
		}
		address := cueRegistryURL(strings.TrimSpace(registry))
		switch {
		case prefix == "" && address == "":
			k.off(CUE)
		case prefix == "":
			k.add(CUE, address, "")
		case address != "":
			prefixes = append(prefixes, scoped{strings.Trim(strings.TrimSpace(prefix), "/"), address})
		}
	}
	sort.SliceStable(prefixes, func(i, j int) bool { return len(prefixes[i].prefix) > len(prefixes[j].prefix) })
	for _, p := range prefixes {
		k.add(CUE, p.url, p.prefix)
	}
}

// cueRegistryURL is a CUE_REGISTRY registry as a URL ("" for none).
func cueRegistryURL(registry string) string {
	if registry == "" || registry == "none" {
		return ""
	}
	host, _, _ := strings.Cut(registry, "/")
	hostname := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		hostname = h
	}
	scheme := "https"
	if hostname == "localhost" || net.ParseIP(strings.Trim(hostname, "[]")).IsLoopback() {
		scheme = "http" // cue's default for this machine
	}
	if i := strings.LastIndex(registry, "+"); i > 0 {
		switch registry[i+1:] {
		case "insecure":
			scheme = "http"
		case "secure":
			scheme = "https"
		default:
			return ""
		}
		registry = registry[:i]
	}
	return scheme + "://" + strings.TrimRight(registry, "/")
}

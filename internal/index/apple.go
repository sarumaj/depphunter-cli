package index

// CocoaPods spec repositories cloned on this machine, and SwiftPM package
// registries: where pods and Swift packages come from beyond the CocoaPods CDN.

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/cocoapods"
	"github.com/sarumaj/depphunter-cli/internal/lang/swift"
	"github.com/sarumaj/depphunter-cli/internal/scan"
	"github.com/sarumaj/depphunter-cli/internal/trace"
	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// SwiftPM is the swift plugin's island of packages. A package in a registry is
// named by its identity, scope.name; one in a git repository by the repository's
// URL, which no registry is asked about. There is no public registry: only the
// registries a registries.json names are.
const SwiftPM = "swiftpm"

// ---------------------------------------------------------------- CocoaPods

// podRepository is a spec repository in CocoaPods' repos directory: a git
// clone `pod repo add` made, or the directory a CDN source (the trunk's
// included) keeps the files it downloaded in.
type podRepository struct {
	url, local string
	cdn        bool
}

// machineCocoaPods reads the spec repositories in CocoaPods' repos directory
// (userconf.CocoaPodsRepositories), in name order: a git clone by its origin
// remote's URL, a CDN source by its .url file. They are not indexes of their
// own: CocoaPods asks the repositories a Podfile lists, and a clone is where
// one of them is read from (see podCopy).
//
// Implements: REQ-SUP-074, REQ-SUP-064
func (c *Config) machineCocoaPods(m userconf.Machine) {
	c.podRepositories = nil
	directory := m.CocoaPodsRepositories()
	if directory == "" {
		return
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		local := filepath.Join(directory, e.Name())
		if data, err := os.ReadFile(filepath.Join(local, ".url")); err == nil {
			if u := strings.TrimSpace(string(data)); u != "" {
				c.podRepositories = append(c.podRepositories, podRepository{url: u, local: local, cdn: true})
			}
			continue
		}
		if u := gitOrigin(filepath.Join(local, ".git", "config")); u != "" {
			c.podRepositories = append(c.podRepositories, podRepository{url: u, local: local})
		}
	}
}

// gitOrigin is the URL of the origin remote a git config file names, or "".
func gitOrigin(config string) string {
	data, err := os.ReadFile(config)
	if err != nil {
		return ""
	}
	in := false
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") {
			in = strings.EqualFold(strings.Join(strings.Fields(strings.Trim(line, "[]")), " "), `remote "origin"`)
			continue
		}
		if k, v, ok := strings.Cut(line, "="); in && ok && strings.EqualFold(strings.TrimSpace(k), "url") {
			return strings.Trim(strings.TrimSpace(v), `"`)
		}
	}
	return ""
}

// podRepositoryURL is a spec repository URL as CocoaPods compares them when it
// looks for the clone of a Podfile's source: lower case, without ".git" and a
// trailing slash, any address of the trunk repository on GitHub as one. A
// credential written into an https URL is not part of the address.
func podRepositoryURL(u string) string {
	u = strings.TrimSpace(u)
	if p, err := url.Parse(u); err == nil && p.User != nil && strings.HasPrefix(p.Scheme, "http") {
		p.User = nil
		u = p.String()
	}
	u = strings.TrimSuffix(strings.TrimSuffix(strings.ToLower(u), ".git"), "/")
	if podTrunkRepository.MatchString(u) {
		return "https://github.com/cocoapods/specs"
	}
	return u
}

// podTrunkRepository matches the trunk spec repository's addresses on GitHub.
var podTrunkRepository = regexp.MustCompile(`github\.com[:/]+cocoapods/specs$`)

// podCopy is the spec repository on this machine's disk that is the copy of an
// index: the clone whose origin is its URL (the public CDN's being a clone of
// the trunk repository, or the trunk CDN source's directory of downloaded files).
// A clone is read in the index's place, with nothing sent anywhere.
func (c *Config) podCopy(index string) podRepository {
	want := podRepositoryURL(index)
	public := index == c.publicURL(CocoaPods)
	var cdn podRepository
	for _, r := range c.podRepositories {
		have := podRepositoryURL(r.url)
		switch {
		case !r.cdn && (have == want || public && have == podRepositoryURL("https://github.com/CocoaPods/Specs")):
			return r
		case r.cdn && have == want && cdn.local == "":
			cdn = r
		}
	}
	return cdn
}

// podGit reports whether a spec repository is served by git rather than as
// files over HTTP (a CDN): an ssh, git or file address, or one ending in .git.
func podGit(index string) bool {
	u := strings.ToLower(strings.TrimRight(strings.TrimSpace(index), "/"))
	if strings.HasSuffix(u, ".git") {
		return true
	}
	if m := scpLike.FindStringSubmatch(u); m != nil && !strings.Contains(u, "://") {
		return true
	}
	p, err := url.Parse(u)
	return err != nil || p.Scheme != "https" && p.Scheme != "http"
}

// cocoapodsCopy answers from a spec repository cloned on this machine, as
// CocoaPods reads one: the specifications below Specs/ when the clone has that
// directory, else below its root; each pod's directory sharded by the MD5 of
// its name as the clone's CocoaPods-version.yml says (`prefix_lengths`), else
// not; one directory per version holding <Name>.podspec.json or, in a
// repository that keeps them as written, the Ruby <Name>.podspec. A pod the
// clone does not have is not found there, and the next repository is asked.
//
// Implements: REQ-SUP-074
func cocoapodsCopy(local string, t lang.Target) ([]dependency, error) {
	version := strings.TrimSpace(t.Version)
	if !podPathElement(t.Package) || lang.Pinned(version) && !podPathElement(version) {
		return nil, fmt.Errorf("%w: %q", errAbsent, t.Package)
	}
	specs := filepath.Join(local, "Specs")
	if info, err := os.Stat(specs); err != nil || !info.IsDir() {
		specs = local
	}
	directory := filepath.Join(append([]string{specs}, podShard(local, t.Package)...)...)
	entries, err := os.ReadDir(filepath.Join(directory, t.Package))
	if err != nil {
		return nil, fmt.Errorf("%w: %s has no %s", errAbsent, local, t.Package)
	}
	if !lang.Pinned(version) {
		var versions []string
		for _, e := range entries {
			if e.IsDir() {
				versions = append(versions, e.Name())
			}
		}
		if version = podVersion(version, versions); version == "" {
			return nil, fmt.Errorf("%w: %s has no version of %s", errAbsent, local, t.Package)
		}
	}
	base := filepath.Join(directory, t.Package, version, t.Package)
	if data, err := os.ReadFile(base + ".podspec.json"); err == nil {
		var spec podSpec
		if err := json.Unmarshal(data, &spec); err != nil {
			return nil, err
		}
		return podDependencies(spec, t.Package), nil
	}
	data, err := os.ReadFile(base + ".podspec")
	if err != nil {
		return nil, fmt.Errorf("%w: %s has no %s %s", errAbsent, local, t.Package, version)
	}
	return podDependencies(podSpecOf(cocoapods.ReadSpec(data)), t.Package), nil
}

// podPathElement reports whether a pod's name or version can stand for one
// directory of a clone, and no other place on the disk.
func podPathElement(s string) bool {
	return s != "" && s != "." && s != ".." && !strings.ContainsAny(s, `/\:`)
}

// podCached is a file the directory of a CDN source holds, the elements of its
// path each one directory or file name; ok is false when it is not there.
func podCached(directory string, elements ...string) (data []byte, ok bool) {
	if directory == "" {
		return nil, false
	}
	for _, e := range elements {
		if !podPathElement(e) {
			return nil, false
		}
	}
	data, err := os.ReadFile(filepath.Join(append([]string{directory}, elements...)...))
	return data, err == nil
}

// podShard is the directories a clone keeps a pod's specifications below: the
// leading hex digits of the MD5 of its name, as many at each level as the
// clone's CocoaPods-version.yml says.
func podShard(local, pod string) []string {
	var metadata struct {
		PrefixLengths []int `yaml:"prefix_lengths"`
	}
	data, err := os.ReadFile(filepath.Join(local, "CocoaPods-version.yml"))
	if err != nil || yaml.Unmarshal(data, &metadata) != nil {
		return nil
	}
	sum := md5.Sum([]byte(pod))
	digits := hex.EncodeToString(sum[:])
	var out []string
	for _, n := range metadata.PrefixLengths {
		if n <= 0 || n > len(digits) {
			return nil
		}
		out, digits = append(out, digits[:n]), digits[n:]
	}
	return out
}

// podVersion picks the version of a pod to read for a requirement: the newest
// release that is not a pre-release and that the requirement allows, else the
// newest release, else "".
func podVersion(requirement string, versions []string) string {
	version, newest := "", ""
	for _, v := range versions {
		if strings.Contains(v, "-") {
			continue // a pre-release
		}
		if newest == "" || compareVersions(v, newest) > 0 {
			newest = v
		}
		// CocoaPods' operators are Terraform's: ~>, >=, <, =, !=.
		if terraformAllows(requirement, v) && (version == "" || compareVersions(v, version) > 0) {
			version = v
		}
	}
	if version == "" {
		return newest
	}
	return version
}

// podDependencies is a specification's answer: the root spec's dependencies and
// those of its default subspecs (all subspecs when it names none), without test
// specs and the pod's own subspecs; "= 1.2.3" is the bare, pinned version.
func podDependencies(spec podSpec, pod string) []dependency {
	seen := map[string]bool{}
	var out []dependency
	walk := func(s podSpec) {
		for _, name := range slices.Sorted(maps.Keys(s.Dependencies)) {
			root, _, _ := strings.Cut(name, "/")
			if root == pod || seen[root] {
				continue
			}
			seen[root] = true
			requirement := strings.Join(s.Dependencies[name], ", ")
			if v, ok := strings.CutPrefix(requirement, "= "); ok && lang.Pinned(v) {
				requirement = v
			}
			out = append(out, dependency{Name: root, Version: requirement})
		}
	}
	walk(spec)
	defaults := map[string]bool{}
	for _, d := range spec.defaults() {
		defaults[d] = true
	}
	for _, subspec := range spec.Subspecs {
		if len(defaults) == 0 || defaults[subspec.Name] {
			walk(subspec)
		}
	}
	sortDependencies(out)
	return out
}

// podSpecOf is a Ruby podspec read as its podspec.json would say it.
func podSpecOf(s cocoapods.Spec) podSpec {
	out := podSpec{Name: s.Name, Dependencies: s.Dependencies}
	if len(s.DefaultSubspecs) > 0 {
		out.DefaultSubspecs, _ = json.Marshal(s.DefaultSubspecs)
	}
	for _, subspec := range s.Subspecs {
		out.Subspecs = append(out.Subspecs, podSpecOf(subspec))
	}
	return out
}

// noPodCopy is the note for a git spec repository with no clone on this machine.
func (c *Client) noPodCopy(index string) {
	// Implements: REQ-TRC-017
	c.note(trace.NoteNoCopy, "CocoaPods spec repository "+index+" is read only from a clone on this machine "+
		"(`pod repo add`, in CocoaPods' repos directory), and there is none: its pods are asked of the next repository")
}

// ---------------------------------------------------------------- SwiftPM

// swiftRegistries is what a registries.json says about registries: the one of
// each package scope, "[default]" the one every other scope is asked of.
type swiftRegistries struct {
	Registries map[string]struct {
		URL string `json:"url"`
	} `json:"registries"`
	Version int `json:"version"`
}

// swiftDefault is registries.json's key for the default registry.
const swiftDefault = "[default]"

// parseSwiftRegistries reads the registries a registries.json names: the
// default registry, which serves every package no scope maps, and each scope's.
// Scopes compare without case, as SwiftPM compares them. A file SwiftPM would
// refuse - another format version, a scope that is no scope - names nothing.
//
// Implements: REQ-SUP-075
func parseSwiftRegistries(data []byte, k sink) {
	var doc swiftRegistries
	if json.Unmarshal(data, &doc) != nil || doc.Version != 1 {
		return
	}
	for scope := range doc.Registries {
		if scope != swiftDefault && !swiftScope.MatchString(scope) {
			return
		}
	}
	for _, scope := range slices.Sorted(maps.Keys(doc.Registries)) {
		u := strings.TrimSpace(doc.Registries[scope].URL)
		switch {
		case u == "":
		case scope == swiftDefault:
			k.add(SwiftPM, u, "")
		default:
			k.add(SwiftPM, u, strings.ToLower(scope))
		}
	}
}

// projectSwiftPM reads the registries.json of each package the repository
// holds: SwiftPM reads .swiftpm/configuration/registries.json beside the root
// package's Package.swift, from the disk whether or not git ignores it.
//
// Implements: REQ-SUP-075
func projectSwiftPM(files []*scan.File, k sink) {
	seen := map[string]bool{}
	for _, f := range files {
		base := strings.ToLower(filepath.Base(f.Path))
		if base != "package.swift" && !(strings.HasPrefix(base, "package@swift") && strings.HasSuffix(base, ".swift")) {
			continue
		}
		directory := filepath.Dir(f.AbsolutePath)
		if seen[directory] {
			continue
		}
		seen[directory] = true
		if data, err := os.ReadFile(filepath.Join(directory, ".swiftpm", "configuration", "registries.json")); err == nil {
			parseSwiftRegistries(data, k)
		}
	}
}

var (
	// swiftScope is a registry scope: alphanumeric and single hyphens, at most 39.
	swiftScope = regexp.MustCompile(`^[A-Za-z0-9](?:-?[A-Za-z0-9]){0,38}$`)
	// swiftIdentityPattern is a registry package's identity, scope.name.
	swiftIdentityPattern = regexp.MustCompile(`^([A-Za-z0-9](?:-?[A-Za-z0-9]){0,38})\.([A-Za-z0-9][A-Za-z0-9_-]{0,99})$`)
)

// swiftIdentity splits a registry package's identity into its scope and name;
// ok is false for a package named by its repository's URL.
func swiftIdentity(packageName string) (scope, name string, ok bool) {
	m := swiftIdentityPattern.FindStringSubmatch(packageName)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

// swiftCandidates lists the registry a Swift package is asked of, as SwiftPM
// merges its configurations - the repository's registries.json over the
// user's, scope by scope and default by default: the repository's registry for
// the package's scope, else the user's, else the repository's default registry,
// else the user's. SwiftPM asks that registry alone. A package named by a URL
// has none.
//
// Implements: REQ-SUP-075
func (c *Config) swiftCandidates(packageName string) []candidate {
	scope, _, ok := swiftIdentity(packageName)
	if !ok {
		return nil
	}
	rank := func(s Source) int {
		r := 0
		if s.Scope == "" {
			r += 2
		}
		if s.Trusted {
			r++
		}
		return r
	}
	var chosen *Source
	for i, s := range c.sources[SwiftPM] {
		if s.Scope != "" && !strings.EqualFold(s.Scope, scope) {
			continue
		}
		if chosen == nil || rank(s) < rank(*chosen) {
			chosen = &c.sources[SwiftPM][i]
		}
	}
	if chosen == nil {
		return nil
	}
	return []candidate{{url: chosen.URL, primary: true, known: c.fetchable(SwiftPM, *chosen)}}
}

// machineNames reports whether this machine's own configuration names an index
// for an ecosystem: a repository naming the same one names an index the user
// already asks.
func (c *Config) machineNames(ecosystem, index string) bool {
	for _, s := range c.sources[ecosystem] {
		if s.Trusted && s.URL == index {
			return true
		}
	}
	return false
}

// swiftRegistryMedia is the media type of the Swift Package Registry API's
// version 1, to which the answer's format is appended.
const swiftRegistryMedia = "application/vnd.swift.registry.v1+"

// swiftPackage reads a registry package's dependencies as SwiftPM does
// (SE-0292, the Swift Package Registry API): `GET <registry>/<scope>/<name>`
// lists its releases, and `GET <registry>/<scope>/<name>/<version>/Package.swift`
// is one release's manifest. The release read is the version asked for when it
// is exact, else the newest release without a problem that is not a
// pre-release and that the requirement admits (a..<b, a...b), else the newest
// such release. The answer is the manifest's registry (`.package(id:)`) and
// repository (`.package(url:)`) dependencies, as the swift plugin reads them.
//
// Implements: REQ-SUP-075
func (c *Client) swiftPackage(ctx context.Context, index string, t lang.Target) ([]dependency, error) {
	scope, name, ok := swiftIdentity(t.Package)
	if !ok {
		return nil, fmt.Errorf("%w: %s is no registry package", errAbsent, t.Package)
	}
	base := strings.TrimRight(index, "/") + "/" + url.PathEscape(scope) + "/" + url.PathEscape(name)
	version := strings.TrimSpace(t.Version)
	if !lang.Pinned(version) {
		metadata, err := acceptJSON[struct {
			Releases map[string]struct {
				Problem *json.RawMessage `json:"problem"`
			} `json:"releases"`
		}](ctx, c, base, swiftRegistryMedia+"json")
		if err != nil {
			return nil, err
		}
		requirement := version
		version = ""
		newest := ""
		for v, release := range metadata.Releases {
			if release.Problem != nil || strings.Contains(v, "-") {
				continue // unavailable, or a pre-release
			}
			if newest == "" || compareVersions(v, newest) > 0 {
				newest = v
			}
			if swiftAdmits(requirement, v) && (version == "" || compareVersions(v, version) > 0) {
				version = v
			}
		}
		if version = cmp.Or(version, newest); version == "" {
			return nil, fmt.Errorf("%w: %s lists no release of %s", errAbsent, index, t.Package)
		}
	}
	body, err := c.accept(ctx, base+"/"+url.PathEscape(version)+"/Package.swift", swiftRegistryMedia+"swift")
	if err != nil {
		return nil, err
	}
	var out []dependency
	for _, d := range swift.ManifestDependencies(string(body)) {
		n := d.ID
		if n == "" {
			n = lang.RepositoryName(d.URL)
		}
		if n != "" {
			out = append(out, dependency{Name: n, Version: d.Requirement})
		}
	}
	sortDependencies(out)
	return out, nil
}

// swiftAdmits reports whether a version satisfies a requirement as the swift
// plugin shows it: none, an exact version, a..<b or a...b. A branch or a
// revision admits no release.
func swiftAdmits(requirement, v string) bool {
	requirement = strings.TrimSpace(requirement)
	if requirement == "" {
		return true
	}
	if low, high, ok := strings.Cut(requirement, "..<"); ok {
		return compareVersions(v, low) >= 0 && compareVersions(v, high) < 0
	}
	if low, high, ok := strings.Cut(requirement, "..."); ok {
		return compareVersions(v, low) >= 0 && compareVersions(v, high) <= 0
	}
	return v == requirement
}

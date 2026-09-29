package cpp

import (
	"encoding/json"
	"maps"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// The package managers whose manifests turn a third-party include into a declared
// package: vcpkg (vcpkg.json) and Conan (conanfile.txt, conanfile.py, conan.lock).
const (
	ecosystemVcpkg = "vcpkg"
	ecosystemConan = "conan"
)

// declaredPackage is one library a vcpkg or Conan manifest declares, or a conan.lock holds.
type declaredPackage struct {
	ecosystem, name string
	version         string // exact when pinned; else the minimum or range declared
	requested       string // what the manifest asked for when a lock or override fixed it
	pinned          bool
	floating        bool // names no version, and nothing fixes one
	// qualifier is a Conan reference's user, channel and recipe revision
	// (ConanReference.Qualifier), the one a lock pins winning.
	qualifier string
}

func (p *declaredPackage) target() lang.Target {
	return lang.Target{
		Ecosystem: p.ecosystem, Package: p.name, Version: p.version, Requested: p.requested,
		Pinned: p.pinned, Floating: p.floating, Registry: p.qualifier,
	}
}

// packages are the libraries declared per directory holding a manifest; a source
// sees those of the directories above it, the nearest first.
type packages struct {
	directories map[string]map[string][]*declaredPackage // dir -> normalized name -> vcpkg first, then Conan
	// order is every manifest directory, shallowest first, then by name: where the
	// includes of a file under no manifest are looked for.
	order []string
	// tree is what a Conan 1 conan.lock (graph_lock) says each "name/version"
	// requires. A Conan 2 lock is a flat list and gives no edges.
	tree map[string][]lang.Target
	// flat are the Conan 2 locks that pinned something, for the note that says
	// they give no edges.
	flat []string
	// fetched is the content the build fetches (fetched.go), shallowest first.
	fetched []Fetched
}

// normalizeName folds the spellings one library goes by in vcpkg, Conan and its include
// directory: case, and "_" for "-" (nlohmann_json, nlohmann-json).
func normalizeName(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), "_", "-")
}

// manifestNames are the files read; the order within one directory matters, since a
// lock refines what a conanfile declared and a configuration's baseline applies
// to vcpkg.json.
var manifestNames = []string{"vcpkg-configuration.json", "vcpkg.json", "conanfile.txt", "conanfile.py", "conan.lock"}

// Implements: REQ-CPP-009, REQ-CPP-010, REQ-CPP-011
func readPackages(all []*scan.File) *packages {
	p := &packages{directories: map[string]map[string][]*declaredPackage{}, tree: map[string][]lang.Target{}}
	byDirectory := map[string]map[string][]byte{}
	for _, f := range all {
		base := path.Base(f.Path)
		if !isManifest(base) || f.Binary || f.TooLarge || f.Size > lang.MaxParseSize {
			continue
		}
		source, err := os.ReadFile(f.AbsolutePath)
		if err != nil {
			continue
		}
		directory := path.Dir(f.Path)
		if byDirectory[directory] == nil {
			byDirectory[directory] = map[string][]byte{}
		}
		byDirectory[directory][base] = source
	}
	for directory, files := range byDirectory {
		var vcpkg, conan []*declaredPackage
		if source, ok := files["vcpkg.json"]; ok {
			vcpkg = readVcpkg(source, files["vcpkg-configuration.json"])
		}
		if source, ok := files["conanfile.txt"]; ok {
			conan = append(conan, readConanfileTxt(source)...)
		}
		if source, ok := files["conanfile.py"]; ok {
			conan = append(conan, readConanfilePy(source)...)
		}
		if source, ok := files["conan.lock"]; ok {
			var flat bool
			if conan, flat = p.lock(source, conan); flat {
				p.flat = append(p.flat, path.Join(directory, "conan.lock"))
			}
		}
		byName := map[string][]*declaredPackage{}
		for _, list := range [][]*declaredPackage{vcpkg, conan} {
			for _, d := range list {
				key := normalizeName(d.name)
				byName[key] = append(byName[key], d)
			}
		}
		if len(byName) > 0 {
			p.directories[directory] = byName
			p.order = append(p.order, directory)
		}
	}
	slices.SortFunc(p.order, func(a, b string) int {
		if n := strings.Count(a, "/") - strings.Count(b, "/"); n != 0 {
			return n
		}
		return strings.Compare(a, b)
	})
	return p
}

func isManifest(base string) bool {
	for _, n := range manifestNames {
		if base == n {
			return true
		}
	}
	return false
}

// vcpkgConfig is the registry configuration, in vcpkg-configuration.json or the
// manifest's own "vcpkg-configuration". Only whether a baseline fixes the
// registry's versions is read.
type vcpkgConfig struct {
	DefaultRegistry *struct {
		Baseline string `json:"baseline"`
	} `json:"default-registry"`
	Registries []struct {
		Baseline string   `json:"baseline"`
		Packages []string `json:"packages"`
	} `json:"registries"`
}

// baseline reports whether the configuration fixes the port's versions to a
// registry commit: the default registry's, or a registry claiming the port.
func (c *vcpkgConfig) baseline(port string) bool {
	if c == nil {
		return false
	}
	for _, r := range c.Registries {
		for _, pattern := range r.Packages {
			if ok, _ := path.Match(pattern, port); ok {
				return r.Baseline != ""
			}
		}
	}
	return c.DefaultRegistry != nil && c.DefaultRegistry.Baseline != ""
}

// readVcpkg reads a vcpkg.json. A dependency, of the manifest or of one of its
// features, is a port name or an object with a name, a "version>=" minimum,
// features and a platform; "overrides" set a port's
// version exactly, which is the only thing that pins one (see REQ-CPP-009).
//
// Implements: REQ-CPP-009
func readVcpkg(source, config []byte) []*declaredPackage {
	var m struct {
		Dependencies []json.RawMessage `json:"dependencies"`
		// A feature's dependencies are installed when the feature is chosen,
		// which the manifest does not say; they count as declared.
		Features  map[string]json.RawMessage `json:"features"`
		Overrides []struct {
			Name          string `json:"name"`
			Version       string `json:"version"`
			VersionSemver string `json:"version-semver"`
			VersionDate   string `json:"version-date"`
			VersionString string `json:"version-string"`
		} `json:"overrides"`
		BuiltinBaseline string       `json:"builtin-baseline"`
		Configuration   *vcpkgConfig `json:"vcpkg-configuration"`
	}
	if json.Unmarshal(source, &m) != nil {
		return nil
	}
	configuration := m.Configuration
	if len(config) > 0 {
		var c vcpkgConfig
		if json.Unmarshal(config, &c) == nil {
			configuration = &c
		}
	}
	overrides := map[string]string{}
	for _, o := range m.Overrides {
		for _, v := range []string{o.Version, o.VersionSemver, o.VersionDate, o.VersionString} {
			if v != "" {
				overrides[o.Name] = v
				break
			}
		}
	}
	dependencies := m.Dependencies
	for _, name := range slices.Sorted(maps.Keys(m.Features)) {
		var f struct {
			Dependencies []json.RawMessage `json:"dependencies"`
		}
		if json.Unmarshal(m.Features[name], &f) == nil {
			dependencies = append(dependencies, f.Dependencies...)
		}
	}
	var out []*declaredPackage
	seen := map[string]bool{}
	for _, raw := range dependencies {
		var name, minimum string
		if json.Unmarshal(raw, &name) != nil {
			var d struct {
				Name    string `json:"name"`
				Minimum string `json:"version>="`
			}
			if json.Unmarshal(raw, &d) != nil {
				continue
			}
			name, minimum = d.Name, d.Minimum
		}
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		d := &declaredPackage{ecosystem: ecosystemVcpkg, name: name}
		if minimum != "" {
			d.version = ">=" + minimum
		}
		if v, ok := overrides[name]; ok {
			d.requested, d.version, d.pinned = d.version, v, true
		} else if minimum == "" && m.BuiltinBaseline == "" && !configuration.baseline(name) {
			// No version, and no baseline to take one from: whatever the vcpkg
			// checkout that builds it has.
			d.floating = true
		}
		out = append(out, d)
	}
	return out
}

// ConanReference is a Conan recipe reference, name/version@user/channel#revision.
type ConanReference struct{ Name, Version, User, Channel, Revision string }

// ParseConanReference reads a Conan reference, name/version@user/channel#revision
// (a Conan 2 lock appends %timestamp, and "name/version@" says there is no user
// or channel). ok is false for anything that is no plain reference: a name or
// version built at run time ({}, $), quoted, or missing.
//
// Implements: REQ-CPP-010
func ParseConanReference(reference string) (r ConanReference, ok bool) {
	reference = strings.TrimSpace(reference)
	reference, _, _ = strings.Cut(reference, "%")
	reference, r.Revision, _ = strings.Cut(reference, "#")
	reference, qualifier, _ := strings.Cut(reference, "@")
	r.User, r.Channel, _ = strings.Cut(qualifier, "/")
	name, version, ok := strings.Cut(reference, "/")
	r.Name, r.Version = strings.TrimSpace(name), strings.TrimSpace(version)
	r.User, r.Channel, r.Revision = strings.TrimSpace(r.User), strings.TrimSpace(r.Channel), strings.TrimSpace(r.Revision)
	if r.User == "_" && r.Channel == "_" || r.User == "_" && r.Channel == "" {
		r.User, r.Channel = "", "" // how a server spells none
	}
	if !ok || r.Name == "" || r.Version == "" || strings.ContainsAny(r.Name, " {}$\"'") ||
		strings.ContainsAny(r.Version, "{}$\"'") || strings.ContainsAny(r.User+r.Channel+r.Revision, " /{}$\"'") {
		return ConanReference{}, false
	}
	return r, true
}

// Qualifier is what the reference says beyond its name and version:
// "@user/channel" when it has a user, then "#revision" when it has one. It is
// the lang.Target.Registry of a Conan package, which is where a Conan remote
// looks the recipe up.
func (r ConanReference) Qualifier() string {
	out := ""
	if r.User != "" {
		out = "@" + r.User
		if r.Channel != "" {
			out += "/" + r.Channel
		}
	}
	if r.Revision != "" {
		out += "#" + r.Revision
	}
	return out
}

// conanPackage is a declared reference: an exact version pins it, a range in
// brackets ([>=1.0 <2], [~1.2]) floats until a lock resolves it.
//
// Implements: REQ-CPP-010
func conanPackage(reference string) *declaredPackage {
	r, ok := ParseConanReference(reference)
	if !ok {
		return nil
	}
	return conanDeclared(r)
}

func conanDeclared(r ConanReference) *declaredPackage {
	return &declaredPackage{ecosystem: ecosystemConan, name: r.Name, version: r.Version, qualifier: r.Qualifier(),
		pinned: !strings.HasPrefix(r.Version, "[")}
}

// readConanfileTxt reads the references under [requires], [tool_requires],
// [build_requires] and [test_requires].
//
// Implements: REQ-CPP-010
func readConanfileTxt(source []byte) []*declaredPackage {
	var out []*declaredPackage
	section := ""
	for _, line := range strings.Split(string(source), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]"):
			section = strings.Trim(line, "[] ")
		case section == "requires" || section == "tool_requires" || section == "build_requires" || section == "test_requires":
			if d := conanPackage(line); d != nil {
				out = append(out, d)
			}
		}
	}
	return out
}

var (
	// self.requires("fmt/10.2.1"), self.tool_requires('cmake/[>=3.20]'): the
	// first argument when it is a string literal (an f-string only when it
	// substitutes nothing, which ParseConanReference checks).
	conanCall = regexp.MustCompile(`self\.(requires|tool_requires|build_requires|test_requires)\(\s*[fF]?["']([^"']+)["']`)
	// requires = "a/1", "b/2" or a list or tuple, possibly over several lines.
	conanAttribute = regexp.MustCompile(`(?m)^[ \t]*(requires|tool_requires|build_requires|test_requires)[ \t]*=[ \t]*`)
	pyString       = regexp.MustCompile(`"([^"\n]*)"|'([^'\n]*)'`)
)

// ConanRequirement is one reference a recipe requires, with how: "requires",
// "tool_requires", "build_requires" or "test_requires".
type ConanRequirement struct {
	Kind      string
	Reference ConanReference
}

// RecipeRequirements reads the string literals a conanfile.py requires with, in
// self.requires() and its kin, and in the requires, tool_requires,
// build_requires and test_requires attributes, in that order. The recipe is not
// run: a reference built at run time (an f-string, a variable) is not seen, and
// a conditional one counts whatever the condition.
//
// Implements: REQ-CPP-010, REQ-CPP-013
func RecipeRequirements(source []byte) []ConanRequirement {
	text := stripPyComments(string(source))
	var out []ConanRequirement
	add := func(kind, reference string) {
		if r, ok := ParseConanReference(reference); ok {
			out = append(out, ConanRequirement{Kind: kind, Reference: r})
		}
	}
	for _, m := range conanCall.FindAllStringSubmatch(text, -1) {
		add(m[1], m[2])
	}
	for _, span := range conanAttribute.FindAllStringSubmatchIndex(text, -1) {
		for _, m := range pyString.FindAllStringSubmatch(pyValue(text[span[1]:]), -1) {
			add(text[span[2]:span[3]], m[1]+m[2])
		}
	}
	return out
}

// readConanfilePy reads what a conanfile.py requires (RecipeRequirements), of
// every kind.
//
// Implements: REQ-CPP-010, REQ-CPP-013
func readConanfilePy(source []byte) []*declaredPackage {
	var out []*declaredPackage
	for _, requirement := range RecipeRequirements(source) {
		out = append(out, conanDeclared(requirement.Reference))
	}
	return out
}

// pyValue is the right-hand side of an assignment: up to the bracket closing the
// one it opens with, or else the end of the line.
func pyValue(s string) string {
	if s == "" || (s[0] != '[' && s[0] != '(') {
		line, _, _ := strings.Cut(s, "\n")
		return line
	}
	depth := 0
	var quote byte
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '[' || c == '(':
			depth++
		case c == ']' || c == ')':
			if depth--; depth == 0 {
				return s[:i+1]
			}
		}
	}
	return s
}

// stripPyComments blanks what follows a # outside a string on each line, so a
// commented-out requirement is not read.
func stripPyComments(s string) string {
	lines := strings.Split(s, "\n")
	for n, line := range lines {
		var quote byte
		for i := 0; i < len(line); i++ {
			c := line[i]
			if quote != 0 {
				if c == quote {
					quote = 0
				}
				continue
			}
			if c == '"' || c == '\'' {
				quote = c
			} else if c == '#' {
				lines[n] = line[:i]
				break
			}
		}
	}
	return strings.Join(lines, "\n")
}

// conanLock is a conan.lock: Conan 2's flat lists, or Conan 1's graph_lock.
type conanLock struct {
	GraphLock *struct {
		Nodes map[string]struct {
			Reference     string   `json:"ref"`
			Path          string   `json:"path"`
			Requires      []string `json:"requires"`
			BuildRequires []string `json:"build_requires"`
		} `json:"nodes"`
	} `json:"graph_lock"`
	Requires       []string `json:"requires"`
	BuildRequires  []string `json:"build_requires"`
	PythonRequires []string `json:"python_requires"`
}

func readConanLock(source []byte) (l conanLock, ok bool) {
	return l, json.Unmarshal(source, &l) == nil
}

// references are the exact references the lock pins: a Conan 2 lock's
// requires, then its build_requires; a Conan 1 lock's nodes by id (in the order
// of the ids, so which of two versions of one library counts does not change
// from run to run), the consumer left out.
func (l conanLock) references() []ConanReference {
	var out []ConanReference
	add := func(reference string) {
		if r, ok := ParseConanReference(reference); ok && !strings.HasPrefix(r.Version, "[") {
			out = append(out, r)
		}
	}
	for _, reference := range append(slices.Clone(l.Requires), l.BuildRequires...) {
		add(reference)
	}
	if l.GraphLock != nil {
		for _, id := range slices.Sorted(maps.Keys(l.GraphLock.Nodes)) {
			if n := l.GraphLock.Nodes[id]; n.Path == "" {
				add(n.Reference)
			}
		}
	}
	return out
}

// ConanLockReferences are the exact references a conan.lock pins, Conan 2's or
// Conan 1's, with their recipe revisions: what Conan installs in place of a
// requirement's range when it is given the lock.
//
// Implements: REQ-CPP-011
func ConanLockReferences(source []byte) []ConanReference {
	l, ok := readConanLock(source)
	if !ok {
		return nil
	}
	return l.references()
}

// lock applies a conan.lock to the references declared beside it: the locked
// version replaces a range, and what the lock holds beyond them (the libraries
// the declared ones need) is installed too and so declared. It reads Conan 2
// locks ("requires", "build_requires" lists) and Conan 1 locks ("graph_lock"
// nodes), whose edges it also keeps for the transitive walk. flat is a Conan 2
// lock that pinned something: it has no edges to keep.
//
// Implements: REQ-CPP-011
func (p *packages) lock(source []byte, declared []*declaredPackage) (_ []*declaredPackage, flat bool) {
	l, ok := readConanLock(source)
	if !ok {
		return declared, false
	}
	var locked []*declaredPackage
	for _, r := range l.references() {
		locked = append(locked, conanDeclared(r))
	}
	if l.GraphLock != nil {
		for _, id := range slices.Sorted(maps.Keys(l.GraphLock.Nodes)) {
			n := l.GraphLock.Nodes[id]
			d := conanPackage(n.Reference)
			if n.Path != "" || d == nil || !d.pinned {
				continue
			}
			key := d.name + "/" + d.version
			for _, requirement := range append(n.Requires, n.BuildRequires...) {
				if dependency := conanPackage(l.GraphLock.Nodes[requirement].Reference); dependency != nil {
					p.tree[key] = append(p.tree[key], dependency.target())
				}
			}
		}
	}
	byName := map[string]*declaredPackage{}
	for _, d := range locked {
		if byName[normalizeName(d.name)] == nil {
			byName[normalizeName(d.name)] = d
		}
	}
	for _, d := range declared {
		if v := byName[normalizeName(d.name)]; v != nil {
			if v.version != d.version {
				d.requested, d.version = d.version, v.version
			}
			d.pinned, d.qualifier = true, v.qualifier
			delete(byName, normalizeName(d.name))
		}
	}
	for _, d := range locked { // in the lock's order, once each
		if byName[normalizeName(d.name)] == d {
			declared = append(declared, d)
		}
	}
	return declared, l.GraphLock == nil && len(locked) > 0
}

// Dependencies implements lang.Transitive for Conan packages a Conan 1 lock gives
// the graph of.
//
// Implements: REQ-CPP-011
func (p *packages) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecosystemConan {
		return nil
	}
	return p.tree[t.Package+"/"+t.Version]
}

// headerAliases name the packages that provide an include directory (or a bare
// header's name) other than the one named like it, as normalizeName spells both. Only
// the well-known cases: anything else must be named like its headers.
//
// Implements: REQ-CPP-012
var headerAliases = map[string][]string{
	// cSpell: disable
	"gtest": {"googletest"}, "gmock": {"gtest", "googletest"},
	"nlohmann": {"nlohmann-json"}, "eigen": {"eigen3"}, "unsupported": {"eigen3", "eigen"},
	"sdl": {"sdl2"}, "sdl2": {"sdl"}, "glfw": {"glfw3"}, "catch": {"catch2"},
	"google": {"protobuf"}, "absl": {"abseil"}, "grpcpp": {"grpc"},
	"zconf": {"zlib"}, "libxml": {"libxml2"}, "event2": {"libevent"},
	"jpeglib": {"libjpeg-turbo", "libjpeg"}, "turbojpeg": {"libjpeg-turbo"},
	"bzlib": {"bzip2"}, "lzma": {"liblzma", "xz-utils"}, "yaml": {"libyaml"},
	"zmq": {"cppzmq", "zeromq"}, "zip": {"libzip", "kuba-zip"}, "archive-entry": {"libarchive"}, "tbb": {"onetbb"}, "oneapi": {"onetbb", "tbb"},
	"ft2build": {"freetype"}, "gsl": {"ms-gsl"}, "range": {"range-v3"},
	"httplib": {"cpp-httplib"}, "pqxx": {"libpqxx"}, "libpq-fe": {"libpq"},
	"opencv2": {"opencv", "opencv4"}, "sol": {"sol2"}, "xercesc": {"xerces-c"},
	"libavcodec": {"ffmpeg"}, "libavformat": {"ffmpeg"}, "libavutil": {"ffmpeg"},
	"libswscale": {"ffmpeg"}, "libswresample": {"ffmpeg"},
	// cSpell: enable
}

// candidates are the package names, in normName's spelling and most specific
// first, that may provide an include: for Boost the port of its second directory
// (<boost/asio/...> is vcpkg's boost-asio) before boost; the library's own name;
// the aliases; the name with "lib" before it (<curl/curl.h> is Conan's libcurl,
// <uv.h> is libuv); and for a Qt module's directory (<QtCore/QString>) its own
// port, then Qt's base.
func candidates(include string) []string {
	libraryName := normalizeName(library(include))
	var out []string
	if libraryName == "boost" {
		if _, rest, ok := strings.Cut(include, "/"); ok {
			second, _, _ := strings.Cut(rest, "/")
			out = append(out, "boost-"+normalizeName(strings.TrimSuffix(second, path.Ext(second))))
		}
	}
	out = append(out, libraryName)
	out = append(out, headerAliases[libraryName]...)
	if !strings.HasPrefix(libraryName, "lib") {
		out = append(out, "lib"+libraryName)
	}
	if strings.HasPrefix(include, "Qt") && len(libraryName) > 2 {
		out = append(out, "qtbase", "qt5-base", "qt6", "qt5", "qt")
	}
	return out
}

// matchNames finds the declared package an include belongs to by its candidate
// names: in the manifests nearest to the file first, the most specific candidate
// name first. A file with no manifest above it at all is looked up in the
// project's other manifests, since a project is often several directories built
// together under one manifest (src/app/conanfile.txt beside src/core/); a file
// under a manifest keeps to its own.
//
// Implements: REQ-CPP-012
func (p *packages) matchNames(file string, names []string) *declaredPackage {
	if len(p.directories) == 0 {
		return nil
	}
	in := func(directory string) *declaredPackage {
		for _, n := range names {
			if list := p.directories[directory][n]; len(list) > 0 {
				return list[0]
			}
		}
		return nil
	}
	governed := false
	for directory := path.Dir(file); ; directory = path.Dir(directory) {
		if d := in(directory); d != nil {
			return d
		}
		governed = governed || p.directories[directory] != nil
		if directory == "." || directory == "/" {
			break
		}
	}
	if governed {
		return nil
	}
	for _, directory := range p.order {
		if d := in(directory); d != nil {
			return d
		}
	}
	return nil
}

// Packages is what the project's vcpkg and Conan manifests declare, for the plugins
// of C and C++ build systems, whose libraries must land on the nodes the includes
// of the sources land on.
type Packages struct{ p *packages }

// ReadPackages reads the vcpkg and Conan manifests of the project.
func ReadPackages(all []*scan.File) Packages { return Packages{readPackages(all)} }

// Library is what a third-party include (<boost/asio.hpp>, <zlib.h>) is attributed
// to from file: the package a manifest over it declares - tried by the include's
// candidate names, then by extra names (another spelling of the library) - else
// content the build fetches under one of those names, or else, unresolved, the
// c-external library named after the include (REQ-CPP-006).
//
// Implements: REQ-CPP-006, REQ-CPP-012, REQ-CPP-017, REQ-CMAKE-006
func (p Packages) Library(file, include string, extra ...string) lang.Target {
	names := candidates(include)
	for _, e := range extra {
		if n := normalizeName(e); !slices.Contains(names, n) {
			names = append(names, n)
		}
	}
	if d := p.p.matchNames(file, names); d != nil {
		return d.target()
	}
	if f := p.p.fetchedFor(file, names); f != nil {
		return f.Target
	}
	return lang.Target{Ecosystem: ecosystemExternal, Package: library(include), Unresolved: true}
}

// PackageEcosystems are the islands Library attributes libraries to, as the cpp
// plugin declares them.
func PackageEcosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecosystemVcpkg, Name: "vcpkg"},
		{ID: ecosystemConan, Name: "Conan"},
		{ID: ecosystemExternal, Name: "C/C++ external"},
	}
}

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
	ecoVcpkg = "vcpkg"
	ecoConan = "conan"
)

// pkg is one library a vcpkg or Conan manifest declares, or a conan.lock holds.
type pkg struct {
	eco, name string
	version   string // exact when pinned; else the minimum or range declared
	requested string // what the manifest asked for when a lock or override fixed it
	pinned    bool
	floating  bool // names no version, and nothing fixes one
}

func (p *pkg) target() lang.Target {
	return lang.Target{
		Ecosystem: p.eco, Package: p.name, Version: p.version, Requested: p.requested,
		Pinned: p.pinned, Floating: p.floating,
	}
}

// packages are the libraries declared per directory holding a manifest; a source
// sees those of the directories above it, the nearest first.
type packages struct {
	dirs map[string]map[string][]*pkg // dir -> normalized name -> vcpkg first, then Conan
	// order is every manifest directory, shallowest first, then by name: where the
	// includes of a file under no manifest are looked for.
	order []string
	// tree is what a Conan 1 conan.lock (graph_lock) says each "name/version"
	// requires. A Conan 2 lock is a flat list and gives no edges.
	tree map[string][]lang.Target
	// flat are the Conan 2 locks that pinned something, for the note that says
	// they give no edges.
	flat []string
}

// normName folds the spellings one library goes by in vcpkg, Conan and its include
// directory: case, and "_" for "-" (nlohmann_json, nlohmann-json).
func normName(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), "_", "-")
}

// manifestNames are the files read; the order within one directory matters, since a
// lock refines what a conanfile declared and a configuration's baseline applies
// to vcpkg.json.
var manifestNames = []string{"vcpkg-configuration.json", "vcpkg.json", "conanfile.txt", "conanfile.py", "conan.lock"}

// Implements: REQ-CPP-009, REQ-CPP-010, REQ-CPP-011
func readPackages(all []*scan.File) *packages {
	p := &packages{dirs: map[string]map[string][]*pkg{}, tree: map[string][]lang.Target{}}
	byDir := map[string]map[string][]byte{}
	for _, f := range all {
		base := path.Base(f.Path)
		if !isManifest(base) || f.Binary || f.TooLarge || f.Size > lang.MaxParseSize {
			continue
		}
		src, err := os.ReadFile(f.Abs)
		if err != nil {
			continue
		}
		dir := path.Dir(f.Path)
		if byDir[dir] == nil {
			byDir[dir] = map[string][]byte{}
		}
		byDir[dir][base] = src
	}
	for dir, files := range byDir {
		var vcpkg, conan []*pkg
		if src, ok := files["vcpkg.json"]; ok {
			vcpkg = readVcpkg(src, files["vcpkg-configuration.json"])
		}
		if src, ok := files["conanfile.txt"]; ok {
			conan = append(conan, readConanfileTxt(src)...)
		}
		if src, ok := files["conanfile.py"]; ok {
			conan = append(conan, readConanfilePy(src)...)
		}
		if src, ok := files["conan.lock"]; ok {
			var flat bool
			if conan, flat = p.lock(src, conan); flat {
				p.flat = append(p.flat, path.Join(dir, "conan.lock"))
			}
		}
		byName := map[string][]*pkg{}
		for _, list := range [][]*pkg{vcpkg, conan} {
			for _, d := range list {
				key := normName(d.name)
				byName[key] = append(byName[key], d)
			}
		}
		if len(byName) > 0 {
			p.dirs[dir] = byName
			p.order = append(p.order, dir)
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
func readVcpkg(src, config []byte) []*pkg {
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
	if json.Unmarshal(src, &m) != nil {
		return nil
	}
	cfg := m.Configuration
	if len(config) > 0 {
		var c vcpkgConfig
		if json.Unmarshal(config, &c) == nil {
			cfg = &c
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
	deps := m.Dependencies
	for _, name := range slices.Sorted(maps.Keys(m.Features)) {
		var f struct {
			Dependencies []json.RawMessage `json:"dependencies"`
		}
		if json.Unmarshal(m.Features[name], &f) == nil {
			deps = append(deps, f.Dependencies...)
		}
	}
	var out []*pkg
	seen := map[string]bool{}
	for _, raw := range deps {
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
		d := &pkg{eco: ecoVcpkg, name: name}
		if minimum != "" {
			d.version = ">=" + minimum
		}
		if v, ok := overrides[name]; ok {
			d.requested, d.version, d.pinned = d.version, v, true
		} else if minimum == "" && m.BuiltinBaseline == "" && !cfg.baseline(name) {
			// No version, and no baseline to take one from: whatever the vcpkg
			// checkout that builds it has.
			d.floating = true
		}
		out = append(out, d)
	}
	return out
}

// conanRef splits a Conan reference, name/version@user/channel#revision (a Conan
// 2 lock appends %timestamp), into its name and version.
func conanRef(ref string) (name, version string, ok bool) {
	ref = strings.TrimSpace(ref)
	ref, _, _ = strings.Cut(ref, "#")
	ref, _, _ = strings.Cut(ref, "%")
	ref, _, _ = strings.Cut(ref, "@")
	name, version, ok = strings.Cut(ref, "/")
	name, version = strings.TrimSpace(name), strings.TrimSpace(version)
	if !ok || name == "" || version == "" || strings.ContainsAny(name, " {}$\"'") || strings.ContainsAny(version, "{}$\"'") {
		return "", "", false
	}
	return name, version, true
}

// conanPkg is a declared reference: an exact version pins it, a range in
// brackets ([>=1.0 <2], [~1.2]) floats until a lock resolves it.
//
// Implements: REQ-CPP-010
func conanPkg(ref string) *pkg {
	name, version, ok := conanRef(ref)
	if !ok {
		return nil
	}
	return &pkg{eco: ecoConan, name: name, version: version, pinned: !strings.HasPrefix(version, "[")}
}

// readConanfileTxt reads the references under [requires], [tool_requires],
// [build_requires] and [test_requires].
//
// Implements: REQ-CPP-010
func readConanfileTxt(src []byte) []*pkg {
	var out []*pkg
	section := ""
	for _, line := range strings.Split(string(src), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]"):
			section = strings.Trim(line, "[] ")
		case section == "requires" || section == "tool_requires" || section == "build_requires" || section == "test_requires":
			if d := conanPkg(line); d != nil {
				out = append(out, d)
			}
		}
	}
	return out
}

var (
	// self.requires("fmt/10.2.1"), self.tool_requires('cmake/[>=3.20]'): the
	// first argument when it is a plain string literal (an f-string is not).
	conanCall = regexp.MustCompile(`self\.(?:requires|tool_requires|build_requires|test_requires)\(\s*["']([^"']+)["']`)
	// requires = "a/1", "b/2" or a list or tuple, possibly over several lines.
	conanAttr = regexp.MustCompile(`(?m)^[ \t]*(?:requires|tool_requires|build_requires|test_requires)[ \t]*=[ \t]*`)
	pyString  = regexp.MustCompile(`"([^"\n]*)"|'([^'\n]*)'`)
)

// readConanfilePy reads the string literals a conanfile.py requires with, in
// self.requires() and its kin, and in the requires, tool_requires, build_requires
// and test_requires attributes. The recipe is not run: a reference built at run
// time (an f-string, a variable) is not seen, and a conditional one counts
// whatever the condition.
//
// Implements: REQ-CPP-010, REQ-CPP-013
func readConanfilePy(src []byte) []*pkg {
	text := stripPyComments(string(src))
	var out []*pkg
	add := func(ref string) {
		if d := conanPkg(ref); d != nil {
			out = append(out, d)
		}
	}
	for _, m := range conanCall.FindAllStringSubmatch(text, -1) {
		add(m[1])
	}
	for _, loc := range conanAttr.FindAllStringIndex(text, -1) {
		for _, m := range pyString.FindAllStringSubmatch(pyValue(text[loc[1]:]), -1) {
			add(m[1] + m[2])
		}
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

// lock applies a conan.lock to the references declared beside it: the locked
// version replaces a range, and what the lock holds beyond them (the libraries
// the declared ones need) is installed too and so declared. It reads Conan 2
// locks ("requires", "build_requires" lists) and Conan 1 locks ("graph_lock"
// nodes), whose edges it also keeps for the transitive walk. flat is a Conan 2
// lock that pinned something: it has no edges to keep.
//
// Implements: REQ-CPP-011
func (p *packages) lock(src []byte, declared []*pkg) (_ []*pkg, flat bool) {
	var l struct {
		GraphLock *struct {
			Nodes map[string]struct {
				Ref           string   `json:"ref"`
				Path          string   `json:"path"`
				Requires      []string `json:"requires"`
				BuildRequires []string `json:"build_requires"`
			} `json:"nodes"`
		} `json:"graph_lock"`
		Requires       []string `json:"requires"`
		BuildRequires  []string `json:"build_requires"`
		PythonRequires []string `json:"python_requires"`
	}
	if json.Unmarshal(src, &l) != nil {
		return declared, false
	}
	var locked []*pkg
	for _, ref := range append(l.Requires, l.BuildRequires...) {
		if d := conanPkg(ref); d != nil && d.pinned {
			locked = append(locked, d)
		}
	}
	if l.GraphLock != nil {
		// In the order of the node ids, so which of two versions of one library
		// counts does not change from run to run.
		ids := slices.Sorted(maps.Keys(l.GraphLock.Nodes))
		for _, id := range ids {
			n := l.GraphLock.Nodes[id]
			if n.Path != "" { // the consumer: the conanfile itself
				continue
			}
			d := conanPkg(n.Ref)
			if d == nil || !d.pinned {
				continue
			}
			locked = append(locked, d)
			key := d.name + "/" + d.version
			for _, req := range append(n.Requires, n.BuildRequires...) {
				if dep := conanPkg(l.GraphLock.Nodes[req].Ref); dep != nil {
					p.tree[key] = append(p.tree[key], dep.target())
				}
			}
		}
	}
	byName := map[string]*pkg{}
	for _, d := range locked {
		if byName[normName(d.name)] == nil {
			byName[normName(d.name)] = d
		}
	}
	for _, d := range declared {
		if v := byName[normName(d.name)]; v != nil {
			if v.version != d.version {
				d.requested, d.version = d.version, v.version
			}
			d.pinned = true
			delete(byName, normName(d.name))
		}
	}
	for _, d := range locked { // in the lock's order, once each
		if byName[normName(d.name)] == d {
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
	if t.Ecosystem != ecoConan {
		return nil
	}
	return p.tree[t.Package+"/"+t.Version]
}

// headerAliases name the packages that provide an include directory (or a bare
// header's name) other than the one named like it, as normName spells both. Only
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
	lib := normName(library(include))
	var out []string
	if lib == "boost" {
		if _, rest, ok := strings.Cut(include, "/"); ok {
			second, _, _ := strings.Cut(rest, "/")
			out = append(out, "boost-"+normName(strings.TrimSuffix(second, path.Ext(second))))
		}
	}
	out = append(out, lib)
	out = append(out, headerAliases[lib]...)
	if !strings.HasPrefix(lib, "lib") {
		out = append(out, "lib"+lib)
	}
	if strings.HasPrefix(include, "Qt") && len(lib) > 2 {
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
func (p *packages) matchNames(file string, names []string) *pkg {
	if len(p.dirs) == 0 {
		return nil
	}
	in := func(dir string) *pkg {
		for _, n := range names {
			if list := p.dirs[dir][n]; len(list) > 0 {
				return list[0]
			}
		}
		return nil
	}
	governed := false
	for dir := path.Dir(file); ; dir = path.Dir(dir) {
		if d := in(dir); d != nil {
			return d
		}
		governed = governed || p.dirs[dir] != nil
		if dir == "." || dir == "/" {
			break
		}
	}
	if governed {
		return nil
	}
	for _, dir := range p.order {
		if d := in(dir); d != nil {
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
// candidate names, then by extra names (another spelling of the library) - or else,
// unresolved, the c-external library named after the include (REQ-CPP-006).
//
// Implements: REQ-CPP-006, REQ-CPP-012, REQ-CMAKE-006
func (p Packages) Library(file, include string, extra ...string) lang.Target {
	names := candidates(include)
	for _, e := range extra {
		if n := normName(e); !slices.Contains(names, n) {
			names = append(names, n)
		}
	}
	if d := p.p.matchNames(file, names); d != nil {
		return d.target()
	}
	return lang.Target{Ecosystem: ecoExternal, Package: library(include), Unresolved: true}
}

// PackageEcosystems are the islands Library attributes libraries to, as the cpp
// plugin declares them.
func PackageEcosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecoVcpkg, Name: "vcpkg"},
		{ID: ecoConan, Name: "Conan"},
		{ID: ecoExternal, Name: "C/C++ external"},
	}
}

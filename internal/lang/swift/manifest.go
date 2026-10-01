package swift

import (
	"cmp"
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/chars"
)

// Package.swift is Swift code, but its dependencies and targets are written as calls
// with literal arguments, and the grammar's error recovery reads some of those calls
// differently from their neighbors (`.package(url: x, .upToNextMajor(from: y))`
// becomes a constructor call). They are read here from the text instead: comments
// blanked, calls found by name, arguments split at the top level.

// call is one `.name(...)` in a manifest: its arguments split at top-level commas,
// and the line it starts on.
type call struct {
	arguments []string
	line      int
}

// stripComments replaces comments with spaces, keeping line breaks and string
// literals ("…", """…""" and #"…"#) as they are.
func stripComments(source string) string {
	b := []byte(source)
	for i := 0; i < len(b); i++ {
		switch {
		case b[i] == '"':
			i = skipString(b, i) - 1
		case b[i] == '/' && i+1 < len(b) && b[i+1] == '/':
			for ; i < len(b) && b[i] != '\n'; i++ {
				b[i] = ' '
			}
		case b[i] == '/' && i+1 < len(b) && b[i+1] == '*':
			depth := 0
			for ; i < len(b); i++ {
				if b[i] == '/' && i+1 < len(b) && b[i+1] == '*' {
					depth++
					b[i], b[i+1] = ' ', ' '
					i++
					continue
				}
				if b[i] == '*' && i+1 < len(b) && b[i+1] == '/' {
					depth--
					b[i], b[i+1] = ' ', ' '
					i++
					if depth == 0 {
						break
					}
					continue
				}
				if b[i] != '\n' {
					b[i] = ' '
				}
			}
		}
	}
	return string(b)
}

// skipString returns the index just past the string literal starting at b[i].
func skipString(b []byte, i int) int {
	if i+2 < len(b) && b[i+1] == '"' && b[i+2] == '"' {
		for j := i + 3; j+2 < len(b); j++ {
			if b[j] == '\\' {
				j++
				continue
			}
			if b[j] == '"' && b[j+1] == '"' && b[j+2] == '"' {
				return j + 3
			}
		}
		return len(b)
	}
	for j := i + 1; j < len(b); j++ {
		switch b[j] {
		case '\\':
			j++
		case '"', '\n':
			return j + 1
		}
	}
	return len(b)
}

// calls finds every `.name(` in src (comments already stripped) not preceded by an
// identifier - `.package(` in a list, not `foo.package(` - and returns its arguments.
func calls(source, name string) []call {
	var out []call
	b := []byte(source)
	needle := "." + name
	for i := 0; ; {
		j := strings.Index(source[i:], needle)
		if j < 0 {
			return out
		}
		at := i + j
		i = at + len(needle)
		if at > 0 && chars.IsWord(b[at-1]) || i < len(b) && chars.IsWord(b[i]) {
			continue
		}
		k := i
		for k < len(b) && (b[k] == ' ' || b[k] == '\t' || b[k] == '\n' || b[k] == '\r') {
			k++
		}
		if k >= len(b) || b[k] != '(' {
			continue
		}
		end := closing(b, k)
		if end < 0 {
			return out
		}
		out = append(out, call{arguments: splitArguments(source[k+1 : end]), line: strings.Count(source[:at], "\n") + 1})
	}
}

// closing returns the index of the bracket closing the one at b[open], or -1.
func closing(b []byte, open int) int {
	depth := 0
	for i := open; i < len(b); i++ {
		switch b[i] {
		case '"':
			i = skipString(b, i) - 1
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// splitArguments splits an argument list at its top-level commas, each argument trimmed.
func splitArguments(s string) []string {
	var out []string
	b := []byte(s)
	depth, start := 0, 0
	for i := 0; i < len(b); i++ {
		switch b[i] {
		case '"':
			i = skipString(b, i) - 1
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	if last := strings.TrimSpace(s[start:]); last != "" {
		out = append(out, last)
	}
	return out
}

// label splits "name: value" into its label and value; an unlabeled argument has no
// label.
func label(argument string) (string, string) {
	i := 0
	for i < len(argument) && chars.IsWord(argument[i]) {
		i++
	}
	if i > 0 && i < len(argument) && argument[i] == ':' {
		return argument[:i], strings.TrimSpace(argument[i+1:])
	}
	return "", argument
}

// labeled returns the value of the argument with the given label.
func labeled(arguments []string, name string) (string, bool) {
	for _, a := range arguments {
		if l, v := label(a); l == name {
			return v, true
		}
	}
	return "", false
}

// literal reads a plain string literal; one with interpolation is not plain.
func literal(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' || strings.HasPrefix(s, `"""`) {
		return "", false
	}
	inner := s[1 : len(s)-1]
	if strings.Contains(inner, `\(`) || strings.Contains(inner, `"`) {
		return "", false
	}
	return inner, true
}

// dependency is one `.package(...)` of a manifest.
type dependency struct {
	url, path, id string // exactly one is set
	name          string // the name: label of manifests before tools 5.2, else ""
	requirement   string // as shown: "2.60.0..<3.0.0", "1.2.3", "branch main"
	pinned        bool   // exact version or revision
	floating      bool   // a branch: names no version at all
	line          int
}

// dependencies reads the `.package(...)` calls of a manifest (comments stripped).
//
// Implements: REQ-SWIFT-006, REQ-SWIFT-008
func dependencies(source string) []dependency {
	var out []dependency
	for _, c := range calls(source, "package") {
		d := dependency{line: c.line}
		if v, ok := labeled(c.arguments, "url"); ok {
			d.url, _ = literal(v)
		}
		if v, ok := labeled(c.arguments, "path"); ok {
			d.path, _ = literal(v)
		}
		if v, ok := labeled(c.arguments, "id"); ok {
			d.id, _ = literal(v)
		}
		if v, ok := labeled(c.arguments, "name"); ok {
			d.name, _ = literal(v)
		}
		if d.url == "" && d.path == "" && d.id == "" {
			continue
		}
		d.requirement, d.pinned, d.floating = requirement(c.arguments)
		out = append(out, d)
	}
	return out
}

// PackageDependency is one package a manifest depends on, for the index client:
// a registry package by its identity (ID, scope.name) or a package in a
// repository by its URL, with its requirement as the plugin shows it
// ("1.2.3", "2.60.0..<3.0.0", "branch main").
type PackageDependency struct {
	ID, URL, Requirement string
}

// ManifestDependencies reads the packages a Package.swift depends on, as the
// plugin reads its own manifests: `.package(id:)` and `.package(url:)`; a local
// `.package(path:)` is no package another machine could fetch and is left out.
//
// Implements: REQ-SUP-075
func ManifestDependencies(source string) []PackageDependency {
	var out []PackageDependency
	for _, d := range dependencies(stripComments(source)) {
		if d.id != "" || d.url != "" {
			out = append(out, PackageDependency{ID: d.id, URL: d.url, Requirement: d.requirement})
		}
	}
	return out
}

var rangeRequirement = regexp.MustCompile(`^"([^"]+)"\s*(\.\.<|\.\.\.)\s*"([^"]+)"$`)

// requirement reads a package's version requirement from its arguments: from:,
// exact:, branch:, revision:, a range literal, or one of the Requirement values
// (.upToNextMajor(from:), .upToNextMinor(from:), .exact, .branch, .revision). A
// bare version string is an exact version.
//
// Implements: REQ-SWIFT-008
func requirement(arguments []string) (string, bool, bool) {
	for _, a := range arguments {
		l, v := label(a)
		switch l {
		case "url", "path", "id", "name", "traits":
			continue
		}
		if l == "" && strings.HasPrefix(v, ".") {
			// .upToNextMajor(from: "1.0.0") and kin
			function, rest, ok := strings.Cut(v[1:], "(")
			if !ok {
				continue
			}
			inner := strings.TrimSuffix(strings.TrimSpace(rest), ")")
			_, inner = label(inner)
			l, v = function, inner
		}
		s, _ := literal(v)
		switch l {
		case "from", "upToNextMajor":
			if s != "" {
				return s + "..<" + nextMajor(s), false, false
			}
		case "upToNextMinor":
			if s != "" {
				return s + "..<" + nextMinor(s), false, false
			}
		case "exact":
			if s != "" {
				return s, true, false
			}
		case "revision":
			if s != "" {
				return s, true, false
			}
		case "branch":
			if s != "" {
				return "branch " + s, false, true
			}
		case "":
			if m := rangeRequirement.FindStringSubmatch(v); m != nil {
				return m[1] + m[2] + m[3], false, false
			}
			if s != "" {
				return s, true, false
			}
		}
	}
	return "", false, false
}

// nextMajor and nextMinor are the exclusive upper bounds of from: and upToNextMinor.
func nextMajor(v string) string {
	parts := strings.Split(v, ".")
	return bump(parts[0]) + ".0.0"
}

func nextMinor(v string) string {
	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		return bump(parts[0]) + ".0.0"
	}
	return parts[0] + "." + bump(parts[1]) + ".0"
}

func bump(s string) string {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return s
		}
		n = n*10 + int(c-'0')
	}
	return strconv.Itoa(n + 1)
}

// target is one target of a manifest.
type target struct {
	name, kind, path string
	products         map[string]string // product name -> package it comes from (.product(name:package:))
}

var targetKinds = []string{"target", "executableTarget", "testTarget", "macro", "plugin", "systemLibrary", "binaryTarget"}

// targets reads a manifest's targets and the products their dependencies name.
//
// Implements: REQ-SWIFT-004, REQ-SWIFT-007
func targets(source string) []target {
	var out []target
	for _, kind := range targetKinds {
		for _, c := range calls(source, kind) {
			v, ok := labeled(c.arguments, "name")
			if !ok {
				continue
			}
			t := target{kind: kind, products: map[string]string{}}
			if t.name, ok = literal(v); !ok || t.name == "" {
				continue
			}
			if v, ok := labeled(c.arguments, "path"); ok {
				t.path, _ = literal(v)
			}
			if dependencies, ok := labeled(c.arguments, "dependencies"); ok {
				for _, p := range calls(dependencies, "product") {
					name, _ := labeled(p.arguments, "name")
					packageName, _ := labeled(p.arguments, "package")
					n, ok1 := literal(name)
					k, ok2 := literal(packageName)
					if ok1 && ok2 {
						t.products[n] = k
					}
				}
			}
			out = append(out, t)
		}
	}
	return out
}

// pin is one package a Package.resolved records.
type pin struct {
	identity, location string
	version, revision  string
	branch             string
	registry           bool
}

// readResolved reads Package.resolved, format version 1 (object.pins with package and
// repositoryURL) and 2 or 3 (pins with identity and location).
//
// Implements: REQ-SWIFT-009
func readResolved(data []byte) []pin {
	type state struct {
		Branch   *string `json:"branch"`
		Revision string  `json:"revision"`
		Version  *string `json:"version"`
	}
	type rawPin struct {
		Identity      string `json:"identity"`
		Kind          string `json:"kind"`
		Location      string `json:"location"`
		Package       string `json:"package"`
		RepositoryURL string `json:"repositoryURL"`
		State         state  `json:"state"`
	}
	var doc struct {
		Pins   []rawPin `json:"pins"`
		Object struct {
			Pins []rawPin `json:"pins"`
		} `json:"object"`
	}
	if json.Unmarshal(data, &doc) != nil {
		return nil
	}
	var out []pin
	for _, p := range append(doc.Pins, doc.Object.Pins...) {
		q := pin{identity: p.Identity, location: p.Location, revision: p.State.Revision, registry: p.Kind == "registry"}
		q.location = cmp.Or(q.location, p.RepositoryURL)
		if q.identity == "" {
			q.identity = identity(q.location)
		}
		if p.State.Version != nil {
			q.version = *p.State.Version
		}
		if p.State.Branch != nil {
			q.branch = *p.State.Branch
		}
		if q.identity == "" {
			continue
		}
		q.identity = strings.ToLower(q.identity)
		out = append(out, q)
	}
	return out
}

// identity is SwiftPM's name for a package at a URL or path: the last path segment
// without ".git", lower case.
func identity(location string) string {
	s := strings.TrimRight(strings.TrimSpace(location), "/")
	if i := strings.LastIndexAny(s, "/:"); i >= 0 {
		s = s[i+1:]
	}
	return strings.ToLower(strings.TrimSuffix(s, ".git"))
}

// packageName is how a package at a URL is named on the map, and to OSV (ecosystem
// SwiftURL) and Trivy: lang.RepoName's spelling of the URL.
//
// Implements: REQ-SWIFT-006
func packageName(url string) string { return lang.RepositoryName(url) }

var (
	pbxObject = regexp.MustCompile(`(?m)^\s*([0-9A-Fa-f]{24})\b[^=\n]*=\s*\{\s*isa\s*=\s*(XCRemoteSwiftPackageReference|XCLocalSwiftPackageReference|XCSwiftPackageProductDependency)\s*;`)
	pbxField  = regexp.MustCompile(`(\w+)\s*=\s*("(?:[^"\\]|\\.)*"|[^;{]*?)\s*;`)
)

// xcodePackages reads the Swift packages an Xcode project references
// (project.pbxproj): remote references with their requirement, local references
// with their path, and the products its targets take from them.
//
// Implements: REQ-SWIFT-010
func xcodePackages(source string) (dependencies []dependency, products map[string]string) {
	products = map[string]string{}
	references := map[string]dependency{}
	type product struct{ name, reference string }
	var prods []product
	b := []byte(source)
	for _, m := range pbxObject.FindAllStringSubmatchIndex(source, -1) {
		open := strings.LastIndex(source[:m[1]], "{")
		end := closing(b, open)
		if end < 0 {
			continue
		}
		fields := map[string]string{}
		for _, f := range pbxField.FindAllStringSubmatch(source[open+1:end], -1) {
			v := strings.TrimSpace(f[2])
			v = unquote(v)
			if i := strings.Index(v, "/*"); i >= 0 {
				v = strings.TrimSpace(v[:i])
			}
			fields[f[1]] = v
		}
		id, isa := source[m[2]:m[3]], source[m[4]:m[5]]
		line := strings.Count(source[:m[2]], "\n") + 1
		switch isa {
		case "XCRemoteSwiftPackageReference":
			d := dependency{url: fields["repositoryURL"], line: line}
			switch fields["kind"] {
			case "upToNextMajorVersion":
				d.requirement = fields["minimumVersion"] + "..<" + nextMajor(fields["minimumVersion"])
			case "upToNextMinorVersion":
				d.requirement = fields["minimumVersion"] + "..<" + nextMinor(fields["minimumVersion"])
			case "versionRange":
				d.requirement = fields["minimumVersion"] + "..<" + fields["maximumVersion"]
			case "exactVersion":
				d.requirement, d.pinned = fields["version"], true
			case "revision":
				d.requirement, d.pinned = fields["revision"], true
			case "branch":
				d.requirement, d.floating = "branch "+fields["branch"], true
			}
			if d.url != "" {
				references[id] = d
			}
		case "XCLocalSwiftPackageReference":
			if p := fields["relativePath"]; p != "" {
				references[id] = dependency{path: p, line: line}
			}
		case "XCSwiftPackageProductDependency":
			if fields["productName"] != "" && fields["package"] != "" {
				prods = append(prods, product{fields["productName"], fields["package"]})
			}
		}
	}
	for _, p := range prods {
		if d, ok := references[p.reference]; ok && d.url != "" {
			products[p.name] = identity(d.url)
		}
	}
	for _, d := range references {
		dependencies = append(dependencies, d)
	}
	sort.Slice(dependencies, func(i, j int) bool { return dependencies[i].line < dependencies[j].line })
	return dependencies, products
}

// unquote reads a pbxproj string: quoted with backslash escapes, or a bare word.
func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		var out string
		if json.Unmarshal([]byte(s), &out) != nil {
			return s[1 : len(s)-1]
		}
		return out
	}
	return s
}

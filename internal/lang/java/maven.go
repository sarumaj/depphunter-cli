package java

import (
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// A Maven package on the map is an artifact, named group:artifact as POMs, Maven
// Central, OSV and Trivy name it, and as the Clojure and Bazel plugins name the
// artifacts they declare - so one library is one node whichever language reaches it.
// A Java import names a package, though, not the artifact that ships it; this file
// is how the one is found from the other.

// artifact is one Maven artifact a manifest declares - or, when virtual, one a
// declared artifact of its group brings along (knownArtifacts).
//
// Implements: REQ-JAVA-012
type artifact struct {
	group, name string // name as published: cats-effect_3
	base        string // name without a Scala binary suffix: cats-effect
	version     string
	conflict    bool // declared at different versions
	virtual     bool
	// rest are the base name's words after those that repeat the group
	// (spring-boot-autoconfigure in org.springframework.boot: autoconfigure).
	rest []string
	// prefixes are the packages that name this artifact in particular: from the
	// table, and derived from its name (see artifactPrefixes).
	prefixes []string
}

func (a *artifact) key() string { return a.group + ":" + a.name }

type maven struct {
	artifacts  map[string]*artifact   // declared, by group:artifact
	groups     map[string][]*artifact // declared, by group, sorted by name
	candidates []*artifact            // declared and virtual, sorted by key
	memo       sync.Map               // import -> lang.Target
}

// scalaSuffix is the Scala binary version sbt appends to an artifact built for
// several (cats-effect_2.13, zio_3, a Scala.js build's _sjs1_3).
var scalaSuffix = regexp.MustCompile(`(_(sjs|native)[\d.]+)?_[23](\.\d+)?$`)

// addArtifact records a declared artifact. Declared again at another version, it
// carries none: which one an import gets is not for the map to guess. A declaration
// without a version does not undo one with.
//
// Implements: REQ-JAVA-007, REQ-SCALA-005
func (m *maven) addArtifact(group, name, version string) {
	group, name, version = strings.TrimSpace(group), strings.TrimSpace(name), strings.TrimSpace(version)
	if group == "" || name == "" {
		return
	}
	a, ok := m.artifacts[group+":"+name]
	if !ok {
		m.artifacts[group+":"+name] = &artifact{group: group, name: name, base: scalaSuffix.ReplaceAllString(name, ""), version: version}
		return
	}
	switch {
	case version == "" || a.conflict || version == a.version:
	case a.version == "":
		a.version = version
	default:
		a.version, a.conflict = "", true
	}
}

// finish indexes the declared artifacts once every manifest is read: their groups,
// the prefixes their names give, and the table's entries - added to the declared
// artifact they name, or, when only other artifacts of that group are declared, to a
// virtual artifact that comes with them.
//
// Implements: REQ-JAVA-007
func (m *maven) finish(l Language) {
	m.groups = map[string][]*artifact{}
	for _, a := range m.artifacts {
		m.groups[a.group] = append(m.groups[a.group], a)
		a.rest, a.prefixes = artifactPrefixes(a.group, a.base, l)
	}
	for _, as := range m.groups {
		sort.Slice(as, func(i, j int) bool { return as[i].name < as[j].name })
	}
	virtual := map[string]*artifact{}
	for pkg, ga := range knownArtifacts {
		group, base, _ := strings.Cut(ga, ":")
		declared := m.groups[group]
		if len(declared) == 0 {
			continue
		}
		var a *artifact
		for _, d := range declared {
			if d.base == base {
				a = d
				break
			}
		}
		if a == nil {
			if a = virtual[ga]; a == nil {
				a = &artifact{group: group, name: base + groupSuffix(declared), base: base, virtual: true}
				a.version, _ = groupVersion(declared)
				a.rest, _ = artifactPrefixes(group, base, l)
				virtual[ga] = a
			}
		}
		a.prefixes = append(a.prefixes, pkg)
	}
	for _, a := range m.artifacts {
		m.candidates = append(m.candidates, a)
	}
	for _, a := range virtual {
		m.candidates = append(m.candidates, a)
	}
	sort.Slice(m.candidates, func(i, j int) bool { return m.candidates[i].key() < m.candidates[j].key() })
}

// groupVersion is the version every declared artifact of a group shares, which an
// artifact arriving with them (jackson-annotations with jackson-databind) is taken
// to have as well: libraries released as a family align their versions.
func groupVersion(as []*artifact) (string, bool) {
	v := as[0].version
	for _, a := range as[1:] {
		if a.version != v {
			return "", false
		}
	}
	return v, true
}

// groupSuffix is the Scala binary suffix every declared artifact of a group shares
// ("" when they do not): cats-core arrives beside cats-effect_3 as cats-core_3.
func groupSuffix(as []*artifact) string {
	s := as[0].name[len(as[0].base):]
	for _, a := range as[1:] {
		if a.name[len(a.base):] != s {
			return ""
		}
	}
	return s
}

// genericRoots are single words an artifact's name may start with that are no root
// package of its own (java-jwt, commons-io, core-utils).
var genericRoots = map[string]bool{
	"java": true, "javax": true, "jakarta": true, "org": true, "com": true, "net": true, "io": true,
	"core": true, "api": true, "common": true, "commons": true, "util": true, "utils": true,
	"test": true, "tests": true, "spring": true, "kotlin": true, "scala": true, "jdk": true,
	"annotations": true,
}

// artifactPrefixes derives the packages an artifact's name suggests: the group
// followed by the name's words beyond those repeating the group (org.springframework
// + spring-context: org.springframework.context; org.junit.jupiter +
// junit-jupiter-api: org.junit.jupiter.api), the same beside the group's last
// segment for a group of three or more (com.fasterxml.jackson.core +
// jackson-databind: com.fasterxml.jackson.databind), and the name's own words as a
// root package, which is how Scala libraries are imported (cats-effect: cats.effect,
// akka-actor-typed: akka.actor.typed). Every shorter run of words counts too
// (ktor-client-core: io.ktor.client). A single generic word is no root.
//
// Implements: REQ-JAVA-007, REQ-SCALA-005
func artifactPrefixes(group, base string, l Language) (rest, prefixes []string) {
	words := strings.FieldsFunc(base, func(r rune) bool { return r == '-' })
	gs := strings.Split(group, ".")
	i := 0
	for i < len(words) && namesGroup(words[i], gs) {
		i++
	}
	rest = words[i:]
	for n := 1; n <= len(rest); n++ {
		tail := strings.Join(rest[:n], ".")
		prefixes = append(prefixes, group+"."+tail)
		if len(gs) >= 3 {
			prefixes = append(prefixes, strings.Join(gs[:len(gs)-1], ".")+"."+tail)
		}
	}
	for n := 1; n <= len(words); n++ {
		p := strings.Join(words[:n], ".")
		if !strings.Contains(p, ".") && genericRoots[p] {
			break
		}
		if l.root(p) {
			continue
		}
		prefixes = append(prefixes, p)
	}
	return rest, prefixes
}

// namesGroup reports whether a word of an artifact's name repeats its group: a
// segment (jackson in com.fasterxml.jackson.core) or the start of one (spring in
// org.springframework, okhttp in com.squareup.okhttp3).
func namesGroup(w string, gs []string) bool {
	for _, s := range gs {
		if s == w || len(w) >= 3 && strings.HasPrefix(s, w) {
			return true
		}
	}
	return false
}

// root reports whether p is the language's own root package (scala, kotlin), which
// the first word of many artifacts spells (scala-xml) without owning it.
func (l Language) root(p string) bool {
	for _, pre := range l.Prefixes {
		if p+"." == pre {
			return true
		}
	}
	return false
}

// under reports whether spec is the package p or inside it.
func under(spec, p string) bool { return spec == p || strings.HasPrefix(spec, p+".") }

// score rates how well an artifact matches an import. A package prefix - one of the
// artifact's own, or its group - is worth its length in segments, the artifact's
// own winning a tie with the group; only without any prefix do the weaker rules
// count: at least three leading segments shared with the group (or all of a
// shorter one), then the group's last segment or the artifact's name as one of the
// import's first two segments (okhttp3 from com.squareup.okhttp3). A virtual
// artifact has only its table prefixes.
func (a *artifact) score(spec string, segments []string) int {
	best := 0
	for _, p := range a.prefixes {
		if under(spec, p) {
			best = max(best, 1000+10*(strings.Count(p, ".")+1)+1)
		}
	}
	if a.virtual {
		return best
	}
	gs := strings.Split(a.group, ".")
	if under(spec, a.group) {
		best = max(best, 1000+10*len(gs))
	}
	if best > 0 {
		return best
	}
	common := 0
	for common < len(gs) && common < len(segments) && gs[common] == segments[common] {
		common++
	}
	if common >= min(3, len(gs)) {
		return 500 + common
	}
	for _, s := range segments[:min(2, len(segments))] {
		if s == gs[len(gs)-1] || s == a.base {
			return 100
		}
	}
	return 0
}

// better breaks a tie between two artifacts matching an import equally well: the
// one more of whose words the import spells (io.ktor.client.engine.cio is
// ktor-client-cio's, not ktor-client-core's), then the family's main artifact - the
// one named after its group alone (spring-boot), else one ending in core, api or
// common (cats-core, not cats-effect, for cats.syntax) - then a declared artifact
// over one that arrives with it, then the shorter name, then the name.
func better(a, b *artifact, segments []string) bool {
	if ha, hb := hits(a, segments), hits(b, segments); ha != hb {
		return ha > hb
	}
	if ra, rb := mainRank(a), mainRank(b); ra != rb {
		return ra < rb
	}
	if a.virtual != b.virtual {
		return !a.virtual
	}
	if len(a.name) != len(b.name) {
		return len(a.name) < len(b.name)
	}
	return a.key() < b.key()
}

func hits(a *artifact, segments []string) int {
	n := 0
	for _, w := range a.rest {
		for _, s := range segments {
			if s == w {
				n++
				break
			}
		}
	}
	return n
}

func mainRank(a *artifact) int {
	switch {
	case len(a.rest) == 0:
		return 0
	case len(a.rest) == 1 && (a.rest[0] == "core" || a.rest[0] == "api" || a.rest[0] == "common"):
		return 1
	case a.rest[len(a.rest)-1] == "core" || a.rest[len(a.rest)-1] == "api":
		return 2
	}
	return 3
}

// artifactOf places an import that names no project source and no standard library
// on the Maven island: the declared artifact matching it best (score, then better),
// at its declared version. An import nothing declared matches is unresolved, named
// after the table's artifact when the table knows the package, else guessed from the
// package: up to three leading package segments as the group and the last of them
// as the artifact (javax.money.convert.X is javax.money.convert:convert).
//
// Implements: REQ-JAVA-007, REQ-JAVA-009, REQ-JAVA-012, REQ-SCALA-005
func (r *resolver) artifactOf(spec string, wildcard bool) lang.Target {
	key := spec
	if wildcard {
		key += ".*" // the guess for an unmatched import differs
	}
	if t, ok := r.memo.Load(key); ok {
		return t.(lang.Target)
	}
	segments := strings.Split(spec, ".")
	var best *artifact
	bestScore := 0
	for _, a := range r.candidates {
		s := a.score(spec, segments)
		if s > bestScore || s == bestScore && s > 0 && better(a, best, segments) {
			best, bestScore = a, s
		}
	}
	var t lang.Target
	if best != nil {
		t = lang.Target{Ecosystem: ecoMaven, Package: best.key(), Version: best.version, Pinned: pinnedMaven(best.version)}
	} else {
		t = lang.Target{Ecosystem: ecoMaven, Package: guessArtifact(segments, wildcard), Unresolved: true}
	}
	r.memo.Store(key, t)
	return t
}

// guessArtifact names an artifact nothing declares: the table's, else a guess from
// the import's package (its segments before the first capitalized one; for an import
// without one, all but the last unless it is a wildcard).
func guessArtifact(segments []string, wildcard bool) string {
	for n := len(segments); n >= 1; n-- {
		if ga, ok := knownArtifacts[strings.Join(segments[:n], ".")]; ok {
			return ga
		}
	}
	pkg := len(segments)
	for i, s := range segments {
		if s != "" && unicode.IsUpper([]rune(s)[0]) {
			pkg = i
			break
		}
	}
	if pkg == len(segments) && !wildcard {
		pkg--
	}
	pkg = max(1, min(3, pkg))
	return strings.Join(segments[:pkg], ".") + ":" + segments[pkg-1]
}

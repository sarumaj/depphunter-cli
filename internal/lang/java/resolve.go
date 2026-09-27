package java

import (
	"encoding/xml"
	"maps"
	"os"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/BurntSushi/toml"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// Implements: REQ-JAVA-002
var jdkPrefixes = []string{
	"java.", "jdk.", "sun.", "com.sun.", "org.w3c.dom", "org.xml.sax", "org.ietf.jgss", "org.omg.",
	// javax is split: these ship with the JDK, others (servlet, persistence, …) do not.
	"javax.swing", "javax.crypto", "javax.net", "javax.xml", "javax.sql", "javax.naming",
	"javax.management", "javax.imageio", "javax.sound", "javax.print", "javax.script",
	"javax.security", "javax.tools", "javax.lang.model", "javax.annotation.processing",
	"javax.accessibility", "javax.rmi", "javax.transaction.xa",
}

// knownGroups maps packages of popular libraries to their groupId where the two share
// too little for group() to connect them. Used only when that group is declared.
//
// Implements: REQ-JAVA-007
var knownGroups = map[string]string{
	"com.google.common":     "com.google.guava",
	"com.google.thirdparty": "com.google.guava",
	"org.junit":             "junit", // JUnit 4; JUnit 5 is org.junit.jupiter by prefix
	"junit":                 "junit",
	"lombok":                "org.projectlombok",
	"org.mockito":           "org.mockito",
	"reactor":               "io.projectreactor",
	"io.reactivex":          "io.reactivex.rxjava3",
	// Modules split out of the Scala library, published apart from it.
	"scala.xml":                 "org.scala-lang.modules",
	"scala.util.parsing":        "org.scala-lang.modules",
	"scala.collection.parallel": "org.scala-lang.modules",
	"scala.swing":               "org.scala-lang.modules",
	"scala.async":               "org.scala-lang.modules",
	"scala.scalajs":             "org.scala-js",
	"scala.scalanative":         "org.scala-native",
}

// Language is what another JVM language adds to Java's resolution. The JDK is
// everyone's and needs no entry; the zero Language is Java.
//
// Implements: REQ-KT-002, REQ-SCALA-002, REQ-SCALA-007
type Language struct {
	// Std is the ecosystem of the language's standard library, Prefixes the packages
	// it holds, and Except the ones under them published apart (Scala's scala.xml).
	Std      string
	Prefixes []string // with the trailing dot: "kotlin."
	Except   []string
	// Relative marks a language whose imports are relative to the packages around
	// the file, as Scala's are: in package a.b, "import c.D" may mean a.b.c.D or
	// a.c.D before it means the root package c.
	Relative bool
	// Implicit are the standard library's packages an import may name without its
	// prefix, because the library's root package is always imported (scala._ makes
	// "import collection.mutable" mean scala.collection.mutable) - unless a package
	// of the project or of a dependency has that name, which then takes precedence
	// (io.circe is not scala.io.circe).
	Implicit []string
}

func (l Language) covers(spec string) bool {
	for _, e := range l.Except {
		if spec == e || strings.HasPrefix(spec, e+".") {
			return false
		}
	}
	for _, p := range l.Prefixes {
		if strings.HasPrefix(spec+".", p) {
			return true
		}
	}
	return false
}

// stdTarget places an import on the language's standard library island. Its package
// is the first two segments, but a class or function of the root package itself -
// kotlin.String, scala.Option - belongs to the root package.
func (l Language) stdTarget(segments []string, wildcard bool) lang.Target {
	n := 2
	if len(segments) == 2 && !wildcard {
		n = 1
	}
	return lang.Target{Ecosystem: l.Std, Package: strings.Join(segments[:min(n, len(segments))], ".")}
}

type resolver struct {
	lang   Language
	byName map[string][]string // "User.java" -> project paths
	dirs   []string            // directories holding Java files, sorted
	groups map[string]string   // Maven groupId -> version ("" when artifacts differ)
	alias  map[string]string   // artifactId or last groupId segment -> groupId
	// words maps the first word of an artifactId to its groupId, the weakest of the
	// hints: Scala libraries name their packages after it (cats.effect from
	// org.typelevel:cats-effect, akka.actor from com.typesafe.akka:akka-actor).
	words map[string]string
	own   []string // groupIds of the project itself
	// Kotlin and Scala declarations (sources.go): fully qualified name -> files,
	// and package -> files; and the package each such file declares.
	decls, packages map[string][]string
	filePkg         map[string]string
	// roots are the first segments of the project's packages and of the declared
	// groups: the root packages known to exist.
	roots map[string]bool
}

// NewResolver is the Java resolver for another JVM language: its imports reach the
// same project sources (Java, Kotlin and Scala alike) and the same Maven groups, read
// from the same manifests; only what l describes differs.
//
// Implements: REQ-KT-004, REQ-SCALA-006
func NewResolver(all []*scan.File, l Language) lang.Resolver {
	return newResolver(all, l)
}

func newResolver(all []*scan.File, l Language) *resolver {
	r := &resolver{
		lang: l, byName: map[string][]string{}, groups: map[string]string{}, alias: map[string]string{},
		words: map[string]string{}, decls: map[string][]string{}, packages: map[string][]string{},
		filePkg: map[string]string{},
	}
	dirs := map[string]bool{}
	for _, f := range all {
		base := path.Base(f.Path)
		switch {
		case strings.HasSuffix(base, ".java"):
			r.byName[base] = append(r.byName[base], f.Path)
			dirs[path.Dir(f.Path)] = true
		case sourceExts[path.Ext(base)]:
			r.readSource(f)
		case base == "pom.xml":
			r.readPOM(f.Abs)
		case base == "build.gradle" || base == "build.gradle.kts":
			r.readGradle(f.Abs)
		case base == "libs.versions.toml":
			r.readCatalog(f.Abs)
		case strings.HasSuffix(base, ".sbt") && path.Base(path.Dir(f.Path)) != "project":
			// project/*.sbt configures sbt itself (its plugins), not the code.
			r.readSBT(f.Abs)
		}
	}
	for d := range dirs {
		r.dirs = append(r.dirs, d)
	}
	sort.Strings(r.dirs)
	for _, m := range []map[string][]string{r.decls, r.packages} {
		for _, files := range m {
			sort.Strings(files)
		}
	}
	r.roots = map[string]bool{}
	for _, names := range [][]string{slices.Collect(maps.Keys(r.packages)), slices.Collect(maps.Keys(r.groups)), r.own} {
		for _, n := range names {
			root, _, _ := strings.Cut(n, ".")
			r.roots[root] = true
		}
	}
	return r
}

// Implements: REQ-JAVA-007, REQ-SCALA-005
func (r *resolver) addGroup(group, artifact, version string) {
	if group == "" {
		return
	}
	artifact = scalaSuffix.ReplaceAllString(artifact, "")
	for _, a := range []string{artifact, group[strings.LastIndex(group, ".")+1:]} {
		if a != "" {
			r.alias[a] = group
		}
	}
	if w, _, ok := strings.Cut(artifact, "-"); ok && w != "" {
		if old, seen := r.words[w]; !seen || group < old { // the same answer in any file order
			r.words[w] = group
		}
	}
	if old, ok := r.groups[group]; ok && old != version {
		version = "" // several artifacts of one group at different versions
	}
	r.groups[group] = version
}

// scalaSuffix is the Scala version sbt appends to an artifact built for several
// (cats-effect_2.13, zio_3, a Scala.js build's _sjs1_3). The groups are what the map
// shows, so the suffix only gets in the way of reading the artifact's name.
var scalaSuffix = regexp.MustCompile(`(_(sjs|native)[\d.]+)?_[23](\.\d+)?$`)

// Implements: REQ-JAVA-001, REQ-JAVA-002, REQ-JAVA-003, REQ-JAVA-009, REQ-KT-002, REQ-KT-003
// Implements: REQ-KT-006, REQ-SCALA-002, REQ-SCALA-003, REQ-SCALA-007
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	spec := imp.Module
	wildcard := strings.HasSuffix(spec, ".*")
	spec = strings.TrimSuffix(spec, ".*")
	if r.lang.Relative {
		if abs, ok := strings.CutPrefix(spec, "_root_."); ok {
			spec = abs
		} else {
			// The packages around the file first, innermost out: they shadow the root.
			for pkg := r.filePkg[file]; pkg != ""; pkg = pkg[:max(0, strings.LastIndex(pkg, "."))] {
				if p := r.local(strings.Split(pkg+"."+spec, "."), wildcard); p != "" {
					return lang.Target{Local: p}
				}
			}
			segments := strings.Split(spec, ".")
			if slices.Contains(r.lang.Implicit, segments[0]) && !r.roots[segments[0]] && r.local(segments, wildcard) == "" {
				root := strings.TrimSuffix(r.lang.Prefixes[0], ".")
				return r.lang.stdTarget(strings.Split(root+"."+spec, "."), wildcard)
			}
		}
	}
	segments := strings.Split(spec, ".")
	if r.lang.covers(spec) {
		return r.lang.stdTarget(segments, wildcard)
	}
	for _, p := range jdkPrefixes {
		if strings.HasPrefix(spec+".", p) || strings.HasPrefix(spec, p) {
			return lang.Target{Ecosystem: ecoJDK, Package: strings.Join(segments[:min(2, len(segments))], ".")}
		}
	}
	if p := r.local(segments, wildcard); p != "" {
		return lang.Target{Local: p}
	}
	// A root no package would have: a class of the default package (a Gradle script
	// declares them, and imports them as TestMode.KSP), or in Scala a value in scope
	// whose members are imported (import builder._). Neither is a dependency.
	if (spec != "" && unicode.IsUpper([]rune(spec)[0])) || (r.lang.Relative && len(segments) == 1) {
		return lang.Target{}
	}
	for _, g := range r.own {
		if spec == g || strings.HasPrefix(spec, g+".") {
			return lang.Target{} // the project's own (e.g. generated) code we cannot see
		}
	}
	if g, ok := r.group(spec); ok {
		v := r.groups[g]
		return lang.Target{Ecosystem: ecoMaven, Package: g, Version: v, Pinned: pinnedMaven(v)}
	}
	return lang.Target{Ecosystem: ecoMaven, Package: strings.Join(segments[:max(1, min(3, len(segments)-1))], "."), Unresolved: true}
}

// local finds the project file or directory an import names: a Java class by its
// path, a Kotlin or Scala definition by what its file declares, and for a wildcard
// the directory of a Java package.
//
// Implements: REQ-JAVA-001, REQ-KT-003, REQ-SCALA-003
func (r *resolver) local(segments []string, wildcard bool) string {
	// The longest prefix naming a project class (static imports add member names).
	for n := len(segments); n >= 1; n-- {
		rel := strings.Join(segments[:n], "/") + ".java"
		for _, p := range r.byName[segments[n-1]+".java"] {
			if p == rel || strings.HasSuffix(p, "/"+rel) {
				return p
			}
		}
	}
	if p := r.declared(segments); p != "" {
		return p
	}
	if wildcard {
		rel := strings.Join(segments, "/")
		for _, d := range r.dirs {
			if d == rel || strings.HasSuffix(d, "/"+rel) {
				return d
			}
		}
	}
	return ""
}

// pinnedMaven reports whether a Maven version names one artifact. A plain version
// does, whatever its shape (Spring writes 1.2.3.RELEASE); a range, the LATEST and
// RELEASE keywords, Gradle's and Ivy's dynamic versions (1.+, latest.release), an
// unexpanded property and a snapshot - republished under the same name - do not.
//
// Implements: REQ-JAVA-008
func pinnedMaven(v string) bool {
	v = strings.TrimSpace(v)
	switch {
	case v == "", strings.Contains(v, "$"), strings.Contains(v, "+"):
		return false
	case strings.HasPrefix(strings.ToLower(v), "latest."):
		return false
	case strings.HasPrefix(v, "["), strings.HasPrefix(v, "("):
		return lang.Pinned(v) // "[1.2.3]" is one version, "[1.0,2.0)" is not
	case strings.EqualFold(v, "LATEST"), strings.EqualFold(v, "RELEASE"):
		return false
	case strings.HasSuffix(strings.ToUpper(v), "-SNAPSHOT"):
		return false
	}
	return true
}

// group finds the declared groupId an import belongs to: the longest groupId that is a
// package prefix, else the one sharing the most leading segments (at least three, or
// all of a shorter groupId) - com.fasterxml.jackson.databind comes from the group
// com.fasterxml.jackson.core - else a group whose artifactId or last segment names
// the import's first package segment (okhttp3.* from com.squareup.okhttp3), else one
// whose artifactId begins with that segment (cats.* from org.typelevel:cats-effect).
//
// Implements: REQ-JAVA-007, REQ-SCALA-005
func (r *resolver) group(spec string) (string, bool) {
	best, bestScore := "", 0
	segments := strings.Split(spec, ".")
	for g := range r.groups {
		gs := strings.Split(g, ".")
		score := 0
		if spec == g || strings.HasPrefix(spec, g+".") {
			score = 100 + len(gs)
		} else {
			common := 0
			for common < len(gs) && common < len(segments) && gs[common] == segments[common] {
				common++
			}
			if common >= min(3, len(gs)) {
				score = common
			}
		}
		if score > bestScore || (score == bestScore && score > 0 && g < best) {
			best, bestScore = g, score
		}
	}
	if bestScore > 0 {
		return best, true
	}
	for pkg, g := range knownGroups {
		if _, declared := r.groups[g]; declared && (spec == pkg || strings.HasPrefix(spec, pkg+".")) {
			return g, true
		}
	}
	for _, s := range segments[:min(2, len(segments))] { // okhttp3.*, org.junit.*
		if g, ok := r.alias[s]; ok {
			return g, true
		}
	}
	// The language's own root is the first word of many artifacts (scala-xml,
	// scala-reflect) and tells none of them apart.
	for _, p := range r.lang.Prefixes {
		if strings.HasPrefix(spec+".", p) {
			return "", false
		}
	}
	for _, s := range segments[:min(2, len(segments))] { // cats.effect.*, akka.actor.*
		if g, ok := r.words[s]; ok {
			return g, true
		}
	}
	return "", false
}

// ---------------------------------------------------------------- manifests

type pomDep struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Version    string `xml:"version"`
}

var property = regexp.MustCompile(`\$\{([^}]+)\}`)

// Implements: REQ-JAVA-003, REQ-JAVA-004
func (r *resolver) readPOM(abs string) {
	data, err := os.ReadFile(abs)
	if err != nil {
		return
	}
	var pom struct {
		GroupID string `xml:"groupId"`
		Version string `xml:"version"`
		Parent  struct {
			GroupID string `xml:"groupId"`
		} `xml:"parent"`
		Properties struct {
			Items []struct {
				XMLName xml.Name
				Value   string `xml:",chardata"`
			} `xml:",any"`
		} `xml:"properties"`
		Deps    []pomDep `xml:"dependencies>dependency"`
		Managed []pomDep `xml:"dependencyManagement>dependencies>dependency"`
	}
	if xml.Unmarshal(data, &pom) != nil {
		return
	}
	group := pom.GroupID
	if group == "" {
		group = pom.Parent.GroupID
	}
	if group != "" {
		r.own = append(r.own, group)
	}
	props := map[string]string{"project.version": pom.Version, "project.groupId": group}
	for _, p := range pom.Properties.Items {
		props[p.XMLName.Local] = strings.TrimSpace(p.Value)
	}
	expand := func(s string) string {
		return property.ReplaceAllStringFunc(s, func(m string) string {
			if v, ok := props[m[2:len(m)-1]]; ok {
				return v
			}
			return m
		})
	}
	managed := map[string]string{}
	for _, d := range pom.Managed {
		managed[expand(d.GroupID)] = expand(d.Version)
	}
	for _, d := range pom.Deps {
		g, v := expand(d.GroupID), expand(d.Version)
		if v == "" {
			v = managed[g]
		}
		if g != group {
			r.addGroup(g, expand(d.ArtifactID), v)
		}
	}
}

var (
	gradleDep   = regexp.MustCompile(`["']([\w.\-]+):([\w.\-]+)(?::([\w.\-$+]+))?["']`)
	gradleGroup = regexp.MustCompile(`(?m)^\s*group\s*=\s*["']([\w.\-]+)["']`)
)

// Implements: REQ-JAVA-005
func (r *resolver) readGradle(abs string) {
	data, err := os.ReadFile(abs)
	if err != nil {
		return
	}
	if m := gradleGroup.FindSubmatch(data); m != nil {
		r.own = append(r.own, string(m[1]))
	}
	for _, m := range gradleDep.FindAllSubmatch(data, -1) {
		r.addGroup(string(m[1]), string(m[2]), string(m[3]))
	}
}

var (
	// "org" %% "name" % "1.2.3", where the version may be a val of the build.
	sbtDep = regexp.MustCompile(`"([\w.\-]+)"\s*%{1,3}\s*"([\w.\-]+)"(?:\s*%\s*(?:"([^"]*)"|(\w+)))?`)
	sbtVal = regexp.MustCompile(`(?m)^\s*(?:lazy\s+)?val\s+(\w+)\s*=\s*"([^"]*)"`)
	sbtOrg = regexp.MustCompile(`(?m)^\s*(?:ThisBuild\s*/\s*)?organization\s*:=\s*"([\w.\-]+)"`)
)

// readSBT reads an sbt build's dependencies. `%%` (and Scala.js's `%%%`) would have
// sbt append the Scala binary version to the artifact - cats-effect_3 - but the map
// shows groups, not artifacts, so all three read as `%` and the artifact keeps the
// name it is written with; scalaVersion is not consulted.
//
// Implements: REQ-SCALA-004
func (r *resolver) readSBT(abs string) {
	data, err := os.ReadFile(abs)
	if err != nil {
		return
	}
	vals := map[string]string{}
	for _, m := range sbtVal.FindAllSubmatch(data, -1) {
		vals[string(m[1])] = string(m[2])
	}
	for _, m := range sbtOrg.FindAllSubmatch(data, -1) {
		r.own = append(r.own, string(m[1]))
	}
	for _, m := range sbtDep.FindAllSubmatch(data, -1) {
		version := string(m[3])
		if len(m[4]) > 0 {
			version = vals[string(m[4])] // "" when it is no val of this file (Test, a setting)
		}
		r.addGroup(string(m[1]), string(m[2]), version)
	}
}

// readCatalog reads a Gradle version catalog: [libraries] entries as "g:a:v" or
// { module = "g:a", version = "1" | version.ref = "name" }.
//
// Implements: REQ-JAVA-006
func (r *resolver) readCatalog(abs string) {
	var cat struct {
		Versions  map[string]any
		Libraries map[string]any
	}
	if _, err := toml.DecodeFile(abs, &cat); err != nil {
		return
	}
	for _, lib := range cat.Libraries {
		switch v := lib.(type) {
		case string:
			parts := strings.Split(v, ":")
			if len(parts) >= 2 {
				ver := ""
				if len(parts) > 2 {
					ver = parts[2]
				}
				r.addGroup(parts[0], parts[1], ver)
			}
		case map[string]any:
			group, _ := v["group"].(string)
			artifact, _ := v["name"].(string)
			if mod, ok := v["module"].(string); ok {
				group, artifact, _ = strings.Cut(mod, ":")
			}
			ver := ""
			switch vv := v["version"].(type) {
			case string:
				ver = vv
			case map[string]any:
				if ref, ok := vv["ref"].(string); ok {
					ver, _ = cat.Versions[ref].(string)
				}
			}
			r.addGroup(group, artifact, ver)
		}
	}
}

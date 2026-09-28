package java

import (
	"cmp"
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
	language    Language
	byName      map[string][]string // "User.java" -> project paths
	directories []string            // directories holding Java files, sorted
	maven                           // the declared Maven artifacts (maven.go)
	own         []string            // groupIds of the project itself
	// Kotlin and Scala declarations (sources.go): fully qualified name -> files,
	// and package -> files; and the package each such file declares.
	declarations, packages map[string][]string
	filePackage            map[string]string
	// roots are the first segments of the project's packages and of the declared
	// groups: the root packages known to exist.
	roots map[string]bool
	// locked is what the Gradle locks record, by group:artifact ("" when two of
	// them disagree).
	locked map[string]string
}

// NewResolver is the Java resolver for another JVM language: its imports reach the
// same project sources (Java, Kotlin and Scala alike) and the same Maven artifacts,
// read from the same manifests; only what l describes differs.
//
// Implements: REQ-KT-004, REQ-SCALA-006
func NewResolver(all []*scan.File, l Language) lang.Resolver {
	return newResolver(all, l)
}

func newResolver(all []*scan.File, l Language) *resolver {
	r := &resolver{
		language: l, byName: map[string][]string{}, maven: maven{artifacts: map[string]*artifact{}},
		declarations: map[string][]string{}, packages: map[string][]string{}, filePackage: map[string]string{},
	}
	directories := map[string]bool{}
	var sbt []*scan.File
	for _, f := range all {
		base := path.Base(f.Path)
		switch {
		case strings.HasSuffix(base, ".java"):
			r.byName[base] = append(r.byName[base], f.Path)
			directories[path.Dir(f.Path)] = true
		case sourceExtensions[path.Ext(base)]:
			r.readSource(f)
		case base == "pom.xml":
			r.readPOM(f.AbsolutePath)
		case base == "build.gradle" || base == "build.gradle.kts":
			r.readGradle(f.AbsolutePath)
		case base == "libs.versions.toml":
			r.readCatalog(f.AbsolutePath)
		case gradleLockfile(f.Path):
			r.readGradleLock(f.AbsolutePath)
		case strings.HasSuffix(base, ".sbt") && path.Base(path.Dir(f.Path)) != "project":
			// project/*.sbt configures sbt itself (its plugins), not the code.
			sbt = append(sbt, f)
		}
	}
	r.readSBTs(sbt)
	r.applyLocks()
	r.finish(l)
	for d := range directories {
		r.directories = append(r.directories, d)
	}
	sort.Strings(r.directories)
	for _, m := range []map[string][]string{r.declarations, r.packages} {
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

// Implements: REQ-JAVA-001, REQ-JAVA-002, REQ-JAVA-003, REQ-JAVA-009, REQ-KT-002, REQ-KT-003
// Implements: REQ-KT-006, REQ-SCALA-002, REQ-SCALA-003, REQ-SCALA-007
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	spec := rawImport.Module
	wildcard := strings.HasSuffix(spec, ".*")
	spec = strings.TrimSuffix(spec, ".*")
	if r.language.Relative {
		if absolute, ok := strings.CutPrefix(spec, "_root_."); ok {
			spec = absolute
		} else {
			// The packages around the file first, innermost out: they shadow the root.
			for packageName := r.filePackage[file]; packageName != ""; packageName = packageName[:max(0, strings.LastIndex(packageName, "."))] {
				if p := r.local(strings.Split(packageName+"."+spec, "."), wildcard); p != "" {
					return lang.Target{Local: p}
				}
			}
			segments := strings.Split(spec, ".")
			if slices.Contains(r.language.Implicit, segments[0]) && !r.roots[segments[0]] && r.local(segments, wildcard) == "" {
				root := strings.TrimSuffix(r.language.Prefixes[0], ".")
				return r.language.stdTarget(strings.Split(root+"."+spec, "."), wildcard)
			}
		}
	}
	segments := strings.Split(spec, ".")
	if r.language.covers(spec) {
		return r.language.stdTarget(segments, wildcard)
	}
	if t, ok := JDK(spec); ok {
		return t
	}
	if p := r.local(segments, wildcard); p != "" {
		return lang.Target{Local: p}
	}
	// A root no package would have: a class of the default package (a Gradle script
	// declares them, and imports them as TestMode.KSP), or in Scala a value in scope
	// whose members are imported (import builder._). Neither is a dependency.
	if (spec != "" && unicode.IsUpper([]rune(spec)[0])) || (r.language.Relative && len(segments) == 1) {
		return lang.Target{}
	}
	for _, g := range r.own {
		if spec == g || strings.HasPrefix(spec, g+".") {
			return lang.Target{} // the project's own (e.g. generated) code we cannot see
		}
	}
	return r.artifactOf(spec, wildcard)
}

// local finds the project file or directory an import names: a Java class by its
// path, a Kotlin or Scala definition by what its file declares, and for a wildcard
// the directory of a Java package.
//
// Implements: REQ-JAVA-001, REQ-KT-003, REQ-SCALA-003
func (r *resolver) local(segments []string, wildcard bool) string {
	// The longest prefix naming a project class (static imports add member names).
	for n := len(segments); n >= 1; n-- {
		relative := strings.Join(segments[:n], "/") + ".java"
		for _, p := range r.byName[segments[n-1]+".java"] {
			if p == relative || strings.HasSuffix(p, "/"+relative) {
				return p
			}
		}
	}
	if p := r.declared(segments); p != "" {
		return p
	}
	if wildcard {
		relative := strings.Join(segments, "/")
		for _, d := range r.directories {
			if d == relative || strings.HasSuffix(d, "/"+relative) {
				return d
			}
		}
	}
	return ""
}

// pinnedMaven is Maven's pin rule (lang.PinnedMaven), shared with the Clojure plugin
// and the index client, which name Maven artifacts too.
//
// Implements: REQ-JAVA-008
func pinnedMaven(v string) bool { return lang.PinnedMaven(v) }

// JDK places a Java package or class name on the JDK island, if the JDK ships it:
// java.util.Date is the package java.util there. Other JVM languages' imports of Java
// classes (Clojure's :import) resolve through it.
//
// Implements: REQ-JAVA-002, REQ-CLOJURE-006
func JDK(spec string) (lang.Target, bool) {
	for _, p := range jdkPrefixes {
		if strings.HasPrefix(spec+".", p) || strings.HasPrefix(spec, p) {
			segments := strings.Split(spec, ".")
			return lang.Target{Ecosystem: ecosystemJDK, Package: strings.Join(segments[:min(2, len(segments))], ".")}, true
		}
	}
	return lang.Target{}, false
}

// ---------------------------------------------------------------- manifests

type pomDependency struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Version    string `xml:"version"`
}

var property = regexp.MustCompile(`\$\{([^}]+)\}`)

// Implements: REQ-JAVA-003, REQ-JAVA-004
func (r *resolver) readPOM(absolute string) {
	data, err := os.ReadFile(absolute)
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
		Dependencies []pomDependency `xml:"dependencies>dependency"`
		Managed      []pomDependency `xml:"dependencyManagement>dependencies>dependency"`
	}
	if lang.UnmarshalXML(data, &pom) != nil {
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
		return strings.TrimSpace(property.ReplaceAllStringFunc(s, func(m string) string {
			if v, ok := props[m[2:len(m)-1]]; ok {
				return v
			}
			return m
		}))
	}
	managed := map[string]string{}
	for _, d := range pom.Managed {
		managed[expand(d.GroupID)+":"+expand(d.ArtifactID)] = expand(d.Version)
	}
	for _, d := range pom.Dependencies {
		g, a, v := expand(d.GroupID), expand(d.ArtifactID), expand(d.Version)
		if v == "" {
			v = managed[g+":"+a]
		}
		if g != group {
			r.addArtifact(g, a, v)
		}
	}
}

var (
	gradleDependency = regexp.MustCompile(`["']([\w.\-]+):([\w.\-]+)(?::([\w.\-$+{}\[\](),]+))?["']`)
	// The map notation: group: 'g', name: 'a', version: 'v' (Groovy) or
	// group = "g", name = "a", version = "v" (Kotlin DSL).
	gradleMap = regexp.MustCompile(`group\s*[:=]\s*["']([\w.\-]+)["']\s*,\s*name\s*[:=]\s*["']([\w.\-]+)["']` +
		`(?:\s*,\s*version\s*[:=]\s*["']([^"']*)["'])?`)
	gradleGroup = regexp.MustCompile(`(?m)^\s*group\s*=\s*["']([\w.\-]+)["']`)
)

// Implements: REQ-JAVA-005
func (r *resolver) readGradle(absolute string) {
	data, err := os.ReadFile(absolute)
	if err != nil {
		return
	}
	if m := gradleGroup.FindSubmatch(data); m != nil {
		r.own = append(r.own, string(m[1]))
	}
	for _, m := range gradleDependency.FindAllSubmatch(data, -1) {
		r.addArtifact(string(m[1]), string(m[2]), string(m[3]))
	}
	for _, m := range gradleMap.FindAllSubmatch(data, -1) {
		r.addArtifact(string(m[1]), string(m[2]), string(m[3]))
	}
}

var (
	// "org" %% "name" % "1.2.3", where the version may be a val of the build.
	sbtDependency = regexp.MustCompile(`"([\w.\-]+)"\s*(%{1,3})\s*"([\w.\-]+)"(?:\s*%\s*(?:"([^"]*)"|(\w+)))?`)
	sbtValue      = regexp.MustCompile(`(?m)^\s*(?:lazy\s+)?val\s+(\w+)\s*=\s*"([^"]*)"`)
	sbtOrg        = regexp.MustCompile(`(?m)^\s*(?:ThisBuild\s*/\s*)?organization\s*:=\s*"([\w.\-]+)"`)
	sbtScala      = regexp.MustCompile(`(?m)^\s*(?:ThisBuild\s*/\s*)?scalaVersion(?:\s+in\s+ThisBuild)?\s*:=\s*(?:"([^"]+)"|(\w+))`)
)

// readSBTs reads the dependencies of sbt builds. `%%` (and Scala.js's `%%%`) has
// sbt append the Scala binary version to the artifact - cats-effect_3 - which is
// the name Maven Central, OSV and the POM know it by, so it is appended here from
// the build's scalaVersion: the file's own, else the one of the build file nearest
// the root. Without a scalaVersion the artifact keeps the name it is written with.
// `%%%` is read as `%%`: the platform suffix of a Scala.js build (_sjs1_3) depends on
// the project, which the build is not evaluated to learn.
//
// Implements: REQ-SCALA-004
func (r *resolver) readSBTs(files []*scan.File) {
	sort.Slice(files, func(i, j int) bool {
		depthI, depthJ := strings.Count(files[i].Path, "/"), strings.Count(files[j].Path, "/")
		return depthI < depthJ || depthI == depthJ && files[i].Path < files[j].Path
	})
	type build struct {
		data   []byte
		values map[string]string
		scala  string
	}
	var builds []build
	global := ""
	for _, f := range files {
		data, err := os.ReadFile(f.AbsolutePath)
		if err != nil {
			continue
		}
		b := build{data: data, values: map[string]string{}}
		for _, m := range sbtValue.FindAllSubmatch(data, -1) {
			b.values[string(m[1])] = string(m[2])
		}
		if m := sbtScala.FindSubmatch(data); m != nil {
			b.scala = string(m[1])
			if len(m[2]) > 0 {
				b.scala = b.values[string(m[2])]
			}
		}
		if global == "" {
			global = b.scala
		}
		builds = append(builds, b)
	}
	for _, b := range builds {
		for _, m := range sbtOrg.FindAllSubmatch(b.data, -1) {
			r.own = append(r.own, string(m[1]))
		}
		suffix := scalaBinary(cmp.Or(b.scala, global))
		for _, m := range sbtDependency.FindAllSubmatch(b.data, -1) {
			version := string(m[4])
			if len(m[5]) > 0 {
				version = b.values[string(m[5])] // "" when it is no val of this file (Test, a setting)
			}
			name := string(m[3])
			if len(m[2]) > 1 && suffix != "" && !scalaSuffix.MatchString(name) {
				name += "_" + suffix
			}
			r.addArtifact(string(m[1]), name, version)
		}
	}
}

// scalaBinary is the binary version sbt's %% appends for a Scala version: 3 for any
// Scala 3, 2.13 for 2.13.x; "" when it cannot tell.
func scalaBinary(v string) string {
	switch parts := strings.Split(v, "."); {
	case len(parts) >= 2 && parts[0] == "3":
		return "3"
	case len(parts) >= 3 && parts[0] == "2":
		return parts[0] + "." + parts[1]
	}
	return ""
}

// gradleLockfile reports whether a file is a Gradle dependency lock: a project's
// gradle.lockfile, or one configuration's <name>.lockfile under
// gradle/dependency-locks (the format before Gradle 7). The locks of the build
// script's own classpath (buildscript-gradle.lockfile, settings-gradle.lockfile,
// buildscript-*.lockfile) hold Gradle plugins, not what the code imports.
func gradleLockfile(p string) bool {
	base := path.Base(p)
	if base == "gradle.lockfile" {
		return true
	}
	directory := path.Dir(p)
	return strings.HasSuffix(base, ".lockfile") && !strings.HasPrefix(base, "buildscript-") &&
		path.Base(directory) == "dependency-locks" && path.Base(path.Dir(directory)) == "gradle"
}

// readGradleLock reads a Gradle dependency lock: `group:artifact:version=configurations`
// lines (`group:artifact:version` alone in a per-configuration lock), `#` comments and
// the `empty=` line aside. Every locked module is recorded by group:artifact; locks
// that disagree on a module's version lock none.
//
// Implements: REQ-JAVA-013
func (r *resolver) readGradleLock(absolute string) {
	data, err := os.ReadFile(absolute)
	if err != nil {
		return
	}
	if r.locked == nil {
		r.locked = map[string]string{}
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		coordinates, _, _ := strings.Cut(line, "=")
		parts := strings.Split(strings.TrimSpace(coordinates), ":")
		if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
			continue // the empty= line, or not a module
		}
		key := parts[0] + ":" + parts[1]
		if have, ok := r.locked[key]; ok && have != parts[2] {
			r.locked[key] = "" // locked at two versions: neither is the one
			continue
		}
		r.locked[key] = parts[2]
	}
}

// applyLocks puts what a Gradle lock records on the artifacts the build declares: the
// locked version replaces the declared one (a dynamic 2.3.+, a range, or none at
// all), which is kept as requested. A module only the lock names - a transitive
// dependency - is not declared by it: imports resolve to what the build asks for.
//
// Implements: REQ-JAVA-013
func (r *resolver) applyLocks() {
	for key, v := range r.locked {
		a := r.artifacts[key]
		if a == nil || v == "" {
			continue
		}
		if a.version != v {
			a.requested = a.version
		}
		a.version, a.conflict = v, false
	}
}

// readCatalog reads a Gradle version catalog: [libraries] entries as "g:a:v" or
// { module = "g:a" | group = "g", name = "a", version = "1" | version.ref = "name" |
// version = { strictly | require | prefer = "1" } }.
//
// Implements: REQ-JAVA-006
func (r *resolver) readCatalog(absolute string) {
	var cat struct {
		Versions  map[string]any
		Libraries map[string]any
	}
	if _, err := toml.DecodeFile(absolute, &cat); err != nil {
		return
	}
	rich := func(v any) string {
		switch v := v.(type) {
		case string:
			return v
		case map[string]any:
			for _, k := range []string{"strictly", "require", "prefer"} {
				if s, ok := v[k].(string); ok {
					return s
				}
			}
		}
		return ""
	}
	for _, library := range cat.Libraries {
		switch v := library.(type) {
		case string:
			parts := strings.Split(v, ":")
			if len(parts) >= 2 {
				version := ""
				if len(parts) > 2 {
					version = parts[2]
				}
				r.addArtifact(parts[0], parts[1], version)
			}
		case map[string]any:
			group, _ := v["group"].(string)
			artifact, _ := v["name"].(string)
			if module, ok := v["module"].(string); ok {
				group, artifact, _ = strings.Cut(module, ":")
			}
			version := rich(v["version"])
			if vv, ok := v["version"].(map[string]any); ok {
				if reference, ok := vv["ref"].(string); ok {
					version = rich(cat.Versions[reference])
				}
			}
			r.addArtifact(group, artifact, version)
		}
	}
}

package java

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// testdata/repo: a Maven module (properties, dependencyManagement) and a Gradle module
// with a version catalog.
func analyze(t *testing.T) map[string]*lang.FileResult {
	t.Helper()
	return langtest.Analyze(t, Plugin{}, "testdata/repo")
}

// Verifies: REQ-JAVA-001, REQ-JAVA-002, REQ-JAVA-003, REQ-JAVA-004, REQ-JAVA-005, REQ-JAVA-006, REQ-JAVA-007, REQ-JAVA-008, REQ-JAVA-009
func TestResolution(t *testing.T) {
	// cSpell: disable
	res := analyze(t)["src/main/java/com/example/app/App.java"]
	langtest.CheckImports(t, res, map[string]lang.Target{
		"import java.util.List":                                 {Ecosystem: "jdk", Package: "java.util"},
		"import javax.swing.JFrame":                             {Ecosystem: "jdk", Package: "javax.swing"},
		"import javax.servlet.http.HttpServlet":                 {Ecosystem: "maven", Package: "javax.servlet:javax.servlet-api", Unresolved: true},
		"import com.example.app.model.User":                     {Local: "src/main/java/com/example/app/model/User.java"},
		"import com.example.app.model.*":                        {Local: "src/main/java/com/example/app/model"},
		"import static com.example.app.util.Strings.trim":       {Local: "src/main/java/com/example/app/util/Strings.java"},
		"import com.example.generated.Gen":                      {},
		"import org.springframework.context.ApplicationContext": {Ecosystem: "maven", Package: "org.springframework:spring-context", Version: "6.1.0", Pinned: true},
		"import com.fasterxml.jackson.databind.ObjectMapper":    {Ecosystem: "maven", Package: "com.fasterxml.jackson.core:jackson-databind", Version: "2.17.0", Pinned: true},
		// A version range is not one artifact.
		"import org.slf4j.Logger": {Ecosystem: "maven", Package: "org.slf4j:slf4j-api", Version: "[1.7,2.0)"},
		// junit-jupiter is declared; the class is in junit-jupiter-api, which comes
		// with it at the version of its group.
		"import static org.junit.jupiter.api.Assertions.assertEquals": {Ecosystem: "maven", Package: "org.junit.jupiter:junit-jupiter-api", Version: "5.10.2", Pinned: true},
		"import okhttp3.OkHttpClient":                                 {Ecosystem: "maven", Package: "com.squareup.okhttp3:okhttp", Version: "4.12.0", Pinned: true},
		// Packages sharing too little with their artifact come from knownArtifacts.
		"import com.google.common.collect.Lists": {Ecosystem: "maven", Package: "com.google.guava:guava", Version: "33.0.0-jre", Pinned: true},
		"import org.junit.Test":                  {Ecosystem: "maven", Package: "junit:junit", Version: "4.13.2", Pinned: true},
		"import org.acme.net.Client":             {Local: "lib/src/main/java/org/acme/net/Client.java"},
	})
	// cSpell: enable
}

func TestSymbols(t *testing.T) {
	langtest.CheckSymbols(t, analyze(t)["src/main/java/com/example/app/App.java"], map[string]string{
		"App": "class", "App.main": "method", "App.helper": "method", "Service": "interface",
		"Mode": "enum", "Mode.weight": "method", "Point": "record",
	})
}

// Verifies: REQ-JAVA-008
func TestPinnedMaven(t *testing.T) {
	for v, want := range map[string]bool{
		"6.1.0": true, "33.0.0-jre": true, "1.2.3.RELEASE": true, "[1.2.3]": true,
		"": false, "[1.7,2.0)": false, "LATEST": false, "RELEASE": false, "1.0-SNAPSHOT": false,
		"${undefined}": false, "$ktorVersion": false,
		// Gradle's and Ivy's (sbt's) dynamic versions.
		"1.+": false, "2.3.+": false, "+": false, "latest.release": false, "latest.integration": false,
	} {
		if got := pinnedMaven(v); got != want {
			t.Errorf("pinnedMaven(%q) = %v, want %v", v, got, want)
		}
	}
}

// Verifies: REQ-KT-003, REQ-KT-005, REQ-SCALA-003
func TestDeclarations(t *testing.T) {
	src := `/*
 * package not.this
 */
@file:JvmName("Tools")

package com.example
package tools

import a.b.C

// class NotThis
private[tools] final case class Box(v: Int)
sealed trait Shape
object Registry:
  def lookup = 1
  class Nested
enum Color:
  case Red
fun <T> List<T>.second(): T = this[1]
fun String.shout() = uppercase()
internal fun interface Action { fun run() }
const val MAX = 1
typealias Name = String
val (a, b) = pair
package object util
`
	pkg, names := declarations([]byte(src))
	if pkg != "com.example.tools" {
		t.Errorf("package %q, want com.example.tools", pkg)
	}
	want := []string{"Box", "Shape", "Registry", "Color", "second", "shout", "Action", "MAX", "Name", "util"}
	if strings.Join(names, " ") != strings.Join(want, " ") {
		t.Errorf("names %v, want %v", names, want)
	}
}

// resolveIn builds a resolver over a project written to a temporary directory and
// resolves each import of a Java file against it.
func resolveIn(t *testing.T, l Language, files map[string]string, imports ...string) map[string]lang.Target {
	t.Helper()
	root := t.TempDir()
	for p, c := range files {
		abs := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r := newResolver(langtest.Files(t, root), l)
	out := map[string]lang.Target{}
	for _, imp := range imports {
		out[imp] = r.Resolve("src/Main.java", lang.RawImport{Module: imp})
	}
	return out
}

func checkTargets(t *testing.T, got, want map[string]lang.Target) {
	t.Helper()
	for imp, w := range want {
		if g := got[imp]; g != w {
			t.Errorf("%s: got %+v, want %+v", imp, g, w)
		}
	}
}

// Every notation names the artifact; ties within a group go to the artifact the
// import spells, else the family's main one; an artifact the table knows arrives
// with its declared siblings at their version.
//
// Verifies: REQ-JAVA-005, REQ-JAVA-006, REQ-JAVA-007
func TestArtifactMatching(t *testing.T) {
	// cSpell: disable
	maven := func(pkg, v string) lang.Target {
		return lang.Target{Ecosystem: "maven", Package: pkg, Version: v, Pinned: pinnedMaven(v)}
	}
	got := resolveIn(t, Language{}, map[string]string{
		"build.gradle": `dependencies {
    implementation group: 'io.ktor', name: 'ktor-client-core', version: '2.3.12'
    implementation(group = "io.ktor", name = "ktor-client-cio", version = "2.3.12")
    implementation "com.fasterxml.jackson.core:jackson-databind:2.17.1"
    implementation "org.springframework.boot:spring-boot-starter-web"
    implementation "org.springframework.boot:spring-boot-starter-data-jpa"
    implementation "commons-io:commons-io:2.16.1"
    implementation "org.apache.commons:commons-lang3:3.14.0"
    implementation "org.projectlombok:lombok:${lombokVersion}"
    implementation "com.acme:dup:1.0"
}`,
		"sub/build.gradle.kts": `dependencies { implementation("com.acme:dup:2.0") }`,
		"gradle/libs.versions.toml": `[versions]
netty = { strictly = "4.1.110.Final" }
[libraries]
netty-handler = { group = "io.netty", name = "netty-handler", version.ref = "netty" }
netty-buffer = { module = "io.netty:netty-buffer", version = { require = "4.1.110.Final" } }
`,
	},
		"io.ktor.client.HttpClient", "io.ktor.client.engine.cio.CIO",
		"com.fasterxml.jackson.databind.ObjectMapper", "com.fasterxml.jackson.annotation.JsonProperty",
		"org.springframework.boot.SpringApplication", "org.springframework.boot.autoconfigure.SpringBootApplication",
		"org.apache.commons.io.FileUtils", "org.apache.commons.lang3.StringUtils", "lombok.Data",
		"io.netty.handler.ssl.SslContext", "io.netty.buffer.ByteBuf", "io.netty.channel.Channel",
		"com.acme.dup.Thing",
		"org.springframework.web.bind.annotation.GetMapping", "org.mockito.Mockito.when",
	)
	checkTargets(t, got, map[string]lang.Target{
		"io.ktor.client.HttpClient":     maven("io.ktor:ktor-client-core", "2.3.12"),
		"io.ktor.client.engine.cio.CIO": maven("io.ktor:ktor-client-cio", "2.3.12"),
		// jackson-annotations arrives with jackson-databind, at its version.
		"com.fasterxml.jackson.databind.ObjectMapper":                  maven("com.fasterxml.jackson.core:jackson-databind", "2.17.1"),
		"com.fasterxml.jackson.annotation.JsonProperty":                maven("com.fasterxml.jackson.core:jackson-annotations", "2.17.1"),
		"org.springframework.boot.SpringApplication":                   maven("org.springframework.boot:spring-boot", ""),
		"org.springframework.boot.autoconfigure.SpringBootApplication": maven("org.springframework.boot:spring-boot-autoconfigure", ""),
		"org.apache.commons.io.FileUtils":                              maven("commons-io:commons-io", "2.16.1"),
		"org.apache.commons.lang3.StringUtils":                         maven("org.apache.commons:commons-lang3", "3.14.0"),
		"lombok.Data":                                                  maven("org.projectlombok:lombok", "${lombokVersion}"),
		"io.netty.handler.ssl.SslContext":                              maven("io.netty:netty-handler", "4.1.110.Final"),
		"io.netty.buffer.ByteBuf":                                      maven("io.netty:netty-buffer", "4.1.110.Final"),
		// netty-transport is in the table, and arrives with its declared siblings.
		"io.netty.channel.Channel": maven("io.netty:netty-transport", "4.1.110.Final"),
		// Declared at two versions: no version.
		"com.acme.dup.Thing": maven("com.acme:dup", ""),
		// Nothing of the group declared: the table's name, unresolved; else a guess
		// from the package, a static import's class not included.
		"org.springframework.web.bind.annotation.GetMapping": {Ecosystem: "maven", Package: "org.springframework:spring-web", Unresolved: true},
		"org.mockito.Mockito.when":                           {Ecosystem: "maven", Package: "org.mockito:mockito-core", Unresolved: true},
	})
	unknown := resolveIn(t, Language{}, nil, "javax.money.convert.CurrencyConversion", "net.sf.saxon.s9api.Processor", "foo.bar.*")
	checkTargets(t, unknown, map[string]lang.Target{
		"javax.money.convert.CurrencyConversion": {Ecosystem: "maven", Package: "javax.money.convert:convert", Unresolved: true},
		"net.sf.saxon.s9api.Processor":           {Ecosystem: "maven", Package: "net.sf.saxon:saxon", Unresolved: true},
		"foo.bar.*":                              {Ecosystem: "maven", Package: "foo.bar:bar", Unresolved: true},
	})
	// cSpell: enable
}

// A Gradle lock puts its version on what the build declares - a dynamic version, a
// range or none - keeping the declared one as requested; locks that disagree lock
// nothing, a module only a lock names is not declared by it, and the build script's
// own classpath locks are not read.
//
// Verifies: REQ-JAVA-013
func TestGradleLockfile(t *testing.T) {
	// cSpell: disable
	got := resolveIn(t, Language{}, map[string]string{
		"build.gradle.kts": `dependencies {
    implementation("io.ktor:ktor-client-core:2.3.+")
    implementation("com.squareup.okhttp3:okhttp")
    implementation("com.fasterxml.jackson.core:jackson-databind:[2.15,3.0)")
    implementation("commons-io:commons-io:2.16.1")
    implementation("com.acme:split:1.+")
    implementation("com.acme:tool")
}`,
		"gradle.lockfile": `# This is a Gradle generated file for dependency locking.
# Manual edits can break the build and are not advised.
# This file is expected to be part of source control.
com.fasterxml.jackson.core:jackson-databind:2.17.2=compileClasspath,runtimeClasspath
com.squareup.okhttp3:okhttp:4.12.0=compileClasspath,runtimeClasspath
com.squareup.okio:okio:3.6.0=runtimeClasspath
commons-io:commons-io:2.16.1=compileClasspath
io.ktor:ktor-client-core:2.3.12=compileClasspath,runtimeClasspath
com.acme:split:1.2=compileClasspath
empty=annotationProcessor
`,
		"lib/gradle/dependency-locks/compileClasspath.lockfile":  "# old format\ncom.acme:split:1.3\n",
		"gradle/dependency-locks/buildscript-classpath.lockfile": "com.acme:tool:9.9\n",
		"buildscript-gradle.lockfile":                            "com.acme:tool:9.9=classpath\n",
		"settings-gradle.lockfile":                               "com.acme:tool:9.9=classpath\n",
	},
		"io.ktor.client.HttpClient", "okhttp3.OkHttpClient", "com.fasterxml.jackson.databind.ObjectMapper",
		"org.apache.commons.io.FileUtils", "com.acme.split.Thing", "okio.Buffer", "com.acme.tool.Tool",
	)
	checkTargets(t, got, map[string]lang.Target{
		"io.ktor.client.HttpClient":                   {Ecosystem: "maven", Package: "io.ktor:ktor-client-core", Version: "2.3.12", Requested: "2.3.+", Pinned: true},
		"okhttp3.OkHttpClient":                        {Ecosystem: "maven", Package: "com.squareup.okhttp3:okhttp", Version: "4.12.0", Pinned: true},
		"com.fasterxml.jackson.databind.ObjectMapper": {Ecosystem: "maven", Package: "com.fasterxml.jackson.core:jackson-databind", Version: "2.17.2", Requested: "[2.15,3.0)", Pinned: true},
		"org.apache.commons.io.FileUtils":             {Ecosystem: "maven", Package: "commons-io:commons-io", Version: "2.16.1", Pinned: true},
		// Two locks disagree: the declared dynamic version stays.
		"com.acme.split.Thing": {Ecosystem: "maven", Package: "com.acme:split", Version: "1.+"},
		// Only the build script's classpath locks name it.
		"com.acme.tool.Tool": {Ecosystem: "maven", Package: "com.acme:tool"},
		// okio is locked, not declared.
		"okio.Buffer": {Ecosystem: "maven", Package: "com.squareup.okio:okio", Unresolved: true},
	})
	// cSpell: enable
	for p, want := range map[string]bool{
		"gradle.lockfile": true, "app/gradle.lockfile": true,
		"gradle/dependency-locks/runtimeClasspath.lockfile":      true,
		"gradle/dependency-locks/buildscript-classpath.lockfile": false,
		"buildscript-gradle.lockfile":                            false,
		"settings-gradle.lockfile":                               false,
		"dependency-locks/compileClasspath.lockfile":             false,
	} {
		if gradleLockfile(p) != want {
			t.Errorf("%s: lock %v, want %v", p, !want, want)
		}
	}
}

// A root package goes to the artifact whose group names it, not to an extension
// named after it; a package the table gives to an undeclared artifact is not taken
// by a declared one with a shorter prefix.
//
// Verifies: REQ-JAVA-007, REQ-JAVA-012
func TestArtifactOwnership(t *testing.T) {
	// cSpell: disable
	got := resolveIn(t, Language{}, map[string]string{
		"build.gradle": `dependencies {
    implementation "com.github.blagerweij:liquibase-sessionlock:1.6.9"
    implementation "org.liquibase:liquibase-core:4.33.0"
    implementation "com.google.guava:guava:33.0.0-jre"
    implementation "commons-validator:commons-validator:1.10.0"
    implementation "org.apache.commons:commons-lang3:3.14.0"
}`,
	}, "liquibase.Liquibase", "com.google.common.jimfs.Jimfs", "com.google.common.collect.ImmutableList",
		"org.apache.commons.validator.routines.UrlValidator")
	checkTargets(t, got, map[string]lang.Target{
		"liquibase.Liquibase":                     {Ecosystem: "maven", Package: "org.liquibase:liquibase-core", Version: "4.33.0", Pinned: true},
		"com.google.common.jimfs.Jimfs":           {Ecosystem: "maven", Package: "com.google.jimfs:jimfs", Unresolved: true},
		"com.google.common.collect.ImmutableList": {Ecosystem: "maven", Package: "com.google.guava:guava", Version: "33.0.0-jre", Pinned: true},
		"org.apache.commons.validator.routines.UrlValidator": {Ecosystem: "maven", Package: "commons-validator:commons-validator",
			Version: "1.10.0", Pinned: true},
	})
	// cSpell: enable
}

// Artifacts is the matcher for another language's class imports: prefixes only,
// the language's own root packages claimed by no artifact.
//
// Verifies: REQ-JAVA-012, REQ-CLOJURE-006
func TestArtifactsForOtherLanguages(t *testing.T) {
	a := NewArtifacts(Language{Prefixes: []string{"clojure."}})
	a.Declare("org.clojure", "clojure", "1.12.0")
	a.Declare("com.fasterxml.jackson.core", "jackson-databind", "2.17.0")
	a.Declare("com.fasterxml.jackson.core", "jackson-core", "2.17.0")
	a.Declare("io.acme.platform.core", "widgets", "1.0")
	a.Finish()
	for _, c := range []struct {
		class string
		want  Match
		ok    bool
	}{
		{"com.fasterxml.jackson.databind.ObjectMapper", Match{Name: "com.fasterxml.jackson.core:jackson-databind", Version: "2.17.0"}, true},
		{"com.fasterxml.jackson.annotation.JsonProperty", Match{Name: "com.fasterxml.jackson.core:jackson-annotations", Version: "2.17.0", Virtual: true}, true},
		{"okhttp3.OkHttpClient", Match{}, false},            // Java's weak rule (the group's last segment) is not applied
		{"clojure.core.async.impl.Channel", Match{}, false}, // org.clojure/clojure owns no clojure.* class here
		{"org.quartz.JobKey", Match{}, false},
	} {
		got, ok := a.Match(c.class)
		if got != c.want || ok != c.ok {
			t.Errorf("Match(%s) = %+v, %v; want %+v, %v", c.class, got, ok, c.want, c.ok)
		}
	}
}

// sbt's %% appends the Scala binary version of the build's scalaVersion (the
// file's own, else the root build's; a val works too); % does not, and without a
// scalaVersion the name stays as written.
//
// Verifies: REQ-SCALA-004
func TestSBTBinarySuffix(t *testing.T) {
	scala := Language{Std: "scala-std", Prefixes: []string{"scala."}, Relative: true}
	got := resolveIn(t, scala, map[string]string{
		"build.sbt": `val scala213 = "2.13.14"
ThisBuild / scalaVersion := scala213
libraryDependencies += "org.typelevel" %% "cats-core" % "2.12.0"
libraryDependencies += "com.google.guava" % "guava" % "33.2.1-jre"
libraryDependencies += "dev.zio" %%% "zio" % "2.1.1"
`,
		"mod/build.sbt": `scalaVersion := "3.4.2"
libraryDependencies += "io.circe" %% "circe-core" % "0.14.9"
`,
	}, "cats.Monad", "com.google.common.base.Strings", "zio.ZIO", "io.circe.Json")
	checkTargets(t, got, map[string]lang.Target{
		"cats.Monad":                     {Ecosystem: "maven", Package: "org.typelevel:cats-core_2.13", Version: "2.12.0", Pinned: true},
		"com.google.common.base.Strings": {Ecosystem: "maven", Package: "com.google.guava:guava", Version: "33.2.1-jre", Pinned: true},
		"zio.ZIO":                        {Ecosystem: "maven", Package: "dev.zio:zio_2.13", Version: "2.1.1", Pinned: true},
		"io.circe.Json":                  {Ecosystem: "maven", Package: "io.circe:circe-core_3", Version: "0.14.9", Pinned: true},
	})
	bare := resolveIn(t, scala, map[string]string{
		"build.sbt": `libraryDependencies += "org.typelevel" %% "cats-effect" % "3.5.4"` + "\n",
	}, "cats.effect.IO")
	checkTargets(t, bare, map[string]lang.Target{
		"cats.effect.IO": {Ecosystem: "maven", Package: "org.typelevel:cats-effect", Version: "3.5.4", Pinned: true},
	})
}

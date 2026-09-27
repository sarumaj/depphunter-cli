package java

import (
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
		"import javax.servlet.http.HttpServlet":                 {Ecosystem: "maven", Package: "javax.servlet.http", Unresolved: true},
		"import com.example.app.model.User":                     {Local: "src/main/java/com/example/app/model/User.java"},
		"import com.example.app.model.*":                        {Local: "src/main/java/com/example/app/model"},
		"import static com.example.app.util.Strings.trim":       {Local: "src/main/java/com/example/app/util/Strings.java"},
		"import com.example.generated.Gen":                      {},
		"import org.springframework.context.ApplicationContext": {Ecosystem: "maven", Package: "org.springframework", Version: "6.1.0", Pinned: true},
		"import com.fasterxml.jackson.databind.ObjectMapper":    {Ecosystem: "maven", Package: "com.fasterxml.jackson.core", Version: "2.17.0", Pinned: true},
		// A version range is not one artifact.
		"import org.slf4j.Logger":                                     {Ecosystem: "maven", Package: "org.slf4j", Version: "[1.7,2.0)"},
		"import static org.junit.jupiter.api.Assertions.assertEquals": {Ecosystem: "maven", Package: "org.junit.jupiter", Version: "5.10.2", Pinned: true},
		"import okhttp3.OkHttpClient":                                 {Ecosystem: "maven", Package: "com.squareup.okhttp3", Version: "4.12.0", Pinned: true},
		// Packages sharing too little with their groupId come from knownGroups.
		"import com.google.common.collect.Lists": {Ecosystem: "maven", Package: "com.google.guava", Version: "33.0.0-jre", Pinned: true},
		"import org.junit.Test":                  {Ecosystem: "maven", Package: "junit", Version: "4.13.2", Pinned: true},
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

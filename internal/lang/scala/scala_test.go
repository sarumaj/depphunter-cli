package scala

import (
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// testdata/repo: an sbt build (a version held in a val, %% dependencies, a test
// dependency, sbt plugins under project/) with Scala files declaring packages they do
// not sit in, and a Kotlin file the Scala code imports.
func analyze(t *testing.T) map[string]*lang.FileResult {
	t.Helper()
	return langtest.Analyze(t, Plugin{}, "testdata/repo")
}

// Verifies: REQ-SCALA-001, REQ-SCALA-002, REQ-SCALA-003, REQ-SCALA-004, REQ-SCALA-005, REQ-SCALA-006
// Verifies: REQ-SCALA-007
func TestResolution(t *testing.T) {
	// cSpell: disable
	res := analyze(t)["src/main/scala/com/example/app/Main.scala"]
	std := func(pkg string) lang.Target { return lang.Target{Ecosystem: "scala-std", Package: pkg} }
	cats := lang.Target{Ecosystem: "maven", Package: "org.typelevel", Version: "3.5.4", Pinned: true}
	akka := lang.Target{Ecosystem: "maven", Package: "com.typesafe.akka", Version: "2.8.5", Pinned: true}
	model := lang.Target{Local: "src/main/scala/domain.scala"}
	langtest.CheckImports(t, res, map[string]lang.Target{
		"import scala.collection.mutable":                std("scala.collection"),
		"import scala.concurrent.Future":                 std("scala.concurrent"),
		"import scala.concurrent.ExecutionContext => EC": std("scala.concurrent"),
		"import scala.util.Try":                          std("scala.util"),
		"import scala.xml.Elem":                          {Ecosystem: "maven", Package: "org.scala-lang.modules", Version: "2.2.0", Pinned: true},
		// Not declared: scala-xml's first word does not claim it.
		"import scala.scalajs.js":                   {Ecosystem: "maven", Package: "scala.scalajs", Unresolved: true},
		"import java.time.Instant":                  {Ecosystem: "jdk", Package: "java.time"},
		"import cats.effect._":                      cats,
		"import cats.syntax.all.*":                  cats,
		"import io.circe.given":                     {Ecosystem: "maven", Package: "io.circe", Version: "0.14.+"},
		"import akka.actor.typed.ActorSystem as AS": akka,
		"import akka.actor.typed._":                 akka,
		"import com.example.app.model.User":         model,
		"import com.example.app.model.Order":        model,
		"import com.example.app.util._":             {Local: "src/main/scala/com/example/app/util"},
		"import com.google.common.base.Strings":     {Ecosystem: "maven", Package: "com.google.guava", Version: "33.0.0-jre", Pinned: true},
		"import org.acme.Tools":                     {Local: "src/main/kotlin/org/acme/Tools.kt"},
		"import org.scalatest.flatspec.AnyFlatSpec": {Ecosystem: "maven", Package: "org.scalatest", Version: "3.2.18", Pinned: true},
		// project/plugins.sbt configures sbt, not the code.
		"import com.github.sbt.git.GitPlugin": {Ecosystem: "maven", Package: "com.github.sbt", Unresolved: true},
		"import com.example.generated.Gen":    {},
		// Imports are relative: to the packages around the file, then to scala._ -
		// though not where a dependency owns the root (io.circe above).
		"import model.Order":                  model,
		"import util.Text":                    {Local: "src/main/scala/com/example/app/util/Text.scala"},
		"import collection.immutable.ListMap": std("scala.collection"),
		"import scala.Option":                 std("scala"),
		"import _root_.cats.effect.IO":        cats,
		// The members of a value in scope.
		"import builder._": {},
	})
	// cSpell: enable
}

// Verifies: REQ-SCALA-001
func TestSymbols(t *testing.T) {
	langtest.CheckSymbols(t, analyze(t)["src/main/scala/com/example/app/Main.scala"], map[string]string{
		"defaultName": "var", "greet": "func", "Service": "trait", "Service.serve": "method",
		"App": "class", "App.serve": "method", "App.helper": "method", "App@39": "object", "App.main": "method",
		"Point": "class", "Color": "enum", "Color.rgb": "method", "Name": "type", "shout": "func",
	})
}

// Verifies: REQ-SCALA-001
func TestExpandImport(t *testing.T) {
	for text, want := range map[string][]string{
		"import a.b.C":                     {"a.b.C"},
		"import a.b._":                     {"a.b.*"},
		"import a.b.{C, D => E}":           {"a.b.C", "a.b.D"},
		"import a.b.{C => _, _}":           {"a.b.*"}, // C is hidden, not imported
		"import a.b.{given, C}":            {"a.b.*", "a.b.C"},
		"import a.b.{given Ordering[Int]}": {"a.b.*"},
		"import a.b as c":                  {"a.b"},
		"import a.B, c.D":                  {"a.B", "c.D"},
		"import a.\n  b.{\n  C,\n  D\n}":   {"a.b.C", "a.b.D"},
		"import scala":                     nil,
	} {
		var got []string
		for _, imp := range expandImport(text) {
			got = append(got, imp.Module)
		}
		if len(got) != len(want) {
			t.Errorf("%q: got %v, want %v", text, got, want)
			continue
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("%q: got %v, want %v", text, got, want)
			}
		}
	}
}

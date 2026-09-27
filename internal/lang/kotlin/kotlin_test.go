package kotlin

import (
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/java"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// testdata/repo: a Gradle project (Kotlin DSL, version catalog) whose Kotlin files do
// not all sit in directories named after their packages, beside a Java module and a
// Scala file, so imports cross between the three languages.
func analyze(t *testing.T) map[string]*lang.FileResult {
	t.Helper()
	return langtest.Analyze(t, Plugin{}, "testdata/repo")
}

// Verifies: REQ-KT-001, REQ-KT-002, REQ-KT-003, REQ-KT-004
func TestResolution(t *testing.T) {
	// cSpell: disable
	res := analyze(t)["src/main/kotlin/com/example/app/App.kt"]
	langtest.CheckImports(t, res, map[string]lang.Target{
		"import kotlinx.coroutines.launch": {Ecosystem: "maven", Package: "org.jetbrains.kotlinx:kotlinx-coroutines-core", Version: "1.8.0", Pinned: true},
		"import kotlin.collections.List":   {Ecosystem: "kotlin-std", Package: "kotlin.collections"},
		"import kotlin.math.*":             {Ecosystem: "kotlin-std", Package: "kotlin.math"},
		// A function of the root package belongs to it.
		"import kotlin.require": {Ecosystem: "kotlin-std", Package: "kotlin"},
		"import java.util.UUID": {Ecosystem: "jdk", Package: "java.util"},
		// Found by the package Models.kt declares, not by where it sits.
		"import com.example.app.model.User": {Local: "src/main/kotlin/models/Models.kt"},
		"import com.example.app.util.*":     {Local: "src/main/kotlin/com/example/app/util"},
		"import com.example.app.util.shout": {Local: "src/main/kotlin/com/example/app/util/Strings.kt"},
		"import com.example.legacy.Legacy":  {Local: "legacy/src/main/scala/Legacy.scala"},
		"import org.acme.net.Client":        {Local: "lib/src/main/java/org/acme/net/Client.java"},
		// A Gradle dynamic version moves with every release.
		"import io.ktor.client.HttpClient":               {Ecosystem: "maven", Package: "io.ktor:ktor-client-core", Version: "2.3.+"},
		"import okhttp3.OkHttpClient as Http":            {Ecosystem: "maven", Package: "com.squareup.okhttp3:okhttp", Version: "4.12.0", Pinned: true},
		"import com.google.common.collect.ImmutableList": {Ecosystem: "maven", Package: "com.google.guava:guava", Version: "33.0.0-jre", Pinned: true},
		"import com.example.generated.Gen":               {},
	})
	// cSpell: enable
}

// A class of the default package, declared by the build script itself, is no
// dependency.
//
// Verifies: REQ-KT-006
func TestDefaultPackage(t *testing.T) {
	langtest.CheckImports(t, analyze(t)["build.gradle.kts"], map[string]lang.Target{"import Mode.FAST": {}})
}

// Java finds Kotlin classes, and the facade class of a Kotlin file's top-level
// functions, through the same index.
//
// Verifies: REQ-KT-003
func TestJavaImportsKotlin(t *testing.T) {
	res := langtest.Analyze(t, java.Plugin{}, "testdata/repo")["lib/src/main/java/org/acme/net/Client.java"]
	langtest.CheckImports(t, res, map[string]lang.Target{
		"import com.example.app.model.User":     {Local: "src/main/kotlin/models/Models.kt"},
		"import com.example.app.util.StringsKt": {Local: "src/main/kotlin/com/example/app/util/Strings.kt"},
	})
}

// Verifies: REQ-KT-001
func TestSymbols(t *testing.T) {
	langtest.CheckSymbols(t, analyze(t)["src/main/kotlin/com/example/app/App.kt"], map[string]string{
		"VERSION": "var", "main": "func", "App": "class", "App.run": "method", "App.helper": "method",
		"App.create": "method", "Service": "interface", "Service.serve": "method", "Registry": "object",
		"Registry.lookup": "method", "Mode": "enum", "Mode.weight": "method", "Point": "class", "Name": "type",
	})
	langtest.CheckSymbols(t, analyze(t)["src/main/kotlin/models/Models.kt"], map[string]string{
		"User": "class", "Shape": "interface",
	})
}

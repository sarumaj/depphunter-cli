package java

import (
	"strings"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// declared renders what declarations found as "package: names; package: names".
func declared(found []packageDeclarations) string {
	var parts []string
	for _, each := range found {
		if len(each.names) == 0 {
			parts = append(parts, each.name)
			continue
		}
		parts = append(parts, each.name+": "+strings.Join(each.names, " "))
	}
	return strings.Join(parts, "; ")
}

// Top-level is decided by nesting, not by column: members at any column stay out,
// top-level definitions at any indentation are read, and nothing inside a comment,
// a string, a character or a template counts - neither its words nor its brackets.
//
// Verifies: REQ-KT-005, REQ-KT-007, REQ-SCALA-003
func TestDeclarationsByNesting(t *testing.T) {
	for _, test := range []struct {
		name, source string
		scala        bool
		want         string
	}{
		{"kotlin members at column 0", "package a\nclass A {\nfun inner() = 1\nclass Nested\n}\nfun outer() = 2\n", false,
			"a: A outer"},
		{"kotlin raw string holding code", "package a\nval sql = \"\"\"\nclass NotThis {\n\"\"\"\nclass Real\n", false,
			"a: sql Real"},
		{"kotlin template with braces and quotes", "package a\nval s = \"${mapOf(\"k\" to \"}\")[\"k\"]} {\"\nclass B\n", false,
			"a: s B"},
		{"kotlin raw string template", "package a\nval s = \"\"\"${ if (x) \"\"\"}\"\"\" else \"{\" }\"\"\"\nobject C\n", false,
			"a: s C"},
		{"kotlin char literals", "package a\nval q = '\"'\nval b = '{'\nval e = '\\''\nval u = '\\u007B'\nclass D\n", false,
			"a: q b e u D"},
		{"nested comments", "package a\n/* outer /* inner */\nclass NotThis {\n*/\nclass E\n", false,
			"a: E"},
		{"line comment with brace", "package a\nclass F { // }\n  fun g() = 1\n}\nclass G\n", false,
			"a: F G"},
		{"constructor parameters", "package a\nclass H(\nval x: Int,\nvar y: Int,\n)\n", false,
			"a: H"},
		{"continuation lines", "package a\nclass I\n    : Base()\nfun j() =\n    listOf(1)\n", false,
			"a: I j"},
		{"backquoted names", "package a\nfun `a { b`() = 1\nclass K\n", false,
			"a: K"},
		{"scala package blocks", "package com.acme\npackage app {\n  class A\n  object B {\n    class Nested\n  }\n}\npackage lib {\n  trait C\n}\n", true,
			"com.acme.app: A B; com.acme.lib: C; com.acme"},
		{"scala nested package blocks", "package outer {\n  package inner {\n    class A\n  }\n  class B\n}\n", true,
			"outer: B; outer.inner: A"},
		{"scala 3 colon bodies", "package a\nobject O:\n  def member = 1\n  class Nested\nend O\ndef top = 2\nenum E:\n  case X\n", true,
			"a: O top E"},
		{"scala 3 package with colon", "package a:\n  class A\n  object B:\n    class Nested\npackage b:\n  trait C\n", true,
			"a: A B; b: C"},
		{"scala interpolators", "package a\nval x = s\"${m(\"}\")} {\"\nval y = raw\"\\\" + \"{\"\nval z = f\"$$ {\"\nval w = s\"$\" {\"\nclass D\n", true,
			"a: x y z w D"},
		{"quotes ending a triple-quoted string", "package a\nval x = \"\"\"a\"\"\"\" + \"{\"\nclass B\n", false,
			"a: x B"},
		{"quotes ending a scala triple-quoted string", "package a\nval x = \"\"\"a\"\"\"\" + \"{\"\nclass B\n", true,
			"a: x B"},
		{"scala triple-quoted strings", "package a\nval x = s\"\"\"\ncase class NotThis(\n${ \"\"\"}\"\"\" }\"\"\"\"\nclass E\n", true,
			"a: x E"},
		{"scala symbols and quotes", "package a\nval s = 'sym\nval q = '{ x }\nclass F\n", true,
			"a: s q F"},
		{"default package", "class A\n", false, ": A"},
	} {
		if got := declared(declarations([]byte(test.source), test.scala)); got != test.want {
			t.Errorf("%s: got %q, want %q", test.name, got, test.want)
		}
	}
}

// Input no compiler accepts - literals and comments left open, brackets that never
// close or close too often, templates nested past the bound - ends in bounded time
// without a panic, and what came before the damage is still read.
//
// Verifies: REQ-KT-005, REQ-KT-007
func TestDeclarationsPathological(t *testing.T) {
	deep := strings.Repeat("{", 200000)
	for _, test := range []struct {
		name, source, want string
	}{
		{"unterminated string", "package a\nclass A\nval s = \"open\nclass B\n", "a: A s B"},
		{"unterminated raw string", "package a\nclass A\nval s = \"\"\"open\nclass NotThis\n", "a: A s"},
		{"unterminated comment", "package a\nclass A\n/* open\nclass NotThis\n", "a: A"},
		{"unterminated nested comment", "package a\nclass A\n/* /* */\nclass NotThis\n", "a: A"},
		{"unterminated template", "package a\nclass A\nval s = s\"${ f(\nclass NotThis\n", "a: A s"},
		{"unterminated char", "package a\nclass A\nval c = '\nclass B\n", "a: A c B"},
		{"deep braces", "package a\nclass A\n" + deep + "\nclass NotThis\n", "a: A"},
		{"stray closers", "package a\n}}})])\nclass A\n", "a: A"},
		{"nested templates", "package a\nclass A\nval s = " + strings.Repeat("\"${", 5000) + "\n", "a: A s"},
		{"long line of quotes", "package a\nclass A\n" + strings.Repeat("\"", 100001) + "\n", "a: A"},
		{"only a quote", "'", ""},
		{"only a backslash char", "'\\", ""},
		{"package block never closed", "package a {\n  class A\n" + deep, "a: A"},
	} {
		for _, scala := range []bool{false, true} {
			start := time.Now()
			got := declared(declarations([]byte(test.source), scala))
			if elapsed := time.Since(start); elapsed > langtest.TimeLimit(2*time.Second) {
				t.Errorf("%s (scala %v): %v", test.name, scala, elapsed)
			}
			if got != test.want && !(test.name == "package block never closed" && !scala) {
				t.Errorf("%s (scala %v): got %q, want %q", test.name, scala, got, test.want)
			}
		}
	}
}

// A fuzz-like sweep: every prefix, and every one-byte hole, of sources mixing all the
// constructs the scanner knows must neither panic nor take long.
//
// Verifies: REQ-KT-007
func TestDeclarationsTruncated(t *testing.T) {
	sources := []string{
		"package a\n/* c /* n */ */\nclass A(val x: Int) {\n  val s = \"${x} {\" + \"\"\"\n}\"\"\"\n  val c = '{'\n}\nfun `b c`() = 1\n",
		"package a {\n  object B:\n    val s = s\"${m(\"}\")} $$ {\"\n    val r = raw\"\\\"\n  end B\n}\npackage b:\n  trait C\n",
	}
	start := time.Now()
	for _, source := range sources {
		for _, scala := range []bool{false, true} {
			for cut := 0; cut <= len(source); cut++ {
				declarations([]byte(source[:cut]), scala)
				if cut < len(source) {
					declarations([]byte(source[:cut]+source[cut+1:]), scala)
				}
			}
		}
	}
	if elapsed := time.Since(start); elapsed > langtest.TimeLimit(5*time.Second) {
		t.Errorf("sweep took %v", elapsed)
	}
}

// The resolver indexes each package block of a Scala file under its own package,
// and resolves the file's relative imports against the package they share.
//
// Verifies: REQ-KT-007, REQ-SCALA-003
func TestPackageBlocksResolve(t *testing.T) {
	scala := Language{Std: "scala-std", Prefixes: []string{"scala."}, Relative: true}
	files := map[string]string{
		"src/blocks.scala": "package com.acme\npackage app {\n  class A\n}\npackage lib {\n  object B {\n    class Inner\n  }\n}\n",
		"src/other.kt":     "package org.x\n\nclass Top {\nclass Member\n}\n",
	}
	got := resolveIn(t, scala, files, "com.acme.app.A", "com.acme.lib.B", "com.acme.lib.B.Inner", "com.acme.lib.*", "org.x.Member")
	checkTargets(t, got, map[string]lang.Target{
		"com.acme.app.A":       {Local: "src/blocks.scala"},
		"com.acme.lib.B":       {Local: "src/blocks.scala"},
		"com.acme.lib.B.Inner": {Local: "src/blocks.scala"},
		"com.acme.lib.*":       {Local: "src"},
	})
	if got["org.x.Member"].Local != "" {
		t.Errorf("a member at column 0 was indexed: %+v", got["org.x.Member"])
	}
	r := newResolver(langtest.Files(t, langtest.Write(t, files)), scala)
	if relative := r.filePackage["src/blocks.scala"]; relative != "com.acme" {
		t.Errorf("imports of blocks.scala relative to %q, want com.acme", relative)
	}
}

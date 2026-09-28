package clojure

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// The fixture is a shop: deps.edn with aliases, a git dependency pinned by its sha
// and one with a tag only, a :local/root project (libs/shared) and a repository;
// src/ holds Clojure and a .cljc with reader conditionals; web/ is a shadow-cljs
// build with a package.json; lein/ a Leiningen project with a profile, a plugin and
// a computed version; scripts/ a babashka project with a bb.edn, a .bb script and
// an extensionless one; java/ a Java class the Clojure code imports.

func mvn(packageName, version string) lang.Target {
	pinned := lang.PinnedMaven(version)
	return lang.Target{Ecosystem: ecosystemMaven, Package: packageName, Version: version, Pinned: pinned, Floating: version != "" && !pinned}
}

var (
	widgets = lang.Target{Ecosystem: ecosystemMaven, Package: "io.github.acme:widgets", Version: "0123456789abcdef0123456789abcdef01234567",
		Requested: "v1.2.0", Pinned: true, Origin: "https://github.com/acme/widgets.git"}
	cheshire = mvn("cheshire:cheshire", "5.12.0")
	bb       = lang.Target{Ecosystem: ecosystemStd, Package: "babashka"}
)

func stdNS(packageName string) lang.Target {
	return lang.Target{Ecosystem: ecosystemStd, Package: packageName}
}

// Verifies: REQ-CLOJURE-002, REQ-CLOJURE-004, REQ-CLOJURE-005, REQ-CLOJURE-006, REQ-CLOJURE-007
func TestNamespaces(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckImports(t, results["src/shop/core.clj"], map[string]lang.Target{
		"clojure.string":                  stdNS("clojure.string"),
		"clojure.java.io":                 stdNS("clojure.java.io"),
		"cheshire.core":                   cheshire,                                                       // artifact = first segment
		"ring.util.response":              mvn("ring:ring-core", "1.11.0"),                                // table
		"next.jdbc":                       mvn("com.github.seancorfield:next.jdbc", "1.3.909"),            // artifact = two segments
		"reitit.ring":                     mvn("metosin:reitit-ring", "0.7.0"),                            // reitit-ring, folded
		"taoensso.timbre":                 mvn("com.taoensso:timbre", "RELEASE"),                          // group, then artifact; RELEASE floats
		"clojure.core.async":              mvn("org.clojure:core.async", "[1.6,2.0)"),                     // contrib; a range floats
		"clojure.spec.alpha":              {Ecosystem: ecosystemMaven, Package: "org.clojure:spec.alpha"}, // Clojure's own dependency
		"clojure.data.json":               {Ecosystem: ecosystemMaven, Package: "org.clojure:data.json", Unresolved: true},
		"acme.widgets.api":                widgets,                                         // git dependency
		"acme.shared.util":                {Local: "libs/shared/src/acme/shared/util.clj"}, // :local/root
		"shop.db-util":                    {Local: "src/shop/db_util.clj"},                 // - is _
		"shop.model.cart":                 {Local: "src/shop/model/cart.clj"},              // prefix list
		"shop.model.order":                {Local: "src/shop/model/order.clj"},             // prefix list
		"unknown.lib":                     {Ecosystem: ecosystemMaven, Package: "unknown:unknown", Unresolved: true},
		"shop.gone":                       {}, // the project's own namespace, missing
		"java.util.Date":                  {Ecosystem: ecosystemJDK, Package: "java.util"},
		"java.util.UUID":                  {Ecosystem: ecosystemJDK, Package: "java.util"},
		"java.io.File":                    {Ecosystem: ecosystemJDK, Package: "java.io"},
		"org.eclipse.jetty.server.Server": mvn("org.eclipse.jetty:jetty-server", "11.0.18"),
		"shop.model.cart.Cart":            {Local: "src/shop/model/cart.clj"}, // a defrecord's class
		"shop.Native":                     {Local: "java/shop/Native.java"},
		"clojure.lang.IFn":                stdNS("clojure.lang"),
		"com.example.Nothing":             {},
		// Classes of Maven artifacts, matched as the Java plugin matches imports: the
		// table's package (com.google.common is Guava's), an artifact arriving with a
		// declared one of its group at their shared version; a class of a jar only a
		// dependency brings, or one the table places in an undeclared artifact, is dropped.
		"com.google.common.collect.ImmutableList":       mvn("com.google.guava:guava", "33.0.0-jre"),
		"com.fasterxml.jackson.annotation.JsonProperty": mvn("com.fasterxml.jackson.core:jackson-annotations", "2.17.0"),
		"com.google.common.jimfs.Jimfs":                 {},
		"org.quartz.JobKey":                             {},
		"clojure.set":                                   stdNS("clojure.set"), // top-level (require '[...])
		`(load "core_extra")`:                           {Local: "src/shop/core_extra.clj"},
		// shop.alias-only (:as-alias) loads nothing; #_ discards shop.never.
	})
	// Reader conditionals: every branch is read, spliced or not; the .cljc's
	// require of its own macros is no edge.
	langtest.CheckImports(t, results["src/shop/common.cljc"], map[string]lang.Target{
		"clojure.edn":        stdNS("clojure.edn"),
		"cljs.reader":        stdNS("cljs.reader"),
		"clojure.java.shell": stdNS("clojure.java.shell"),
		"goog.object":        stdNS("goog"),
		"clojure.spec.alpha": {Ecosystem: ecosystemMaven, Package: "org.clojure:spec.alpha"},
		"shop.common":        {},
	})
	langtest.CheckImports(t, results["test/shop/core_test.clj"], map[string]lang.Target{
		"clojure.test":      stdNS("clojure.test"),
		"shop.core":         {Local: "src/shop/core.clj"},
		"kaocha.repl":       mvn("lambdaisland:kaocha", "1.87.1366"), // an alias's :extra-deps
		"ring.mock.request": {Ecosystem: ecosystemMaven, Package: "ring:ring-mock", Unresolved: true},
	})
	// The :local/root project sees its own dependencies and its users' root.
	langtest.CheckImports(t, results["libs/shared/src/acme/shared/util.clj"], map[string]lang.Target{
		"medley.core":   mvn("medley:medley", "1.4.0"),
		"cheshire.core": cheshire,
	})
	langtest.CheckImports(t, results["lein/src/clj/legacy/handler.clj"], map[string]lang.Target{
		"compojure.core":    mvn("compojure:compojure", "1.7.0"), // :use
		"clj-http.client":   mvn("clj-http:clj-http", "3.12.3"),
		"hiccup.page":       mvn("hiccup:hiccup", "1.0.5"),  // ~version, managed
		"ring.mock.request": mvn("ring:ring-mock", "0.4.0"), // a profile's dependency
		"legacy.views":      {Local: "lein/src/clj/legacy/views.clj"},
	})
}

// Verifies: REQ-CLOJURE-009, REQ-CLOJURE-005
func TestClojureScript(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckImports(t, results["web/src/shop/ui.cljs"], map[string]lang.Target{
		`"react"`:                {Ecosystem: ecosystemNPM, Package: "react", Version: "18.2.0", Pinned: true},
		`"@mui/material/Button"`: {Ecosystem: ecosystemNPM, Package: "@mui/material", Version: "^5.15.0"},
		`"fs"`:                   {Ecosystem: ecosystemNode, Package: "fs"},
		`"./local.js"`:           {Local: "web/src/shop/local.js"},
		`"lodash"`:               {Ecosystem: ecosystemNPM, Package: "lodash", Unresolved: true},
		"reagent.core":           mvn("reagent:reagent", "1.2.0"), // shadow-cljs.edn :dependencies
		"re-frame.core":          mvn("re-frame:re-frame", "1.4.2"),
		"goog.string":            stdNS("goog"),
		"cljs.core.async":        mvn("org.clojure:core.async", "[1.6,2.0)"),
		"shop.common":            {Local: "src/shop/common.cljc"},                                                // the root project's source path
		"left-pad":               {Ecosystem: ecosystemNPM, Package: "left-pad", Version: "1.3.0", Pinned: true}, // a symbol naming an npm package
		"shop.macros":            {Local: "web/src/shop/macros.clj"},                                             // :require-macros
		"goog.net.XhrIo":         stdNS("goog"),
	})
}

// Verifies: REQ-CLOJURE-004, REQ-CLOJURE-005
func TestBabashka(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckImports(t, results["scripts/release.bb"], map[string]lang.Target{
		"babashka.fs":          mvn("babashka:fs", "0.5.20"), // declared by bb.edn
		"babashka.http-client": bb,                           // built into bb
		"cheshire.core":        cheshire,                     // the root deps.edn's
	})
	langtest.CheckImports(t, results["scripts/tool"], map[string]lang.Target{
		"babashka.process":                 bb,
		`(load-file "scripts/release.bb")`: {Local: "scripts/release.bb"},
	})
	langtest.CheckImports(t, results["scripts/bb.edn"], map[string]lang.Target{
		"babashka/fs":      mvn("babashka:fs", "0.5.20"),
		"babashka.process": bb, // tasks :requires
		"clojure.string":   stdNS("clojure.string"),
	})
}

// Verifies: REQ-CLOJURE-004, REQ-CLOJURE-008, REQ-CLOJURE-010
func TestManifests(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckImports(t, results["deps.edn"], map[string]lang.Target{
		"org.clojure/clojure":               mvn("org.clojure:clojure", "1.11.1"),
		"cheshire/cheshire":                 cheshire, // the main :deps before the :dev alias
		"ring/ring-core":                    mvn("ring:ring-core", "1.11.0"),
		"com.github.seancorfield/next.jdbc": mvn("com.github.seancorfield:next.jdbc", "1.3.909"),
		"metosin/reitit-ring":               mvn("metosin:reitit-ring", "0.7.0"),
		"com.taoensso/timbre":               mvn("com.taoensso:timbre", "RELEASE"),
		"org.clojure/core.async":            mvn("org.clojure:core.async", "[1.6,2.0)"),
		"io.github.acme/widgets":            widgets,
		"io.github.acme/gadgets": {Ecosystem: ecosystemMaven, Package: "io.github.acme:gadgets", Version: "v0.3",
			Origin: "https://github.com/acme/gadgets.git"}, // a tag alone: neither pinned nor floating
		"acme/shared":                                 {Local: "libs/shared/deps.edn"},
		"org.eclipse.jetty/jetty-server":              mvn("org.eclipse.jetty:jetty-server", "11.0.18"),
		"com.google.guava/guava":                      mvn("com.google.guava:guava", "33.0.0-jre"),
		"com.fasterxml.jackson.core/jackson-databind": mvn("com.fasterxml.jackson.core:jackson-databind", "2.17.0"),
		"lambdaisland/kaocha":                         mvn("lambdaisland:kaocha", "1.87.1366"),
		"integrant/repl":                              mvn("integrant:repl", "0.3.3"),
	})
	langtest.CheckImports(t, results["lein/project.clj"], map[string]lang.Target{
		"org.clojure/clojure": mvn("org.clojure:clojure", "1.10.3"),
		"compojure":           mvn("compojure:compojure", "1.7.0"), // no group: compojure/compojure
		"clj-http":            mvn("clj-http:clj-http", "3.12.3"),
		"hiccup":              mvn("hiccup:hiccup", "1.0.5"),
		"lein-ring":           mvn("lein-ring:lein-ring", "0.12.6"), // a plugin
		"ring/ring-mock":      mvn("ring:ring-mock", "0.4.0"),
	})
	langtest.CheckImports(t, results["web/shadow-cljs.edn"], map[string]lang.Target{
		"reagent":  mvn("reagent:reagent", "1.2.0"),
		"re-frame": mvn("re-frame:re-frame", "1.4.2"),
	})
	langtest.CheckImports(t, results["resources/config.edn"], map[string]lang.Target{})
	for _, imported := range results["lein/project.clj"].Imports {
		if imported.Spec == "lein-ring" && imported.Line != 10 {
			t.Errorf("lein-ring on line %d, want 10", imported.Line)
		}
	}
}

// Verifies: REQ-CLOJURE-003
func TestSymbols(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckSymbols(t, results["src/shop/core.clj"], map[string]string{
		"shop.core": "namespace", "helper": "func", "start": "func", "*config*": "var", "state": "var",
		"with-shop": "macro", "area": "func", "area :circle": "method", "area [:square :big]": "method",
		"Priced": "interface", "Priced.price": "method", "Priced.discount": "method", "Item": "class",
		"Box": "class", "last-one": "var",
		// Not symbols: a (comment ...) block's defn, an s/def spec.
	})
	langtest.CheckSymbols(t, results["src/shop/common.cljc"], map[string]string{
		"shop.common": "namespace", "now": "func", "when-shop": "macro", // now once, from both branches
	})
	langtest.CheckSymbols(t, results["test/shop/core_test.clj"], map[string]string{"shop.core-test": "namespace", "starts": "test"})
	langtest.CheckSymbols(t, results["deps.edn"], map[string]string{})
	langtest.CheckSymbols(t, results["resources/config.edn"], map[string]string{})
}

// Verifies: REQ-CLOJURE-001
func TestClaims(t *testing.T) {
	for p, want := range map[string]bool{
		"src/a.clj": true, "src/a.cljs": true, "src/a.cljc": true, "x.bb": true, "deps.edn": true,
		"conf/any.edn": true, "project.clj": true, "build.boot": true, "bb.edn": true, "shadow-cljs.edn": true,
		"a.java": false, ".shadow-cljs/builds/a.cljs": false, "node_modules/x/a.cljs": false, ".cpcache/x.edn": false,
	} {
		if got := (Plugin{}).Claims(&scan.File{Path: p}); got != want {
			t.Errorf("Claims(%s) = %v, want %v", p, got, want)
		}
	}
	if !(Plugin{}).Claims(&scan.File{Path: "bin/tool", Interpreter: "bb"}) {
		t.Error("a bb script is not claimed")
	}
	if (Plugin{}).Claims(&scan.File{Path: "bin/tool.py", Interpreter: "bb"}) {
		t.Error("a script with another language's extension is claimed")
	}
	// project.clj is a manifest, core.clj source: the same extension, told apart.
	if lang.ClassOf(Plugin{}, &scan.File{Path: "project.clj"}) == lang.ClassOf(Plugin{}, &scan.File{Path: "core.clj"}) {
		t.Error("project.clj and core.clj share a cache class")
	}
}

// Verifies: REQ-CLOJURE-007
func TestScore(t *testing.T) {
	// strong: >= 50, used whenever declared; weak: only the group in common, used
	// for namespaces no table knows.
	for _, c := range []struct{ namespace, art, want string }{
		{"cheshire.core", "cheshire:cheshire", "strong"},
		{"next.jdbc.result-set", "com.github.seancorfield:next.jdbc", "strong"},
		{"honey.sql.helpers", "com.github.seancorfield:honeysql", "strong"},
		{"taoensso.timbre", "com.taoensso:timbre", "strong"},
		{"day8.re-frame.http-fx", "day8.re-frame:http-fx", "strong"},
		{"ring.mock.request", "ring:ring-mock", "strong"},
		{"jmh.core", "jmh-clojure:jmh-clojure", "strong"},
		{"datomic.api", "com.datomic:datomic-pro", "weak"},
		{"ring.util.response", "ring:ring-jetty-adapter", "weak"},
		{"babashka.curl", "babashka:fs", "none"},
		{"foo.core", "bar:baz", "none"},
	} {
		got := "none"
		if s := score(c.namespace, c.art); s >= 50 {
			got = "strong"
		} else if s > 0 {
			got = "weak"
		}
		if got != c.want {
			t.Errorf("score(%s, %s) = %d, %s, want %s", c.namespace, c.art, score(c.namespace, c.art), got, c.want)
		}
	}
	if s := score("cognitect.transit", "com.cognitect:transit-clj"); s != 40 {
		t.Errorf("cognitect.transit: %d, want 40 (group, then a word of the artifact)", s)
	}
	// The artifact named after more of the namespace wins.
	if score("reitit.ring", "metosin:reitit-ring") <= score("reitit.ring", "metosin:reitit") {
		t.Error("reitit.ring prefers metosin/reitit over reitit-ring")
	}
	if score("integrant.repl", "integrant:repl") <= score("integrant.repl", "integrant:integrant") {
		t.Error("integrant.repl prefers integrant/integrant over integrant/repl")
	}
}

// Verifies: REQ-CLOJURE-008
func TestGitURLs(t *testing.T) {
	for library, want := range map[string]string{
		"io.github.acme/widgets":  "https://github.com/acme/widgets.git",
		"com.github.acme/widgets": "https://github.com/acme/widgets.git",
		"io.gitlab.acme/widgets":  "https://gitlab.com/acme/widgets.git",
		"ht.sr.acme/widgets":      "https://git.sr.ht/~acme/widgets",
		"acme/widgets":            "",
	} {
		if got := inferGitURL(library); got != want {
			t.Errorf("inferGitURL(%s) = %q, want %q", library, got, want)
		}
	}
}

// Every prefix of every fixture file reads without a panic, and pathological input
// reads in linear time.
//
// Verifies: REQ-CLOJURE-011
func TestTruncated(t *testing.T) {
	files := langtest.Files(t, "testdata/repo")
	for _, f := range files {
		source, err := os.ReadFile(f.AbsolutePath)
		if err != nil {
			t.Fatal(err)
		}
		for n := 0; n <= len(source); n++ {
			if extraction, err := (Plugin{}).Extract(f, source[:n]); err != nil || extraction == nil {
				t.Fatalf("%s[:%d]: %v", f.Path, n, err)
			}
			for kind := range manifestNames {
				readManifest(kind, source[:n])
			}
			namespaceName(source[:n])
		}
	}
	for _, s := range []string{
		strings.Repeat("(", 200000), strings.Repeat(")", 200000), strings.Repeat("[{(", 50000),
		strings.Repeat("'", 200000), strings.Repeat("^", 200000), strings.Repeat("#_", 100000),
		strings.Repeat("#?(", 50000), strings.Repeat("#?@(:clj [", 30000), strings.Repeat("#:a{", 50000),
		strings.Repeat(`"`, 100001), strings.Repeat(`\`, 100001), strings.Repeat("#", 100000),
		strings.Repeat("(ns a (:require [b [c [d ", 20000), strings.Repeat("(defproject a \"1\" :dependencies [[", 20000),
		strings.Repeat("(require '[a.b :as c]) ", 20000), strings.Repeat("(ns x.y (:import (a.b C D E))) ", 10000),
	} {
		start := time.Now()
		readSource([]byte(s))
		for kind := range manifestNames {
			readManifest(kind, []byte(s))
		}
		if d := time.Since(start); d > 5*time.Second {
			t.Errorf("%q... took %v", s[:min(20, len(s))], d)
		}
	}
}

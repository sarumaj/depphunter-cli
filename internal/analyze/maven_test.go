package analyze

import (
	"context"
	"strings"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/graph"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/bazel"
	"github.com/sarumaj/depphunter-cli/internal/lang/clojure"
	"github.com/sarumaj/depphunter-cli/internal/lang/java"
)

// Java, Clojure and Bazel all name a Maven package group:artifact, so Guava reached
// through a Java import (declared by pom.xml), a deps.edn dependency, a Clojure
// :import of a Guava class (matched by the Java plugin's rules) and a Bazel
// maven.install artifact is one node.
//
// Verifies: REQ-JAVA-012, REQ-CLOJURE-006
func TestOneMavenNodeAcrossJavaClojureAndBazel(t *testing.T) {
	root := t.TempDir()
	writeProject(t, root, map[string]string{
		"pom.xml": `<project><groupId>com.example</groupId><artifactId>app</artifactId>
  <dependencies>
    <dependency><groupId>com.google.guava</groupId><artifactId>guava</artifactId><version>33.0.0-jre</version></dependency>
  </dependencies>
</project>`,
		"src/main/java/com/example/App.java": "package com.example;\n\nimport com.google.common.collect.Lists;\n\nclass App {}\n",
		"clj/deps.edn":                       `{:paths ["src"] :deps {com.google.guava/guava {:mvn/version "33.0.0-jre"}}}`,
		"clj/src/app/core.clj":               "(ns app.core\n  (:import (com.google.common.collect ImmutableList)))\n",
		"MODULE.bazel": `bazel_dep(name = "rules_jvm_external", version = "6.1")
maven = use_extension("@rules_jvm_external//:extensions.bzl", "maven")
maven.install(artifacts = ["com.google.guava:guava:33.0.0-jre"])
use_repo(maven, "maven")
`,
		"BUILD.bazel": `java_library(name = "app", srcs = glob(["src/main/java/**/*.java"]), deps = ["@maven//:com_google_guava_guava"])` + "\n",
	})
	g, _, err := Run(context.Background(), root, Options{Plugins: []lang.Plugin{java.Plugin{}, clojure.Plugin{}, bazel.Plugin{}}})
	if err != nil {
		t.Fatal(err)
	}
	var maven []string
	for _, n := range g.Nodes {
		if n.Kind == graph.KindPackage && n.Parent == graph.EcosystemID("maven") {
			maven = append(maven, n.Name)
		}
	}
	if len(maven) != 1 || maven[0] != "com.google.guava:guava" {
		t.Fatalf("maven packages %v, want only com.google.guava:guava", maven)
	}
	id := graph.PackageID("maven", "com.google.guava:guava")
	from := map[string]bool{}
	for _, e := range g.Edges {
		if e.To == id {
			from[e.From] = true
		}
	}
	for _, f := range []string{"src/main/java/com/example/App.java", "clj/deps.edn", "clj/src/app/core.clj", "BUILD.bazel"} {
		if !from[graph.FileID(f)] {
			t.Errorf("%s has no edge to %s (edges from %v)", f, id, strings.Join(keys(from), ", "))
		}
	}
	if n := byID(g)[id]; n.Version != "33.0.0-jre" {
		t.Errorf("version %q, want 33.0.0-jre", n.Version)
	}
}

func keys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

package rego

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// The fixture: policy/lib holds package lib.kubernetes in two files and
// lib.util; rules/bugs a package with a quoted segment (as Regal writes
// them); deny.rego imports them in every form (with an alias, a namespace,
// built-ins, external data), deny_test.rego its package and a rule of
// lib.util without importing it, ns.rego a package below an imported
// namespace.
//
// Verifies: REQ-REGO-002, REQ-REGO-004, REQ-REGO-007
func TestImports(t *testing.T) {
	res := langtest.Analyze(t, Plugin{}, "testdata/repo")
	k8s, k8sMore := lang.Target{Local: "policy/lib/kubernetes.rego"}, lang.Target{Local: "policy/lib/kubernetes_more.rego"}
	util := lang.Target{Local: "policy/lib/util.rego"}
	langtest.CheckImports(t, res["policy/deny.rego"], map[string]lang.Target{
		"data.lib.kubernetes (policy/lib/kubernetes.rego)":      k8s, // one per file of the package
		"data.lib.kubernetes (policy/lib/kubernetes_more.rego)": k8sMore,
		"data.lib.util as u":                       util,
		"data.external.inventory":                  {}, // no policy declares it: data or a bundle
		"data.rules.bugs.constant-condition as cc": {Local: "policy/rules/bugs/constant_condition.rego"},
		// import data.lib (a namespace), import rego.v1, future.keywords and
		// input are no edges; references the imports cover are not repeated.
	})
	langtest.CheckImports(t, res["policy/deny_test.rego"], map[string]lang.Target{
		"data.main":           {Local: "policy/deny.rego"},
		"data.lib.util.allow": util,
	})
	langtest.CheckImports(t, res["policy/ns.rego"], map[string]lang.Target{
		"data.lib.kubernetes.pods (policy/lib/kubernetes.rego)":      k8s,
		"data.lib.kubernetes.pods (policy/lib/kubernetes_more.rego)": k8sMore,
	})
	// A package's own files do not link each other.
	langtest.CheckImports(t, res["policy/lib/kubernetes.rego"], map[string]lang.Target{})
}

// Verifies: REQ-REGO-003
func TestSymbols(t *testing.T) {
	res := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckSymbols(t, res["policy/deny.rego"], map[string]string{
		"main": "package", "deny": "rule", "warn": "rule", "f": "function", "allow": "rule",
	})
	langtest.CheckSymbols(t, res["policy/lib/util.rego"], map[string]string{
		"lib.util": "package", "allow": "rule", "helper": "function", "ref.head.rule": "rule",
	})
	langtest.CheckSymbols(t, res["policy/rules/bugs/constant_condition.rego"], map[string]string{
		"rules.bugs.constant-condition": "package", "report": "rule",
	})
}

// Verifies: REQ-REGO-001
func TestClaims(t *testing.T) {
	for p, want := range map[string]bool{"a.rego": true, "policy/x_test.rego": true, ".manifest": false, "data.json": false} {
		if got := (Plugin{}).Claims(&scan.File{Path: p}); got != want {
			t.Errorf("%s: %v", p, got)
		}
	}
}

// Every prefix of the fixtures, and pathological inputs, are read in time
// without a panic.
//
// Verifies: REQ-REGO-006
func TestTruncated(t *testing.T) {
	var srcs [][]byte
	filepath.Walk("testdata", func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			if b, err := os.ReadFile(p); err == nil {
				srcs = append(srcs, b)
			}
		}
		return nil
	})
	for _, src := range srcs {
		for i := 0; i <= len(src); i++ {
			extractSource(src[:i])
			packageOf(src[:i])
		}
	}
	for _, unit := range []string{"{", "[", "(", "data.a", "import data.x as y\n", "y.z ", "`", "\"", "package a[\"b\"]", "\na.b.c := 1", "x.", "[\"x\"]"} {
		src := []byte(strings.Repeat(unit, 200_000/len(unit)))
		start := time.Now()
		extractSource(src)
		if d := time.Since(start); d > 2*time.Second {
			t.Errorf("%q x %d: %v", unit, 200_000/len(unit), d)
		}
	}
}

// Verifies: REQ-REGO-005
func TestEcosystems(t *testing.T) {
	if e := (Plugin{}).Ecosystems(); len(e) != 0 {
		t.Errorf("%+v", e)
	}
}

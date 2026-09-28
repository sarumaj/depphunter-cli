package cue

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

// The fixture is the module example.com/shop@v0 (cue.mod/module.cue with
// four dependencies, one written as a label chain, one without a version)
// beside a go.mod requiring k8s.io/api. cue.mod/gen holds what `cue get go`
// generated from k8s.io/api, net/http and the module's own Go package api/;
// cue.mod/usr augments k8s.io/api/apps/v1; cue.mod/pkg vendors two packages
// the old way, one with a module file of its own. schema/ has two files of
// package schema, one of package other and a sub-package. sub/ is a module
// whose module.cue is empty.
var (
	k8sAPI  = lang.Target{Ecosystem: ecoGo, Package: "k8s.io/api", Version: "v0.29.0", Pinned: true}
	schemas = lang.Target{Ecosystem: ecoCUE, Package: "github.com/acme/schemas", Version: "v0.3.1", Pinned: true}
	k8sReg  = lang.Target{Ecosystem: ecoCUE, Package: "cue.dev/x/k8s.io", Version: "v0.5.0", Pinned: true}
	chain   = lang.Target{Ecosystem: ecoCUE, Package: "github.com/acme/chain", Version: "v1.2.0", Pinned: true}
	unver   = lang.Target{Ecosystem: ecoCUE, Package: "github.com/acme/unversioned", Floating: true}
)

func stdPkg(p string) lang.Target { return lang.Target{Ecosystem: ecoStd, Package: p} }

// Verifies: REQ-CUE-002, REQ-CUE-004, REQ-CUE-005, REQ-CUE-006, REQ-CUE-007, REQ-CUE-010
func TestImports(t *testing.T) {
	res := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckImports(t, res["config/main.cue"], map[string]lang.Target{
		"strings":                                stdPkg("strings"),
		"encoding/json":                          stdPkg("encoding/json"),
		"list":                                   stdPkg("list"),
		"tool/exec":                              stdPkg("tool/exec"),
		"example.com/shop/schema (a.cue)":        {Local: "schema/a.cue"}, // one import per file of the package
		"example.com/shop/schema (b.cue)":        {Local: "schema/b.cue"},
		"example.com/shop/schema:other":          {Local: "schema/c.cue"},
		"example.com/shop/schema/sub":            {Local: "schema/sub/sub.cue"},
		"example.com/shop/nothere":               {},
		"k8s.io/api/core/v1":                     k8sAPI, // cue.mod/gen, required by go.mod
		"k8s.io/api/apps/v1":                     k8sAPI, // cue.mod/usr
		"net/http":                               {Ecosystem: ecoGoStd, Package: "net/http"},
		"example.com/shop/api":                   {Local: "api"}, // generated from the module's own Go package
		"github.com/acme/schemas/k8s":            schemas,
		"cue.dev/x/k8s.io/api/apps/v1:appsv1reg": k8sReg,
		"github.com/acme/chain/x":                chain,
		"github.com/acme/unversioned/y":          unver,
		"github.com/legacy/lib":                  {Ecosystem: ecoCUE, Package: "github.com/legacy/lib"}, // cue.mod/pkg with a module file
		"example.org/old/defs":                   {Ecosystem: ecoCUE, Package: "example.org/old"},
		"github.com/nobody/thing/pkg":            {Ecosystem: ecoCUE, Package: "github.com/nobody/thing", Unresolved: true},
		"unknownstd":                             {Ecosystem: ecoCUE, Package: "unknownstd", Unresolved: true},
	})
	// A module with an empty module.cue: the enclosing module's path.
	langtest.CheckImports(t, res["sub/x.cue"], map[string]lang.Target{
		"example.com/shop/schema:other": {Local: "schema/c.cue"},
	})
	langtest.CheckImports(t, res["cue.mod/module.cue"], map[string]lang.Target{
		"github.com/acme/schemas@v0":     schemas,
		"cue.dev/x/k8s.io@v0":            k8sReg,
		"github.com/acme/chain@v1":       chain,
		"github.com/acme/unversioned@v0": unver,
	})
	for p := range res {
		if strings.Contains(p, "cue.mod/gen/") || strings.Contains(p, "cue.mod/pkg/") || strings.Contains(p, "cue.mod/usr/") {
			t.Errorf("%s: a dependency tree was analyzed", p)
		}
	}
}

// Verifies: REQ-CUE-003
func TestSymbols(t *testing.T) {
	res := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckSymbols(t, res["config/main.cue"], map[string]string{ // cSpell: words interp
		"config": "package", "#Config": "type", "_#Hidden": "type", "_hidden": "field",
		"name": "field", "quoted-field": "field", "opt": "field", "req": "field", "a": "field",
		"aliased": "field", "import": "field", "fake": "field", "multi": "field", "raw": "field",
		"interp": "field", "items": "field", "obj": "field", "cmd": "field", "pod": "field",
		"deploy": "field", "header": "field", "item": "field", "other": "field", "sub": "field",
	})
	langtest.CheckSymbols(t, res["schema/a.cue"], map[string]string{"schema": "package", "#Product": "type"})
	langtest.CheckSymbols(t, res["cue.mod/module.cue"], map[string]string{"module": "field", "language": "field", "deps": "field"})
}

// Verifies: REQ-CUE-002, REQ-CUE-009
func TestLiteralsHideCode(t *testing.T) {
	src := []byte(`package x

// import "a"
s: "import \"b\""
m: """
	import "c"
	"""
r: ##"import "d" "# \##(1) import "e""##
i: "\(strings.Join(["import", "f"], ")"))"
b: '''
	import "g"
	'''
import "real"
`)
	ex := extractSource(src)
	if len(ex.Imports) != 1 || ex.Imports[0].Spec != "real" || ex.Imports[0].Line != 13 {
		t.Errorf("got %+v", ex.Imports)
	}
	names := map[string]bool{}
	for _, s := range ex.Symbols {
		names[s.Name] = true
	}
	for _, n := range []string{"s", "m", "r", "i", "b"} {
		if !names[n] {
			t.Errorf("%s: field lost after a string", n)
		}
	}
}

// Verifies: REQ-CUE-005
func TestModuleFile(t *testing.T) {
	mf := readModule([]byte(`module: "github.com/a/b@v1"
language: version: "v0.10.0"
deps: {
	"cue.dev/x/crd/cert-manager.io@v0": {v: "v0.1.0", default: true}
}
deps: "github.com/c/d@v2": v: "v2.0.1"
`))
	if mf.path != "github.com/a/b" || len(mf.deps) != 2 {
		t.Fatalf("got %+v", mf)
	}
	if d := mf.deps[0]; d.key != "cue.dev/x/crd/cert-manager.io@v0" || d.path != "cue.dev/x/crd/cert-manager.io" || d.v != "v0.1.0" || d.line != 4 {
		t.Errorf("got %+v", d)
	}
	if d := mf.deps[1]; d.path != "github.com/c/d" || d.v != "v2.0.1" {
		t.Errorf("got %+v", d)
	}
	if p, q := splitImport("example.com/x@v0/y:z"); p != "example.com/x/y" || q != "z" {
		t.Errorf("splitImport: %s %s", p, q)
	}
}

// Verifies: REQ-CUE-001
func TestClaims(t *testing.T) {
	for p, want := range map[string]bool{
		"a.cue":                        true,
		"cue.mod/module.cue":           true,
		"x/cue.mod/module.cue":         true,
		"cue.mod/gen/k8s.io/api/a.cue": false,
		"cue.mod/pkg/github.com/a.cue": false,
		"x/cue.mod/usr/k8s.io/a.cue":   false,
		"pkg/gen/a.cue":                true,
		"a.json":                       false,
	} {
		if got := (Plugin{}).Claims(&scan.File{Path: p}); got != want {
			t.Errorf("%s: got %v, want %v", p, got, want)
		}
	}
	if (Plugin{}).Class(&scan.File{Path: "x/cue.mod/module.cue"}) != classMod || (Plugin{}).Class(&scan.File{Path: "module.cue"}) != "" {
		t.Error("class")
	}
}

// Verifies: REQ-CUE-008
func TestIslands(t *testing.T) {
	isStd := map[string]bool{}
	for _, e := range (Plugin{}).Ecosystems() {
		isStd[e.ID] = e.Std
	}
	if len(isStd) != 4 || isStd[ecoCUE] || !isStd[ecoStd] || isStd[ecoGo] || !isStd[ecoGoStd] {
		t.Errorf("got %v", isStd)
	}
}

// Every prefix of every fixture file and pathological inputs go through the
// lexer and the readers without panicking, quickly.
//
// Verifies: REQ-CUE-009
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
			readModule(src[:i])
		}
	}
	for _, unit := range []string{"{", "[", "(", "}", "a: ", "a: {", "\"\\(", "#", "##\"", "\"\"\"", "import (", "import ", "@x(", "\n", "a: b: "} {
		src := []byte(strings.Repeat(unit, 200_000/len(unit)))
		start := time.Now()
		extractSource(src)
		readModule(src)
		if d := time.Since(start); d > 2*time.Second {
			t.Errorf("%q x %d: %v", unit, 200_000/len(unit), d)
		}
	}
}

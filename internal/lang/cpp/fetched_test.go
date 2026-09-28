package cpp

import (
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// fetches stands in for the cmake plugin's reader.
type fetches []Fetched

func (f fetches) Fetched([]*scan.File) []Fetched { return f }
func (fetches) FetchIsland() lang.Ecosystem      { return lang.Ecosystem{ID: "fetch", Name: "Fetched"} }

// An include goes to the fetched content declared nearest above it (its names
// folded like the include's), else to the shallowest declared anywhere; a package a
// manifest declares comes first.
//
// Verifies: REQ-CPP-017
func TestFetchedContent(t *testing.T) {
	pkg := func(name string) lang.Target { return lang.Target{Ecosystem: "fetch", Package: name} }
	reader := fetches{
		{Dir: "b/c", Names: []string{"lib-x"}, Target: pkg("bc")},
		{Dir: "z", Names: []string{"Only", ""}, Target: pkg("z")},
		{Dir: "a", Names: []string{"Lib_X"}, Target: pkg("a")},
		{Dir: "z/y", Names: []string{"only"}, Target: pkg("zy")},
		{Dir: ".", Names: []string{"LIB-X", "fmt"}, Target: pkg("root")},
	}
	root := langtest.Write(t, map[string]string{
		"vcpkg.json":  `{"dependencies": ["fmt"]}`,
		"a/s.cpp":     "#include <lib_x/h.h>\n",
		"b/c/d/s.cpp": "#include <lib_x/h.h>\n",
		"b/s.cpp":     "#include <lib_x/h.h>\n",
		"top.cpp":     "#include <lib_x/h.h>\n#include <fmt/core.h>\n#include <none/n.h>\n",
		"q/s.cpp":     "#include <only/o.h>\n",
		"z/y/x/s.cpp": "#include <only/o.h>\n",
	})
	res := langtest.Analyze(t, Plugin{Fetches: reader}, root)
	for file, want := range map[string]map[string]lang.Target{
		"a/s.cpp":     {"#include <lib_x/h.h>": pkg("a")},
		"b/c/d/s.cpp": {"#include <lib_x/h.h>": pkg("bc")},
		"b/s.cpp":     {"#include <lib_x/h.h>": pkg("root")},
		"top.cpp": {
			"#include <lib_x/h.h>":  pkg("root"),
			"#include <fmt/core.h>": {Ecosystem: ecoVcpkg, Package: "fmt", Floating: true},
			"#include <none/n.h>":   {Ecosystem: ecoExternal, Package: "none", Unresolved: true},
		},
		"q/s.cpp":     {"#include <only/o.h>": pkg("z")},
		"z/y/x/s.cpp": {"#include <only/o.h>": pkg("zy")},
	} {
		langtest.CheckImports(t, res[file], want)
	}
	// Nothing fetched, or no reader at all, leaves the library c-external.
	for _, p := range []Plugin{{Fetches: fetches{}}, {}} {
		langtest.CheckImports(t, langtest.Analyze(t, p, root)["q/s.cpp"], map[string]lang.Target{
			"#include <only/o.h>": {Ecosystem: ecoExternal, Package: "only", Unresolved: true},
		})
	}
}

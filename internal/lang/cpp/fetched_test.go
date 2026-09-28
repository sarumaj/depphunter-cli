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
	packageName := func(name string) lang.Target { return lang.Target{Ecosystem: "fetch", Package: name} }
	reader := fetches{
		{Directory: "b/c", Names: []string{"lib-x"}, Target: packageName("bc")},
		{Directory: "z", Names: []string{"Only", ""}, Target: packageName("z")},
		{Directory: "a", Names: []string{"Lib_X"}, Target: packageName("a")},
		{Directory: "z/y", Names: []string{"only"}, Target: packageName("zy")},
		{Directory: ".", Names: []string{"LIB-X", "fmt"}, Target: packageName("root")},
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
	results := langtest.Analyze(t, Plugin{Fetches: reader}, root)
	for file, want := range map[string]map[string]lang.Target{
		"a/s.cpp":     {"#include <lib_x/h.h>": packageName("a")},
		"b/c/d/s.cpp": {"#include <lib_x/h.h>": packageName("bc")},
		"b/s.cpp":     {"#include <lib_x/h.h>": packageName("root")},
		"top.cpp": {
			"#include <lib_x/h.h>":  packageName("root"),
			"#include <fmt/core.h>": {Ecosystem: ecosystemVcpkg, Package: "fmt", Floating: true},
			"#include <none/n.h>":   {Ecosystem: ecosystemExternal, Package: "none", Unresolved: true},
		},
		"q/s.cpp":     {"#include <only/o.h>": packageName("z")},
		"z/y/x/s.cpp": {"#include <only/o.h>": packageName("zy")},
	} {
		langtest.CheckImports(t, results[file], want)
	}
	// Nothing fetched, or no reader at all, leaves the library c-external.
	for _, p := range []Plugin{{Fetches: fetches{}}, {}} {
		langtest.CheckImports(t, langtest.Analyze(t, p, root)["q/s.cpp"], map[string]lang.Target{
			"#include <only/o.h>": {Ecosystem: ecosystemExternal, Package: "only", Unresolved: true},
		})
	}
}

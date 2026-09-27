package analyze

import (
	"context"
	"strings"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/graph"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/beam"
	"github.com/sarumaj/depphunter-cli/internal/lang/gleam"
)

// Gleam packages are Hex packages: gleam_stdlib locked by an Elixir project's
// mix.lock and by a Gleam package's manifest.toml, and imported as gleam/list, is
// one node of the one Hex island; so is :gleam@list called from Elixir.
//
// Verifies: REQ-GLEAM-009
func TestOneHexNodeAcrossElixirAndGleam(t *testing.T) {
	root := t.TempDir()
	writeProject(t, root, map[string]string{
		"mix.exs": `defmodule App.MixProject do
  use Mix.Project
  def project, do: [app: :app, deps: [{:gleam_stdlib, "~> 0.40"}]]
end
`,
		"mix.lock": `%{
  "gleam_stdlib": {:hex, :gleam_stdlib, "0.40.0", "86606b75a600bbd05e539eb59fabc6e307eeea7b1e5865afb6d980a93bcb2181", [:gleam], [], "hexpm", "3fdc2d5ee2d4a2ff0a6c4e1f0b5a8b8b8f2c4c9fd6d6e5b2c5f6a0b1d3e4f5a6"},
}
`,
		"lib/app.ex":       "defmodule App do\n  def f, do: :gleam@list.reverse([1])\nend\n",
		"gleam/gleam.toml": "name = \"core\"\n\n[dependencies]\ngleam_stdlib = \">= 0.34.0 and < 2.0.0\"\n",
		"gleam/manifest.toml": `packages = [
  { name = "gleam_stdlib", version = "0.40.0", build_tools = ["gleam"], requirements = [], otp_app = "gleam_stdlib", source = "hex", outer_checksum = "86606B75A600BBD05E539EB59FABC6E307EEEA7B1E5865AFB6D980A93BCB2181" },
]

[requirements]
gleam_stdlib = { version = ">= 0.34.0 and < 2.0.0" }
`,
		"gleam/src/core.gleam":      "import gleam/list\n\npub fn f() { list.reverse([1]) }\n",
		"gleam/src/core/util.gleam": "pub fn x() { 1 }\n",
		"gleam/src/core_ffi.erl":    "-module(core_ffi).\n-export([f/0]).\nf() -> core@util:x().\n",
	})
	g, _, err := Run(context.Background(), root, Options{Plugins: []lang.Plugin{beam.Plugin{}, gleam.Plugin{}}})
	if err != nil {
		t.Fatal(err)
	}
	var hex []string
	for _, n := range g.Nodes {
		if n.Kind == graph.KindPackage && n.Parent == graph.EcosystemID("hex") {
			hex = append(hex, n.Name)
		}
	}
	if len(hex) != 1 || hex[0] != "gleam_stdlib" {
		t.Fatalf("hex packages %v, want only gleam_stdlib", hex)
	}
	id := graph.PackageID("hex", "gleam_stdlib")
	from := map[string]bool{}
	for _, e := range g.Edges {
		if e.To == id {
			from[e.From] = true
		}
	}
	// :gleam@list in Elixir is the compiled gleam/list module of gleam_stdlib.
	for _, f := range []string{"mix.exs", "lib/app.ex", "gleam/gleam.toml", "gleam/manifest.toml", "gleam/src/core.gleam"} {
		if !from[graph.FileID(f)] {
			t.Errorf("%s has no edge to %s (edges from %v)", f, id, strings.Join(keys(from), ", "))
		}
	}
	// An Erlang FFI file calling a Gleam module of its package: core@util.
	local := false
	for _, e := range g.Edges {
		local = local || e.From == graph.FileID("gleam/src/core_ffi.erl") && e.To == graph.FileID("gleam/src/core/util.gleam")
	}
	if !local {
		t.Error("core_ffi.erl has no edge to core/util.gleam")
	}
	if n := byID(g)[id]; n.Version != "0.40.0" || n.Floating {
		t.Errorf("version %q floating %v, want 0.40.0 not floating", n.Version, n.Floating)
	}
}

package beam

import (
	"slices"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// rebar.lock pins without edges and says so; mix.lock, which records them, does
// not, nor does a rebar.lock beside one; a lock the scan left out and read from disk is noted as such.
//
// Verifies: REQ-BEAM-010, REQ-TRC-017
func TestLockNotes(t *testing.T) {
	root := langtest.Write(t, map[string]string{
		"erl/rebar.config": "{deps, [cowboy]}.\n",
		"erl/rebar.lock":   "{\"1.2.0\",\n[{<<\"cowboy\">>,{pkg,<<\"cowboy\">>,<<\"2.10.0\">>},0}]}.\n",
		"ex/mix.exs":       "defmodule Ex.MixProject do\n  use Mix.Project\n  def project, do: [app: :ex, deps: [{:jason, \"~> 1.4\"}]]\nend\n",
		"ex/mix.lock":      "%{\n  \"jason\": {:hex, :jason, \"1.4.1\", \"h\", [:mix], [], \"hexpm\", \"h2\"},\n}\n",
		// Both beside each other: mix.lock's edges answer, nothing to say.
		"both/mix.exs":      "defmodule Both.MixProject do\n  use Mix.Project\n  def project, do: [app: :both, deps: [{:jason, \"~> 1.4\"}]]\nend\n",
		"both/mix.lock":     "%{\n  \"jason\": {:hex, :jason, \"1.4.1\", \"h\", [:mix], [], \"hexpm\", \"h2\"},\n}\n",
		"both/rebar.config": "{deps, [jason]}.\n",
		"both/rebar.lock":   "{\"1.2.0\",\n[{<<\"jason\">>,{pkg,<<\"jason\">>,<<\"1.4.1\">>},0}]}.\n",
	})
	files := langtest.Files(t, root)
	if got, want := langtest.Notes(newResolver(root, files)), []string{"erl/rebar.lock lock-flat"}; !slices.Equal(got, want) {
		t.Errorf("notes %q, want %q", got, want)
	}
	got := langtest.Notes(newResolver(root, langtest.Without(files, "erl/rebar.lock", "ex/mix.lock")))
	want := []string{"erl/rebar.lock lock-flat", "erl/rebar.lock lock-ignored", "ex/mix.lock lock-ignored"}
	if !slices.Equal(got, want) {
		t.Errorf("ignored: notes %q, want %q", got, want)
	}
}

package beam

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// A package of a private Hex organization names its repository as the target's
// Registry: organization: and repo: in mix.exs, the repository mix.lock records for
// the package and for each of its requirements. hex.pm's own ("hexpm") is none.
//
// Verifies: REQ-BEAM-013
func TestHexOrganizationPackages(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{
		"mix.exs": `defmodule Shop.MixProject do
  use Mix.Project
  def project, do: [app: :shop, deps: deps()]
  defp deps do
    [
      {:billing, "~> 1.0", organization: "acme"},
      {:ledger, "~> 2.0", repo: "hexpm:acme"},
      {:vault, "~> 0.1", repo: "hexpm:acme"},
      {:mini, "~> 0.1", repo: "mini_repo"},
      {:plug, "~> 1.16", repo: "hexpm"},
      {:jason, "~> 1.4"}
    ]
  end
end
`,
		"mix.lock": `%{
  "billing": {:hex, :billing, "1.2.0", "h", [:mix], [{:jason, "~> 1.0", [hex: :jason, repo: "hexpm", optional: false]}, {:ledger, "~> 2.0", [hex: :ledger, repo: "hexpm:acme", optional: false]}, {:audit, "~> 0.3", [hex: :audit, repo: "hexpm:acme", optional: false]}], "hexpm:acme", "h2"},
  "ledger": {:hex, :ledger, "2.0.1", "h", [:mix], [], "hexpm:acme", "h2"},
  "jason": {:hex, :jason, "1.4.1", "h", [:mix], [], "hexpm", "h2"},
}
`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	r := newResolver(root, langtest.Files(t, root))
	for app, want := range map[string]lang.Target{
		"billing": {Ecosystem: ecoHex, Package: "billing", Version: "1.2.0", Requested: "~> 1.0", Pinned: true, Registry: "hexpm:acme"},
		"ledger":  {Ecosystem: ecoHex, Package: "ledger", Version: "2.0.1", Requested: "~> 2.0", Pinned: true, Registry: "hexpm:acme"},
		"vault":   {Ecosystem: ecoHex, Package: "vault", Version: "~> 0.1", Registry: "hexpm:acme"},
		"mini":    {Ecosystem: ecoHex, Package: "mini", Version: "~> 0.1", Registry: "mini_repo"},
		"plug":    {Ecosystem: ecoHex, Package: "plug", Version: "~> 1.16"},
		"jason":   {Ecosystem: ecoHex, Package: "jason", Version: "1.4.1", Requested: "~> 1.4", Pinned: true},
	} {
		if got := r.dependency("mix.exs", app); got != want {
			t.Errorf("%s: %+v\nwant %+v", app, got, want)
		}
	}
	got := r.Dependencies(lang.Target{Ecosystem: ecoHex, Package: "billing", Version: "1.2.0", Registry: "hexpm:acme"})
	want := []lang.Target{
		{Ecosystem: ecoHex, Package: "audit", Version: "~> 0.3", Registry: "hexpm:acme"},
		{Ecosystem: ecoHex, Package: "jason", Version: "1.4.1", Requested: "~> 1.0", Pinned: true},
		{Ecosystem: ecoHex, Package: "ledger", Version: "2.0.1", Requested: "~> 2.0", Pinned: true, Registry: "hexpm:acme"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("billing: %+v\nwant %+v", got, want)
	}
}

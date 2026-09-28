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

// rebar3 records no repository for a package, in rebar.config's deps or in
// rebar.lock, and asks the repositories its configuration names in order, then
// hex.pm's: a rebar3 project's Hex packages, locked or not, name that order as
// their Registry - the project's {hex, [{repos, ...}]} and "*" for the machine's
// and hex.pm's after them, or the replacing list alone. An umbrella's
// applications take the root's; a Mix project's packages are Mix's, even beside a
// rebar.config or inside a rebar3 project's directory (organization: and mix.lock
// decide there); git and path dependencies name none.
//
// Verifies: REQ-BEAM-013
func TestRebar3OrganizationRepositories(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{
		"shop/rebar.config": `{deps, [billing, {jsx, "~> 3.1"}, {meck, {git, "https://github.com/eproxus/meck.git", {tag, "0.9.2"}}}]}.
{hex, [{repos, [#{name => <<"hexpm:acme">>, repo_key => <<"k">>}]}, {doc, #{provider => edoc}}]}.
`,
		"shop/rebar.lock": `{"1.2.0",
[{<<"billing">>,{pkg,<<"billing">>,<<"1.2.0">>},0},
 {<<"ledger">>,{pkg,<<"ledger">>,<<"2.0.1">>},1}]}.
[{pkg_hash,[{<<"billing">>, <<"AB">>},{<<"ledger">>, <<"CD">>}]}].
`,
		"shop/apps/api/rebar.config":    `{deps, [{cowboy, "2.10.0"}]}.` + "\n",
		"shop/apps/api/src/api.app.src": `{application, api, [{applications, [kernel, cowboy]}]}.` + "\n",
		"shop/tools/mix.exs": `defmodule Tools.MixProject do
  use Mix.Project
  def project, do: [app: :tools, deps: [{:credo, "~> 1.7"}]]
end
`,
		"replaced/rebar.config": `{deps, [billing]}.
{hex, [{repos, replace, [#{name => <<"hexpm:acme">>}, #{name => <<"hexpm">>}]}]}.
`,
		"hexonly/rebar.config": `{deps, [jsx]}.
{hex, [{repos, replace, [#{name => <<"hexpm">>}]}]}.
`,
		"mixed/mix.exs": `defmodule Mixed.MixProject do
  use Mix.Project
  def project, do: [app: :mixed, deps: [{:jsx, "~> 3.1"}]]
end
`,
		"mixed/rebar.config": `{deps, [jsx]}.
{hex, [{repos, [#{name => <<"hexpm:acme">>}]}]}.
`,
	} {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	r := newResolver(root, langtest.Files(t, root))
	for _, tc := range []struct {
		file, app string
		want      lang.Target
	}{
		{"shop/rebar.config", "billing", lang.Target{Ecosystem: ecoHex, Package: "billing", Version: "1.2.0", Pinned: true, Registry: "hexpm:acme,*"}},
		{"shop/rebar.config", "ledger", lang.Target{Ecosystem: ecoHex, Package: "ledger", Version: "2.0.1", Pinned: true, Registry: "hexpm:acme,*"}},
		{"shop/rebar.config", "jsx", lang.Target{Ecosystem: ecoHex, Package: "jsx", Version: "~> 3.1", Registry: "hexpm:acme,*"}},
		{"shop/rebar.config", "meck", lang.Target{Ecosystem: ecoHex, Package: "meck", Version: "0.9.2", Floating: true, Origin: "https://github.com/eproxus/meck.git"}},
		{"shop/apps/api/rebar.config", "cowboy", lang.Target{Ecosystem: ecoHex, Package: "cowboy", Version: "2.10.0", Pinned: true, Registry: "hexpm:acme,*"}},
		{"replaced/rebar.config", "billing", lang.Target{Ecosystem: ecoHex, Package: "billing", Floating: true, Registry: "hexpm:acme,hexpm"}},
		{"hexonly/rebar.config", "jsx", lang.Target{Ecosystem: ecoHex, Package: "jsx", Floating: true}},
		{"shop/tools/mix.exs", "credo", lang.Target{Ecosystem: ecoHex, Package: "credo", Version: "~> 1.7"}},
		{"mixed/mix.exs", "jsx", lang.Target{Ecosystem: ecoHex, Package: "jsx", Version: "~> 3.1"}},
	} {
		if got := r.dependency(tc.file, tc.app); got != tc.want {
			t.Errorf("%s %s: %+v\nwant %+v", tc.file, tc.app, got, tc.want)
		}
	}
	for src, want := range map[string]string{
		`{deps, []}.`: "*",
		`{hex, [{repos, [#{name => <<"hexpm:a">>}]}, {repos, [#{name => "hexpm:b"}]}]}.`: "hexpm:a,hexpm:b,*",
		`{hex, [{repos, [#{name => <<"hexpm">>}]}]}.`:                                    "hexpm,*",
		`{hex, [{repos, replace, []}]}.`:                                                 "-",
		`{hex, [{repos, [#{name => <<"a,b">>}]}]}.`:                                      "*",
	} {
		if got := rebarRegistry(erlForms([]byte(src))); got != want {
			t.Errorf("%s: %q, want %q", src, got, want)
		}
	}
}

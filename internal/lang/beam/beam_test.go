package beam

import (
	"reflect"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// An umbrella (apps/shop, apps/shop_web) sharing the root mix.lock, a path
// dependency (libs/local_lib), a package fetched into deps/ whose module is not
// named after it (elixir_uuid's UUID), and a rebar3 application (erl/) with
// rebar.lock and a _build module file: nested and multi aliases, __MODULE__,
// aliases a used module's quote injects, a Phoenix router scope, protocols and
// their implementations, Erlang calls from Elixir and back, includes, and manifest
// dependencies and applications as imports. The rebar3 application's Hex packages
// are asked of the repositories rebar3 would ask (Registry "*": none of its own).
//
// Verifies: REQ-BEAM-001, REQ-BEAM-002, REQ-BEAM-003, REQ-BEAM-004, REQ-BEAM-005
// Verifies: REQ-BEAM-006, REQ-BEAM-007, REQ-BEAM-008, REQ-BEAM-009, REQ-BEAM-010
// Verifies: REQ-BEAM-011, REQ-BEAM-013
func TestUmbrellaAndRebarApp(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	imports := map[string]map[string]lang.Target{
		"apps/shop/lib/shop.ex": {
			"String":    {Ecosystem: ecosystemElixir, Package: "String"},
			"GenServer": {Ecosystem: ecosystemElixir, Package: "GenServer"},
		},
		"apps/shop/lib/shop/cart.ex": {
			"use GenServer":            {Ecosystem: ecosystemElixir, Package: "GenServer"},
			"require Logger":           {Ecosystem: ecosystemElixir, Package: "Logger"},
			"alias Ecto.Changeset":     {Ecosystem: ecosystemHex, Package: "ecto", Version: "3.11.2", Pinned: true},
			"alias Shop.Item":          {Local: "apps/shop/lib/shop/item.ex"},
			"alias Shop.Pricing.Rules": {Local: "apps/shop/lib/shop/pricing.ex"},
			"alias Shop.Cart.Line":     {Local: "apps/shop/lib/shop/pricing.ex"},
			"import Ecto.Query":        {Ecosystem: ecosystemHex, Package: "ecto", Version: "3.11.2", Pinned: true},
			":gen_statem":              {Ecosystem: ecosystemOTP, Package: "gen_statem"},
			"Jason":                    {Ecosystem: ecosystemHex, Package: "jason", Version: "1.4.1", Pinned: true},
			":ets":                     {Ecosystem: ecosystemOTP, Package: "ets"},
			"UUID":                     {Ecosystem: ecosystemHex, Package: "elixir_uuid", Version: "1.2.1", Requested: "~> 1.2", Pinned: true},
			"NimbleCSV.RFC4180":        {Ecosystem: ecosystemHex, Package: "nimble_csv", Version: "1.2.0", Pinned: true},
			"Ecto.Adapters.SQL":        {Ecosystem: ecosystemHex, Package: "ecto_sql", Version: "3.11.3", Requested: "~> 3.10", Pinned: true},
			"Shop.Repo":                {Local: "apps/shop/lib/shop.ex"},
			"LocalLib":                 {Local: "libs/local_lib/lib/local_lib.ex"},
			"Money":                    {Ecosystem: ecosystemHex, Package: "money", Version: "v1.12.0", Floating: true, Origin: "https://github.com/elixirmoney/money.git"},
			"Unknown.Thing":            {Ecosystem: ecosystemHex, Package: "unknown", Unresolved: true},
			":crypto":                  {Ecosystem: ecosystemOTP, Package: "crypto"},
		},
		"apps/shop/lib/shop/item.ex": {
			"use Shop.Schema": {Local: "apps/shop/lib/shop/schema.ex"},
			"Cart":            {Local: "apps/shop/lib/shop/cart.ex"},
			"Multi":           {Ecosystem: ecosystemHex, Package: "ecto", Version: "3.11.2", Pinned: true},
			"Shop.Cart.Line":  {Local: "apps/shop/lib/shop/pricing.ex"},
		},
		"apps/shop/lib/shop/pricing.ex": {},
		"apps/shop/lib/shop/schema.ex": {
			"use Ecto.Schema":  {Ecosystem: ecosystemHex, Package: "ecto", Version: "3.11.2", Pinned: true},
			"alias Ecto.Multi": {Ecosystem: ecosystemHex, Package: "ecto", Version: "3.11.2", Pinned: true},
			"alias Shop.Cart":  {Local: "apps/shop/lib/shop/cart.ex"},
			"alias Shop.Item":  {Local: "apps/shop/lib/shop/item.ex"},
		},
		"apps/shop/mix.exs": {
			"use Mix.Project":                                           {Ecosystem: ecosystemElixir, Package: "Mix"},
			"{:ecto_sql, \"~> 3.10\"}":                                  {Ecosystem: ecosystemHex, Package: "ecto_sql", Version: "3.11.3", Requested: "~> 3.10", Pinned: true},
			"{:jason, \"== 1.4.1\"}":                                    {Ecosystem: ecosystemHex, Package: "jason", Version: "1.4.1", Pinned: true},
			"{:elixir_uuid, \"~> 1.2\"}":                                {Ecosystem: ecosystemHex, Package: "elixir_uuid", Version: "1.2.1", Requested: "~> 1.2", Pinned: true},
			"{:nimble_csv, \"1.2.0\"}":                                  {Ecosystem: ecosystemHex, Package: "nimble_csv", Version: "1.2.0", Pinned: true},
			"{:local_lib, path: \"../../libs/local_lib\"}":              {Local: "libs/local_lib/mix.exs"},
			"{:money, github: \"elixirmoney/money\", tag: \"v1.12.0\"}": {Ecosystem: ecosystemHex, Package: "money", Version: "v1.12.0", Floating: true, Origin: "https://github.com/elixirmoney/money.git"},
		},
		"apps/shop/test/cart_test.exs": {
			"use ExUnit.Case":      {Ecosystem: ecosystemElixir, Package: "ExUnit"},
			"use ExUnitProperties": {Ecosystem: ecosystemHex, Package: "stream_data", Unresolved: true},
			"Shop.Cart":            {Local: "apps/shop/lib/shop/cart.ex"},
		},
		"apps/shop_web/lib/shop_web.ex": {
			"use Phoenix.Controller":       {Ecosystem: ecosystemHex, Package: "phoenix", Version: "8d2f6a5fbb2b7bd4bc7f4b0c3a5e16d4ac8a7a12", Pinned: true, Origin: "https://github.com/phoenixframework/phoenix.git"},
			"alias ShopWeb.Router.Helpers": {Local: "apps/shop_web/lib/shop_web/router.ex"},
		},
		"apps/shop_web/lib/shop_web/cart_live.ex": {
			"use Phoenix.LiveView": {Ecosystem: ecosystemHex, Package: "phoenix_live_view", Version: "0.20.14", Requested: "~> 0.20.0", Pinned: true},
			":legacy_parser":       {Local: "apps/shop_web/src/legacy_parser.erl"},
		},
		"apps/shop_web/lib/shop_web/controllers/page_controller.ex": {
			"use ShopWeb": {Local: "apps/shop_web/lib/shop_web.ex"},
			"Plug.Conn":   {Ecosystem: ecosystemHex, Package: "plug", Version: "1.16.0", Pinned: true},
			"Routes":      {Local: "apps/shop_web/lib/shop_web/router.ex"},
		},
		"apps/shop_web/lib/shop_web/router.ex": {
			"use Phoenix.Router":                {Ecosystem: ecosystemHex, Package: "phoenix", Version: "8d2f6a5fbb2b7bd4bc7f4b0c3a5e16d4ac8a7a12", Pinned: true, Origin: "https://github.com/phoenixframework/phoenix.git"},
			"import Phoenix.LiveView.Router":    {Ecosystem: ecosystemHex, Package: "phoenix_live_view", Version: "0.20.14", Requested: "~> 0.20.0", Pinned: true},
			"ShopWeb.PageController":            {Local: "apps/shop_web/lib/shop_web/controllers/page_controller.ex"},
			"ShopWeb.CartLive":                  {Local: "apps/shop_web/lib/shop_web/cart_live.ex"},
			"ShopWeb.Admin.DashboardController": {Local: "apps/shop_web/lib/shop_web.ex"},
		},
		"apps/shop_web/mix.exs": {
			"use Mix.Project": {Ecosystem: ecosystemElixir, Package: "Mix"},
			"{:phoenix, github: \"phoenixframework/phoenix\", branch: \"main\", override: true}": {Ecosystem: ecosystemHex, Package: "phoenix", Version: "8d2f6a5fbb2b7bd4bc7f4b0c3a5e16d4ac8a7a12", Pinned: true, Origin: "https://github.com/phoenixframework/phoenix.git"},
			"{:phoenix_live_view, \"~> 0.20.0\"}":                                                {Ecosystem: ecosystemHex, Package: "phoenix_live_view", Version: "0.20.14", Requested: "~> 0.20.0", Pinned: true},
			"{:shop, in_umbrella: true}":                                                         {Local: "apps/shop/mix.exs"},
		},
		"apps/shop_web/src/legacy_parser.erl": {
			"Elixir.Shop.Cart": {Local: "apps/shop/lib/shop/cart.ex"},
			"lists":            {Ecosystem: ecosystemOTP, Package: "lists"},
		},
		"erl/include/erl_app.hrl": {
			"logger": {Ecosystem: ecosystemOTP, Package: "logger"},
		},
		"erl/rebar.config": {
			"{cowboy, \"2.10.0\"}":                       {Ecosystem: ecosystemHex, Package: "cowboy", Version: "2.10.0", Pinned: true, Registry: "*"},
			"{jsx, \"~> 3.1\"}":                          {Ecosystem: ecosystemHex, Package: "jsx", Version: "3.1.0", Requested: "~> 3.1", Pinned: true, Registry: "*"},
			"erlware_commons":                            {Ecosystem: ecosystemHex, Package: "erlware_commons", Version: "1.7.0", Pinned: true, Registry: "*"},
			"{hackney_fork, \"1.20.1\", {pkg, hackney}}": {Ecosystem: ecosystemHex, Package: "hackney", Version: "1.20.1", Pinned: true, Registry: "*"},
			"{meck, {git, \"https://github.com/eproxus/meck.git\", {ref, \"4ecc1ae9089edc6977e8c8c4cd41081513cc5590\"}}}": {Ecosystem: ecosystemHex, Package: "meck", Version: "4ecc1ae9089edc6977e8c8c4cd41081513cc5590", Pinned: true, Origin: "https://github.com/eproxus/meck.git"},
			"{recon, {git, \"https://github.com/ferd/recon.git\", {branch, \"master\"}}}":                                 {Ecosystem: ecosystemHex, Package: "recon", Version: "master", Floating: true, Origin: "https://github.com/ferd/recon.git"},
			"{proper, \"1.4.0\"}": {Ecosystem: ecosystemHex, Package: "proper", Version: "1.4.0", Pinned: true, Registry: "*"},
		},
		"erl/src/erl_app.app.src": {
			"application kernel":    {Ecosystem: ecosystemOTP, Package: "kernel"},
			"application stdlib":    {Ecosystem: ecosystemOTP, Package: "stdlib"},
			"application crypto":    {Ecosystem: ecosystemOTP, Package: "crypto"},
			"application sasl":      {Ecosystem: ecosystemOTP, Package: "sasl"},
			"application cowboy":    {Ecosystem: ecosystemHex, Package: "cowboy", Version: "2.10.0", Pinned: true, Registry: "*"},
			"application jsx":       {Ecosystem: ecosystemHex, Package: "jsx", Version: "3.1.0", Requested: "~> 3.1", Pinned: true, Registry: "*"},
			"application local_dep": {Ecosystem: ecosystemHex, Package: "local_dep", Unresolved: true},
		},
		"erl/src/erl_app.erl": { // cSpell: words behaviour
			"-behaviour(application)":                         {Ecosystem: ecosystemOTP, Package: "application"},
			"-include(\"erl_app.hrl\")":                       {Local: "erl/include/erl_app.hrl"},
			"-include_lib(\"kernel/include/logger.hrl\")":     {Ecosystem: ecosystemOTP, Package: "kernel"},
			"-include_lib(\"cowlib/include/cow_inline.hrl\")": {Ecosystem: ecosystemHex, Package: "cowlib", Version: "2.12.1", Pinned: true, Registry: "*"},
			"-include_lib(\"erl_app/include/erl_app.hrl\")":   {Local: "erl/include/erl_app.hrl"},
			"-include(\"generated.hrl\")":                     {},
			"cowboy_router":                                   {Ecosystem: ecosystemHex, Package: "cowboy", Version: "2.10.0", Pinned: true, Registry: "*"},
			"cowboy":                                          {Ecosystem: ecosystemHex, Package: "cowboy", Version: "2.10.0", Pinned: true, Registry: "*"},
			"jsx":                                             {Ecosystem: ecosystemHex, Package: "jsx", Version: "3.1.0", Requested: "~> 3.1", Pinned: true, Registry: "*"},
			"ec_file":                                         {Ecosystem: ecosystemHex, Package: "erlware_commons", Version: "1.7.0", Pinned: true, Registry: "*"},
			"crypto":                                          {Ecosystem: ecosystemOTP, Package: "crypto"},
			"erl_worker":                                      {Local: "erl/src/erl_worker.erl"},
			"lists":                                           {Ecosystem: ecosystemOTP, Package: "lists"},
			"Elixir.Jason":                                    {Ecosystem: ecosystemHex, Package: "jason", Unresolved: true},
			"unknown_mod":                                     {Ecosystem: ecosystemHex, Package: "unknown_mod", Unresolved: true},
		},
		"erl/src/erl_handler.erl": {
			"-include(\"erl_app.hrl\")": {Local: "erl/include/erl_app.hrl"},
			"cowboy_req":                {Ecosystem: ecosystemHex, Package: "cowboy", Version: "2.10.0", Pinned: true, Registry: "*"},
		},
		"erl/src/erl_worker.erl": {
			"-behaviour(gen_server)": {Ecosystem: ecosystemOTP, Package: "gen_server"},
			"-import(lists)":         {Ecosystem: ecosystemOTP, Package: "lists"},
		},
		"libs/local_lib/lib/local_lib.ex": {},
		"libs/local_lib/mix.exs": {
			"use Mix.Project": {Ecosystem: ecosystemElixir, Package: "Mix"},
		},
		"mix.exs": {
			"use Mix.Project": {Ecosystem: ecosystemElixir, Package: "Mix"},
			"{:credo, \"~> 1.7\", only: [:dev, :test], runtime: false}": {Ecosystem: ecosystemHex, Package: "credo", Version: "1.7.7", Requested: "~> 1.7", Pinned: true},
		},
	}
	for file, want := range imports {
		t.Run(file, func(t *testing.T) { langtest.CheckImports(t, results[file], want) })
	}
	symbols := map[string]map[string]string{
		"apps/shop/lib/shop.ex": {
			"Shop":             "module",
			"%Shop{}":          "struct",
			"Shop.t":           "type",
			"Shop.start/1":     "function",
			"Shop.start/2":     "function",
			"Shop.secret?/1":   "func",
			"Shop.__using__/1": "macro",
			"Shop.Error":       "module",
			"%Shop.Error{}":    "exception",
			"Shop.fail!/0":     "function",
		},
		"apps/shop/lib/shop/cart.ex": {
			"Shop.Cart":                          "module",
			"Shop.Cart.new/1":                    "function",
			"Shop.Cart.total/2":                  "function",
			"Shop.Cart.Priced":                   "protocol",
			"Shop.Cart.Priced.price/1":           "function",
			"Shop.Cart.Priced.Shop.Item":         "impl",
			"Shop.Cart.Priced.Shop.Item.price/1": "function",
		},
		"apps/shop/lib/shop/item.ex": {
			"Shop.Item":         "module",
			"Shop.Item.price/1": "function",
			"Shop.Item.line/0":  "function",
		},
		"apps/shop/lib/shop/pricing.ex": {
			"Shop.Pricing.Rules":         "module",
			"Shop.Pricing.Rules.apply/3": "function",
			"Shop.Cart.Line":             "module",
			"%Shop.Cart.Line{}":          "struct",
			"Shop.Cart.Line.new/0":       "function",
		},
		"apps/shop/lib/shop/schema.ex": {
			"Shop.Schema":             "module",
			"Shop.Schema.__using__/1": "macro",
		},
		"apps/shop/mix.exs": {
			"Shop.MixProject":           "module",
			"Shop.MixProject.project/0": "function",
			"Shop.MixProject.deps/0":    "func",
		},
		"apps/shop/test/cart_test.exs": {
			"Shop.CartTest": "module",
		},
		"apps/shop_web/lib/shop_web.ex": {
			"ShopWeb":              "module",
			"ShopWeb.controller/0": "function",
			"ShopWeb.__using__/1":  "macro",
		},
		"apps/shop_web/lib/shop_web/cart_live.ex": {
			"ShopWeb.CartLive":          "module",
			"ShopWeb.CartLive.render/1": "function",
			"ShopWeb.CartLive.mount/3":  "function",
		},
		"apps/shop_web/lib/shop_web/controllers/page_controller.ex": {
			"ShopWeb.PageController":         "module",
			"ShopWeb.PageController.index/2": "function",
		},
		"apps/shop_web/lib/shop_web/router.ex": {
			"ShopWeb.Router": "module",
		},
		"apps/shop_web/mix.exs": {
			"ShopWeb.MixProject":           "module",
			"ShopWeb.MixProject.project/0": "function",
			"ShopWeb.MixProject.deps/0":    "func",
		},
		"apps/shop_web/src/legacy_parser.erl": {
			"legacy_parser": "module",
			"parse/1":       "function",
		},
		"erl/include/erl_app.hrl": {
			"#state{}":      "record",
			"?TIMEOUT":      "macro",
			"?LOG":          "macro",
			"port_number()": "type",
		},
		"erl/rebar.config": {},
		"erl/src/erl_app.app.src": {
			"erl_app": "application",
		},
		"erl/src/erl_app.erl": {
			"erl_app":  "module",
			"mode()":   "type",
			"token()":  "type",
			"start/2":  "function",
			"stop/1":   "function",
			"helper/2": "func",
		},
		"erl/src/erl_handler.erl": {
			"erl_handler": "module",
			"init/2":      "function",
		},
		"erl/src/erl_worker.erl": {
			"erl_worker": "module",
			"run/1":      "function",
			"init/1":     "function",
		},
		"libs/local_lib/lib/local_lib.ex": {
			"LocalLib":        "module",
			"LocalLib.help/0": "function",
		},
		"libs/local_lib/mix.exs": {
			"LocalLib.MixProject":           "module",
			"LocalLib.MixProject.project/0": "function",
		},
		"mix.exs": {
			"Shop.Umbrella.MixProject":           "module",
			"Shop.Umbrella.MixProject.project/0": "function",
			"Shop.Umbrella.MixProject.deps/0":    "func",
		},
	}
	for file, want := range symbols {
		t.Run(file+"/symbols", func(t *testing.T) { langtest.CheckSymbols(t, results[file], want) })
	}
	if len(results) != len(imports) {
		t.Errorf("analyzed %d files, want %d", len(results), len(imports))
	}
	for f := range results {
		if _, ok := imports[f]; !ok {
			t.Errorf("unexpected file %s", f)
		}
	}
}

// mix.lock lists each Hex package's own requirements: they are the package's
// dependencies, as the lock resolved them (the requirement kept as requested), an
// optional one only when the lock has it; a git package lists none.
//
// Verifies: REQ-BEAM-010, REQ-SUP-011
func TestMixLockDependencies(t *testing.T) {
	r := newResolver("testdata/repo", langtest.Files(t, "testdata/repo"))
	got := r.Dependencies(lang.Target{Ecosystem: ecosystemHex, Package: "ecto", Version: "3.11.2", Pinned: true})
	want := []lang.Target{
		{Ecosystem: ecosystemHex, Package: "decimal", Version: "2.1.1", Requested: "~> 2.0", Pinned: true},
		{Ecosystem: ecosystemHex, Package: "jason", Version: "1.4.1", Requested: "~> 1.0", Pinned: true},
		{Ecosystem: ecosystemHex, Package: "telemetry", Version: "1.2.1", Requested: "~> 0.4 or ~> 1.0", Pinned: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ecto: got %+v\nwant %+v", got, want)
	}
	// db_connection is not in the lock: its requirement is all that is known.
	got = r.Dependencies(lang.Target{Ecosystem: ecosystemHex, Package: "ecto_sql", Version: "3.11.3"})
	if len(got) != 3 || got[0] != (lang.Target{Ecosystem: ecosystemHex, Package: "db_connection", Version: "~> 2.4.1 or ~> 2.5"}) {
		t.Errorf("ecto_sql: got %+v", got)
	}
	if got := r.Dependencies(lang.Target{Ecosystem: ecosystemHex, Package: "phoenix"}); got != nil {
		t.Errorf("git package: got %+v, want none", got)
	}
	if got := r.Dependencies(lang.Target{Ecosystem: ecosystemHex, Package: "cowboy", Version: "2.10.0"}); got != nil {
		t.Errorf("rebar.lock records no edges: got %+v", got)
	}
	if got := r.Dependencies(lang.Target{Ecosystem: ecosystemOTP, Package: "crypto"}); got != nil {
		t.Errorf("OTP: got %+v", got)
	}
}

// Verifies: REQ-BEAM-010
func TestReadLocks(t *testing.T) {
	mix := readMixLock([]byte(`%{
  "plug": {:hex, :plug, "1.16.0", "abc", [:mix], [{:mime, "~> 2.0", [hex: :mime, repo: "hexpm", optional: false]}], "hexpm", "def"},
  "my_fork": {:hex, :jason, "1.4.1", "abc", [:mix], [], "hexpm", "def"},
  "dep": {:git, "https://github.com/acme/dep.git", "8d2f6a5fbb2b7bd4bc7f4b0c3a5e16d4ac8a7a12", [tag: "v1"]},
  "local": {:path, "../local", []},
}`))
	if l := mix["plug"]; l == nil || l.version != "1.16.0" || len(l.dependencies) != 1 || l.dependencies[0] != (lockedDependency{app: "mime", packageName: "mime", requirement: "~> 2.0"}) {
		t.Errorf("plug: %+v", l)
	}
	if l := mix["my_fork"]; l == nil || l.packageName != "jason" {
		t.Errorf("an app under another package name: %+v", l)
	}
	if l := mix["dep"]; l == nil || l.git != "https://github.com/acme/dep.git" || l.reference != "8d2f6a5fbb2b7bd4bc7f4b0c3a5e16d4ac8a7a12" {
		t.Errorf("git: %+v", l)
	}
	if _, ok := mix["local"]; ok {
		t.Error("an unknown source was read")
	}
	// The older format: a bare list, and "name" => entries in mix.lock.
	old := readRebarLock([]byte(`[{<<"jsx">>,{pkg,<<"jsx">>,<<"2.8.0">>},0},{<<"idna">>,{pkg,<<"idna">>,<<"6.0.0">>},1}].`))
	if old["jsx"] == nil || old["jsx"].version != "2.8.0" || old["idna"] == nil || old["idna"].level != 1 {
		t.Errorf("rebar.lock v0: %+v", old)
	}
	if l := readMixLock([]byte(`%{"x" => {:hex, :x, "0.1.0", "h", [:mix], [], "hexpm"}}`))["x"]; l == nil || l.version != "0.1.0" {
		t.Errorf("=> keys: %+v", l)
	}
}

// Verifies: REQ-BEAM-011
func TestHexPinned(t *testing.T) {
	for requirement, want := range map[string]string{
		"1.2.3": "1.2.3", "== 1.2.3": "1.2.3", "==1.2.3": "1.2.3", "1.2.3-rc.1": "1.2.3-rc.1",
		"~> 1.2": "", ">= 1.0.0": "", "~> 1.0 or ~> 2.0": "", "": "", ">= 1.0.0 and < 2.0.0": "",
	} {
		v, pinned := hexPinned(requirement)
		if pinned != (want != "") || pinned && v != want {
			t.Errorf("%q: got %q %v, want %q", requirement, v, pinned, want)
		}
	}
}

// The lexers keep strings, sigils, heredocs, character literals, comments and
// quoted atoms from being read as code, and tell keyword keys and atoms apart.
//
// Verifies: REQ-BEAM-002, REQ-BEAM-004
func TestLexers(t *testing.T) {
	extraction := extractElixir([]byte(`defmodule A do
  # Comment.Module.call()
  @doc ~S"""
  Doc.Module.call()
  """
  def f(x), do: {?", ~r/Regex.Module/i, ~w(Words.Module)a, 'Char.List', :"Quoted.Atom", "#{Interp.Module.x()} \" Esc.Module"}
  def g(x), do: [Keyword: 1, key: Real.Module.x(), other: :atom]
  def h(x) when x::y, do: x
  def unquote(name)(), do: nil
  def left <> right, do: nil
end
`))
	var specs []string
	for _, rawImport := range extraction.Imports {
		specs = append(specs, rawImport.Spec)
	}
	if !reflect.DeepEqual(specs, []string{"Real.Module"}) {
		t.Errorf("Elixir imports: %q", specs)
	}
	var names []string
	for _, s := range extraction.Symbols {
		names = append(names, s.Name)
	}
	if want := []string{"A", "A.f/1", "A.g/1", "A.h/1"}; !reflect.DeepEqual(names, want) {
		t.Errorf("Elixir symbols: %q, want %q", names, want)
	}
	erl := extractErlang([]byte(`-module(m).
%% comment:call()
f() -> "string:call()", 'quoted atom':x(), $:, <<"bin">>, X = 1.0e3, Y:z(), ?M:w(), 16#FF.
`))
	specs = nil
	for _, rawImport := range erl.Imports {
		specs = append(specs, rawImport.Spec)
	}
	if !reflect.DeepEqual(specs, []string{"quoted atom"}) {
		t.Errorf("Erlang imports: %q", specs)
	}
}

// Mix's and rebar3's build and dependency directories are not the project's.
//
// Verifies: REQ-BEAM-001
func TestClaims(t *testing.T) {
	for p, want := range map[string]bool{
		"lib/a.ex": true, "test/a_test.exs": true, "mix.exs": true, "src/a.erl": true, "include/a.hrl": true,
		"src/a.app.src": true, "rebar.config": true, "mix.lock": false, "rebar.lock": false,
		"deps/plug/lib/plug.ex": false, "_build/dev/lib/a/ebin/a.app": false, "apps/x/deps/y/src/y.erl": false,
		"sys.config": false, "lib/a.eex": false, "lib/app/deps/helper.ex": true,
	} {
		if got := (Plugin{}).Claims(&scan.File{Path: p}); got != want {
			t.Errorf("%s: got %v", p, got)
		}
	}
}

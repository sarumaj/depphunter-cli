package crystal

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// The fixture is a shard "shop" with src/ and spec/, a shard.yml declaring
// shards from GitHub, GitLab, a private git server and a path in the repository
// (libs/widgets) by version range, exact version, branch, tag and commit, a
// shard.lock (version 2.0) and a shard.override.yml. lib/ holds what shards
// installed: kemal with its shard.yml, and exception_page, a dependency of kemal
// only the lock names. tools/lib/ is a lib/ directory without a shard.yml beside
// it, so it is source.

var (
	kemal     = lang.Target{Ecosystem: ecosystemShards, Package: "kemal", Version: "1.4.0", Requested: "~> 1.4", Pinned: true}
	database  = lang.Target{Ecosystem: ecosystemShards, Package: "db", Version: "0.13.1", Pinned: true}
	sqlite    = lang.Target{Ecosystem: ecosystemShards, Package: "crystal-sqlite3", Version: "0.21.0", Pinned: true}
	exception = lang.Target{Ecosystem: ecosystemShards, Package: "exception_page", Version: "0.4.1", Pinned: true}
	spectator = lang.Target{Ecosystem: ecosystemShards, Package: "spectator", Version: "0.12.1+git.commit.0a1b2c3d4e5f60718293a4b5c6d7e8f901234567",
		Requested: "~> 0.12", Pinned: true}
	markd    = lang.Target{Ecosystem: ecosystemShards, Package: "markd", Version: "5e5a4a4f7c3e8f3d0f9b7e2a3c1d4b6a8e9f0a1b", Pinned: true}
	radix    = lang.Target{Ecosystem: ecosystemShards, Package: "radix", Version: "9c0ffee9c0ffee9c0ffee9c0ffee9c0ffee9c0ff", Pinned: true}
	crinja   = lang.Target{Ecosystem: ecosystemShards, Package: "crinja", Version: "v0.8.1"}
	internal = lang.Target{Ecosystem: ecosystemShards, Package: "internal", Version: ">= 1.0", Floating: true,
		Origin: "https://git.acme.internal/shop/internal.git"}
)

func std(name string) lang.Target { return lang.Target{Ecosystem: ecosystemStd, Package: name} }

// The machine's CRYSTAL_PATH must not change what the fixtures resolve to.
func TestMain(m *testing.M) {
	os.Unsetenv("CRYSTAL_PATH")
	os.Exit(m.Run())
}

func noEnvironment(string) string { return "" }

// Verifies: REQ-CRYSTAL-002, REQ-CRYSTAL-004, REQ-CRYSTAL-006, REQ-CRYSTAL-007, REQ-CRYSTAL-011
func TestRequires(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckImports(t, results["src/shop.cr"], map[string]lang.Target{
		// ./shop/* is the .cr files of src/shop; ./shop/models/** those below it too.
		"./shop/cart.cr":              {Local: "src/shop/cart.cr"},
		"./shop/version.cr":           {Local: "src/shop/version.cr"},
		"./shop/models/user.cr":       {Local: "src/shop/models/user.cr"},
		"./shop/models/admin/role.cr": {Local: "src/shop/models/admin/role.cr"},
		"./missing":                   {},
		"./shop/windows":              {}, // in a {% if %} branch, missing
		"json":                        std("json"),
		"http/client":                 std("http"),
		"digest/sha256":               std("digest"),
		"c/stdio":                     std("lib_c"),
		"kemal":                       kemal, // installed in lib/
		"kemal/cli":                   kemal,
		"exception_page":              exception, // installed, only the lock names it
		"db":                          database,  // an exact version the lock pins
		"sqlite3":                     sqlite,    // declared as crystal-sqlite3
		"markd":                       markd,     // a commit
		"radix":                       radix,     // shard.override.yml's commit
		"crinja":                      crinja,    // a tag: shown, neither pinned nor floating
		"internal/client":             internal,  // a range on a private git server
		"widgets":                     {Local: "libs/widgets/src/widgets.cr"},
		"widgets/button":              {Local: "libs/widgets/src/widgets/button.cr"},
		"shop/version":                {Local: "src/shop/version.cr"}, // the shard itself
		"nothere/thing":               {Ecosystem: ecosystemShards, Package: "nothere", Unresolved: true},
	})
	langtest.CheckImports(t, results["spec/shop_spec.cr"], map[string]lang.Target{
		"spec":        std("spec"),
		"spectator":   spectator,
		"../src/shop": {Local: "src/shop.cr"},
		"shop/cart":   {Local: "src/shop/cart.cr"},
	})
	langtest.CheckImports(t, results["tools/script.cr"], map[string]lang.Target{
		"./lib/helper":     {Local: "tools/lib/helper.cr"},
		"../src/shop/cart": {Local: "src/shop/cart.cr"},
	})
	langtest.CheckImports(t, results["libs/widgets/src/widgets.cr"], map[string]lang.Target{
		"./widgets/button.cr": {Local: "libs/widgets/src/widgets/button.cr"},
	})
}

// Verifies: REQ-CRYSTAL-005, REQ-CRYSTAL-006
func TestManifests(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckImports(t, results["shard.yml"], map[string]lang.Target{
		"kemal":             kemal,
		"db":                database,
		"radix":             radix,
		"markd":             markd,
		"crinja":            crinja,
		"internal":          internal,
		"crystal-sqlite3":   sqlite,
		"widgets":           {Local: "libs/widgets"},
		"spectator":         spectator,
		"main: src/shop.cr": {Local: "src/shop.cr"},
	})
	langtest.CheckImports(t, results["shard.lock"], map[string]lang.Target{
		"crystal-sqlite3": sqlite,
		"db":              database,
		"exception_page":  exception,
		"kemal":           kemal,
		"spectator":       spectator,
		"widgets":         {Local: "libs/widgets"},
	})
	langtest.CheckImports(t, results["shard.override.yml"], map[string]lang.Target{"radix": radix})
	langtest.CheckSymbols(t, results["shard.yml"], map[string]string{"shop": "package"})
	langtest.CheckSymbols(t, results["libs/widgets/shard.yml"], map[string]string{"widgets": "package"})
}

// Verifies: REQ-CRYSTAL-006
func TestPinRule(t *testing.T) {
	for _, c := range []struct {
		d    dependency
		want lang.Target
	}{
		{dependency{commit: "5e5a4a4f7c3e8f3d0f9b7e2a3c1d4b6a8e9f0a1b"}, lang.Target{Version: "5e5a4a4f7c3e8f3d0f9b7e2a3c1d4b6a8e9f0a1b", Pinned: true}},
		{dependency{tag: "v1.0.0"}, lang.Target{Version: "v1.0.0"}},
		{dependency{branch: "main"}, lang.Target{Version: "main", Floating: true}},
		{dependency{version: "1.2.3"}, lang.Target{Version: "1.2.3"}},
		{dependency{version: "~> 1.2"}, lang.Target{Version: "~> 1.2", Floating: true}},
		{dependency{version: ">= 0.5, < 1.0"}, lang.Target{Version: ">= 0.5, < 1.0", Floating: true}},
		{dependency{version: "*"}, lang.Target{Version: "*", Floating: true}},
		{dependency{}, lang.Target{Floating: true}},
	} {
		var got lang.Target
		pinRule(&got, &c.d)
		if got != c.want {
			t.Errorf("%+v: got %+v, want %+v", c.d, got, c.want)
		}
	}
}

// Verifies: REQ-CRYSTAL-003
func TestSymbols(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckSymbols(t, results["src/shop.cr"], map[string]string{
		"Shop":                     "module",
		"Shop::VERSION_NAME":       "const",
		"Shop::TEMPLATE":           "const",
		"Shop::QUOTED":             "const",
		"Shop::WORDS":              "const",
		"Shop::CHAR":               "const",
		"Shop::RE":                 "const",
		"Shop::Audited":            "annotation",
		"Shop::LibSSL":             "lib",
		"Shop::LibSSL::SizeT":      "type",
		"Shop::LibSSL::Context":    "type",
		"Shop::LibSSL::Buffer":     "struct",
		"Shop::LibSSL::Value":      "union",
		"Shop::LibSSL.ssl_init":    "func",
		"Shop::LibSSL.ssl_free":    "func",
		"Shop::Status":             "enum",
		"Shop::Status.open?":       "method",
		"Shop::Money":              "type",
		"Shop::Price":              "struct",
		"Shop::Price.to_s":         "method",
		"Shop::Item":               "class",
		"Shop::Item.price":         "method",
		"Shop::Item.name":          "attr",
		"Shop::Item.active":        "attr",
		"Shop::Item.count":         "attr",
		"Shop::Store":              "class",
		"Shop::Store.initialize":   "method",
		"Shop::Store.price":        "method",
		"Shop::Store.open":         "method",
		"Shop::Store.[]":           "method",
		"Shop::Store.==":           "method",
		"Shop::Store.name=":        "method",
		"Shop::Store.begin":        "method",
		"Shop::Store.delegate_all": "macro",
		"main":                     "func",
	})
	langtest.CheckSymbols(t, results["src/shop/models/admin/role.cr"], map[string]string{
		"Shop::Models::Admin":       "module",
		"Shop::Models::Admin::Role": "class",
	})
}

// What looks like code inside strings, heredocs, %-literals, regexes, chars,
// comments and macro tags is not read as code.
//
// Verifies: REQ-CRYSTAL-002, REQ-CRYSTAL-010
func TestLiteralsHideCode(t *testing.T) {
	source := `# require "comment"
x = "#{"require \"nested\""} #{y}"
s = <<-EOS
  require "heredoc"
  EOS
t = <<-'RAW'
  class Raw
  RAW
q = %q(C:\) + %Q[require "q"] + %w(require x) + %<a <b> c>
r = /require "re"/ if x
c = '"'
d = '\''
e = a // 2
{% for n in %w(a b) %}
  require "./{{n.id}}"
{% end %}
cmd = ` + "`require \"cmd\"`" + `
require "./real"
class Real
  def fetch
    1 if x
  end
end
`
	extraction := extractSource([]byte(source))
	var specs []string
	for _, rawImport := range extraction.Imports {
		specs = append(specs, rawImport.Spec)
	}
	if !reflect.DeepEqual(specs, []string{"./real"}) {
		t.Errorf("imports: %v", specs)
	}
	want := []lang.Symbol{{Name: "Real", Kind: "class", Line: 19}, {Name: "Real.fetch", Kind: "method", Line: 20}}
	if !reflect.DeepEqual(extraction.Symbols, want) {
		t.Errorf("symbols: %v", extraction.Symbols)
	}
}

// Branches of a macro {% if %} each start from the same place, so two versions
// of one method's signature leave one body open for the `end` below.
//
// Verifies: REQ-CRYSTAL-003
func TestMacroBranches(t *testing.T) {
	source := `module M
  {% if flag?(:x) %}
  def f(a : Int32)
  {% else %}
  def f(a)
  {% end %}
    a
  end

  def g
  end
end
`
	got := map[string]bool{}
	for _, s := range extractSource([]byte(source)).Symbols {
		got[s.Name] = true
	}
	if !got["M.g"] || !got["M.f"] {
		t.Errorf("symbols: %v", got)
	}
}

// Verifies: REQ-CRYSTAL-001
func TestClaims(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	for _, p := range []string{"lib/kemal/src/kemal.cr", "lib/kemal/shard.yml", ".crystal/cache/macro.cr"} {
		if results[p] != nil {
			t.Errorf("%s: analyzed", p)
		}
	}
	for _, p := range []string{"tools/lib/helper.cr", "shard.lock", "shard.override.yml", "libs/widgets/shard.yml"} {
		if results[p] == nil {
			t.Errorf("%s: not analyzed", p)
		}
	}
	for p, want := range map[string]string{"shard.yml": classShard, "a/shard.lock": classLock,
		"shard.override.yml": classOverride, "config.yml": "", "src/x.cr": ""} {
		if got := (Plugin{}).Class(&scan.File{Path: p}); got != want {
			t.Errorf("Class(%s) = %q, want %q", p, got, want)
		}
	}
	if (Plugin{}).Claims(&scan.File{Path: "config.yml"}) || !(Plugin{}).Claims(&scan.File{Path: "lib/x.cr"}) {
		t.Error("claims")
	}
}

// Verifies: REQ-CRYSTAL-008
func TestInstalledDependencies(t *testing.T) {
	r := newResolver("testdata/repo", langtest.Files(t, "testdata/repo"), noEnvironment)
	got := r.Dependencies(kemal)
	want := []lang.Target{exception, radix} // ameba is a development dependency
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if !r.Installed(kemal) || r.Installed(database) {
		t.Error("installed")
	}
	if dependencies := r.Dependencies(database); dependencies != nil {
		t.Errorf("db: %v", dependencies)
	}
}

// A library commits no shard.lock: lib/.shards.info pins what shards installed,
// kemal and radix, the dependency only kemal names. A garbage .shards.info pins
// nothing, and radix floats on kemal's requirement.
//
// Verifies: REQ-CRYSTAL-008
func TestShardsInfo(t *testing.T) {
	files := map[string]string{
		"shard.yml":           "name: app\ndependencies:\n  kemal:\n    github: kemalcr/kemal\n    version: ~> 1.4\n",
		"lib/kemal/shard.yml": "name: kemal\ndependencies:\n  radix:\n    github: luislavena/radix\n    version: ~> 0.4.0\n",
		"lib/radix/shard.yml": "name: radix\n",
		"lib/.shards.info": "version: 1.0\nshards:\n  kemal:\n    git: https://github.com/kemalcr/kemal.git\n    version: 1.4.0\n" +
			"  radix:\n    git: https://github.com/luislavena/radix.git\n    version: 0.4.1\n",
	}
	root := langtest.Write(t, files)
	r := newResolver(root, langtest.Files(t, root), noEnvironment)
	app := r.projects["."]
	kemal := lang.Target{Ecosystem: ecosystemShards, Package: "kemal", Version: "1.4.0", Requested: "~> 1.4", Pinned: true}
	if got := r.shardTarget(app, "kemal"); got != kemal {
		t.Errorf("kemal: %+v", got)
	}
	radix := lang.Target{Ecosystem: ecosystemShards, Package: "radix", Version: "0.4.1", Pinned: true}
	if got := r.Dependencies(kemal); !reflect.DeepEqual(got, []lang.Target{radix}) || !r.Installed(kemal) {
		t.Errorf("kemal depends on %+v", got)
	}

	files["lib/.shards.info"] = "{{{ not yaml"
	root = langtest.Write(t, files)
	r = newResolver(root, langtest.Files(t, root), noEnvironment)
	floating := lang.Target{Ecosystem: ecosystemShards, Package: "radix", Version: "~> 0.4.0", Floating: true}
	if got := r.Dependencies(kemal); !reflect.DeepEqual(got, []lang.Target{floating}) {
		t.Errorf("garbage .shards.info: %+v", got)
	}
}

// Every prefix of every fixture file, and long runs of what opens something,
// are read without a panic and in linear time.
//
// Verifies: REQ-CRYSTAL-010
func TestTruncated(t *testing.T) {
	var files []string
	filepath.Walk("testdata", func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	for _, p := range files {
		source, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		f := &scan.File{Path: strings.TrimPrefix(filepath.ToSlash(p), "testdata/repo/")}
		for i := 0; i <= len(source); i++ {
			if _, err := (Plugin{}).Extract(f, source[:i]); err != nil {
				t.Fatal(err)
			}
			extractSource(source[:i])
		}
	}
	for _, unit := range []string{"{", "(", "[", "}", ")", "end ", "\"", "\"#{", "#{", "'", "\\", "%(", "%q(", "%w[", "/",
		"<<-A\n", "{%", "{{", "{% if x %}", "{% else %}", "{% end %}", "class A\n", "def f\n", "if x\n", "x if y\n",
		"record A, b do\n", "getter a, ", "require \"", "::", ":", "@[", "$", "é", "macro m\n", "lib L\n fun f\n"} {
		source := []byte(strings.Repeat(unit, 200_000/len(unit)+1))
		start := time.Now()
		extractSource(source)
		if d := time.Since(start); d > 5*time.Second {
			t.Errorf("%q x %d: %v", unit, len(source)/len(unit), d)
		}
	}
}

// Verifies: REQ-CRYSTAL-009
func TestIslands(t *testing.T) {
	ids := map[string]bool{}
	for _, e := range (Plugin{}).Ecosystems() {
		ids[e.ID] = e.Std
	}
	if std, ok := ids[ecosystemStd]; !ok || !std || len(ids) != 2 || ids[ecosystemShards] {
		t.Fatalf("ecosystems: %v", ids)
	}
	for f, r := range langtest.Analyze(t, Plugin{}, "testdata/repo") {
		for _, imported := range r.Imports {
			if e := imported.Target.Ecosystem; e != "" && e != ecosystemShards && e != ecosystemStd {
				t.Errorf("%s: %s -> %s", f, imported.Spec, e)
			}
		}
	}
}

// CRYSTAL_PATH adds directories of the repository to look requires up in:
// deps_local/ (relative) and extra/ (absolute), by the compiler's rules for a
// shard. Entries outside the repository, `$ORIGIN` ones and garbage add
// nothing.
//
// Verifies: REQ-CRYSTAL-004
func TestCrystalPath(t *testing.T) {
	root := langtest.Write(t, map[string]string{
		"shard.yml":                     "name: app\n",
		"src/app.cr":                    "require \"vendorlib\"\nrequire \"tool/helper\"\nrequire \"extra\"\nrequire \"json\"\nrequire \"orphan\"\n",
		"deps_local/vendorlib.cr":       "module VendorLib; end\n",
		"deps_local/tool/src/helper.cr": "module Helper; end\n",
		"extra/extra/src/extra.cr":      "module Extra; end\n",
		"deps2/orphan.cr":               "module Orphan; end\n",
	})
	absolute, _ := filepath.Abs(filepath.Join(root, "extra"))
	separator := string(os.PathListSeparator)
	for _, c := range []struct {
		environment string
		want        map[string]lang.Target
	}{
		{"lib" + separator + "deps_local" + separator + absolute + separator + "/usr/share/crystal/src" + separator + "$ORIGIN/../deps2", map[string]lang.Target{
			"vendorlib":   {Local: "deps_local/vendorlib.cr"},
			"tool/helper": {Local: "deps_local/tool/src/helper.cr"},
			"extra":       {Local: "extra/extra/src/extra.cr"},
			"json":        std("json"),
			"orphan":      {Ecosystem: ecosystemShards, Package: "orphan", Unresolved: true},
		}},
		{"  " + separator + separator + "../elsewhere" + separator + "src/app.cr", map[string]lang.Target{
			"vendorlib":   {Ecosystem: ecosystemShards, Package: "vendorlib", Unresolved: true},
			"tool/helper": {Ecosystem: ecosystemShards, Package: "tool", Unresolved: true},
			"extra":       {Ecosystem: ecosystemShards, Package: "extra", Unresolved: true},
			"json":        std("json"),
			"orphan":      {Ecosystem: ecosystemShards, Package: "orphan", Unresolved: true},
		}},
	} {
		t.Setenv("CRYSTAL_PATH", c.environment)
		langtest.CheckImports(t, langtest.Analyze(t, Plugin{}, root)["src/app.cr"], c.want)
	}
}

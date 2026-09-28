package luarocks

import (
	"reflect"
	"strings"
	"testing"
)

// Verifies: REQ-LUA-012
func TestLex(t *testing.T) {
	source := "\xef\xbb\xbfx = 'a\\tb' .. \"\\65\\x42\\u{43}\\z\n  d\" --[=[ gone ]=] [==[\nlong]]x]==] 0x1F 3.5e-2 a//=b `i{1}`"
	var got []string
	for _, token := range Lex([]byte(source)) {
		got = append(got, token.Text)
	}
	want := []string{"x", "=", "a\tb", "..", "ABCd", "long]]x", "0x1F", "3.5e-2", "a", "//=", "b", ""}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A rockspec's variables, concatenations, platform dependencies, test and build
// dependencies, and modules as strings, C sources and install.lua.
//
// Verifies: REQ-LUA-006
func TestReadRockspec(t *testing.T) {
	source := `local MODREV, SPECREV = 'scm', '-1'
rockspec_format = '3.0'
package = 'telescope.nvim'
version = MODREV .. SPECREV
description = { detailed = [[
  long ]] }
dependencies = {
  'lua == 5.1',
  'plenary.nvim',
  "LuaSocket >= 3.0, < 4",
  platforms = { windows = { "winapi" } },
}
build_dependencies = { "luarocks-build-rust-mlua" }
test_dependencies = { "busted ~> 2" }
build = {
  type = "builtin",
  modules = {
    ["a.b"] = "src/a/b.lua",
    c = { sources = { "src/c.c", "src/c2.c" } },
    d = { "src/d.c" },
  },
  install = { lua = { ["a.e"] = "src/a/e.lua" } },
  platforms = { unix = { modules = { u = "src/u.lua" } } },
}
if something() then build.modules.x = "x.lua" end
`
	r := ReadRockspec([]byte(source))
	if r.Package != "telescope.nvim" || r.Version != "scm-1" {
		t.Errorf("package %q version %q", r.Package, r.Version)
	}
	var dependencies []string
	for _, d := range r.Dependencies {
		dependencies = append(dependencies, d.Section+":"+d.Name+"|"+d.Constraint)
	}
	wantDependencies := []string{"dependencies:lua|== 5.1", "dependencies:plenary.nvim|", "dependencies:luasocket|>= 3.0, < 4",
		"dependencies:winapi|", "build_dependencies:luarocks-build-rust-mlua|", "test_dependencies:busted|~> 2"}
	if !reflect.DeepEqual(dependencies, wantDependencies) {
		t.Errorf("deps %q", dependencies)
	}
	if r.Dependencies[1].Line != 9 {
		t.Errorf("line %d", r.Dependencies[1].Line)
	}
	var modules []string
	for _, m := range r.Modules {
		modules = append(modules, m.Name+"="+m.File)
	}
	wantModules := []string{"a.b=src/a/b.lua", "c=src/c.c", "d=src/d.c", "x=x.lua", "a.e=src/a/e.lua", "u=src/u.lua"}
	if !reflect.DeepEqual(modules, wantModules) {
		t.Errorf("modules %q", modules)
	}
}

// Verifies: REQ-LUA-007, REQ-LUA-008
func TestLockAndConstraints(t *testing.T) {
	lock := ReadLock([]byte(`return {
   dependencies = {
      ["lua-cjson"] = "2.1.0.10-1",
      Penlight = "1.14.0-3",
   },
}`))
	if want := map[string]string{"lua-cjson": "2.1.0.10-1", "penlight": "1.14.0-3"}; !reflect.DeepEqual(lock, want) {
		t.Errorf("lock %v", lock)
	}
	for c, want := range map[string]string{"== 1.2": "1.2", "1.2.3": "1.2.3", "== 3.0-rc1": "3.0-rc1"} {
		if v, ok := Exact(c); !ok || v != want {
			t.Errorf("Exact(%q) = %q, %v", c, v, ok)
		}
	}
	for _, c := range []string{"", "~> 1.2", ">= 1", ">= 1, < 2", "~= 1"} {
		if v, ok := Exact(c); ok {
			t.Errorf("Exact(%q) = %q", c, v)
		}
	}
	if name, c := ParseDependency(" Lua-CJSON>=2.1 "); name != "lua-cjson" || c != ">=2.1" {
		t.Errorf("ParseDep: %q %q", name, c)
	}
}

// Verifies: REQ-SUP-052
func TestVersions(t *testing.T) {
	ordered := []string{"1.0rc1-1", "1.0-1", "1.0-2", "1.2-1", "1.9.2-1", "1.10.0-1", "scm-1"}
	for i := 1; i < len(ordered); i++ {
		if Compare(ordered[i-1], ordered[i]) >= 0 {
			t.Errorf("%s !< %s", ordered[i-1], ordered[i])
		}
	}
	cases := []struct {
		v, c string
		ok   bool
	}{
		{"1.5.4-1", "~> 1.5", true}, {"1.6.0-1", "~> 1.5", false}, {"2.0-1", ">= 1.0, < 2.0", false},
		{"1.9-1", ">= 1.0, < 2.0", true}, {"1.2.3-4", "== 1.2.3", true}, {"1.2.3-4", "1.2.3-3", false},
		{"3.0-1", "~= 3.0", false}, {"1.0-1", "", true},
	}
	for _, c := range cases {
		if got := Satisfies(c.v, c.c); got != c.ok {
			t.Errorf("Satisfies(%q, %q) = %v", c.v, c.c, got)
		}
	}
	versions := []string{"1.2.0-1", "scm-1", "1.10.0-2", "1.10.0-1", "2.0.0-1"}
	if got := Newest(versions, "< 2"); got != "1.10.0-2" {
		t.Errorf("Newest: %s", got)
	}
	if got := Newest([]string{"scm-1"}, ""); got != "scm-1" {
		t.Errorf("Newest dev only: %s", got)
	}
}

// Verifies: REQ-SUP-052
func TestManifestAndServers(t *testing.T) {
	m := ReadManifest([]byte(`commands = {}
modules = {}
repository = {
   penlight = {
      ["1.14.0-3"] = { { arch = "rockspec" }, { arch = "all" } },
      ["1.13.1-1"] = { { arch = "src" } },
   },
   ["lua-cjson"] = { ["2.1.0.10-1"] = { { arch = "rockspec" } } },
}`))
	if want := map[string][]string{"penlight": {"1.14.0-3"}, "lua-cjson": {"2.1.0.10-1"}}; !reflect.DeepEqual(m, want) {
		t.Errorf("manifest %v", m)
	}
	s := Servers([]byte(`rocks_servers = {
   { "https://rocks.corp.test/", "https://mirror.corp.test" },
   "https://luarocks.org",
}
local_by_default = true`))
	if want := []string{"https://rocks.corp.test/", "https://mirror.corp.test", "https://luarocks.org"}; !reflect.DeepEqual(s, want) {
		t.Errorf("servers %v", s)
	}
}

// Deep nesting and cut-off input do not break the evaluator.
//
// Verifies: REQ-LUA-012
func TestEvalSurvives(t *testing.T) {
	source := "x = " + strings.Repeat("{", 5000) + strings.Repeat("(", 5000) + strings.Repeat("- ", 5000)
	for i := 0; i < len(source); i += 997 {
		Eval([]byte(source[:i]))
	}
	rockspec := `package = "p" version = "1-1" dependencies = { "a >= 1", "b" } build = { modules = { x = "x.lua" } }`
	for i := range rockspec {
		ReadRockspec([]byte(rockspec[:i]))
	}
}

package lua

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// A LuaRocks project (a rockspec with platform dependencies, a C module and
// build.modules, luarocks.lock, sources under src/, a .luarc.json library, the
// lua_modules tree that is not claimed), a Neovim plugin (lua/<name>/), a Roblox
// game (a Rojo place project, wally.toml and wally.lock, .luaurc aliases, requires
// by instance and by string) and a Teal file.
//
// Verifies: REQ-LUA-001, REQ-LUA-002, REQ-LUA-004, REQ-LUA-005, REQ-LUA-006
// Verifies: REQ-LUA-007, REQ-LUA-008, REQ-LUA-009, REQ-LUA-010, REQ-LUA-011
func TestRequiresAndManifests(t *testing.T) {
	res := langtest.Analyze(t, Plugin{}, "testdata/repo")
	penlight := lang.Target{Ecosystem: ecoRocks, Package: "penlight", Version: "1.14.0-3", Requested: "~> 1.5", Pinned: true}
	luasocket := lang.Target{Ecosystem: ecoRocks, Package: "luasocket", Floating: true}
	cjson := lang.Target{Ecosystem: ecoRocks, Package: "lua-cjson", Version: "2.1.0", Pinned: true}
	lpeg := lang.Target{Ecosystem: ecoRocks, Package: "lpeg", Version: "1.1.0", Pinned: true}
	roact := lang.Target{Ecosystem: ecoWally, Package: "roblox/roact", Version: "1.4.4", Requested: "1.4.0", Pinned: true}
	promise := lang.Target{Ecosystem: ecoWally, Package: "evaera/promise", Version: "4.0.0", Pinned: true}
	imports := map[string]map[string]lang.Target{
		"shop-dev-1.rockspec": {
			"penlight ~> 1.5":      penlight,
			"luasocket":            luasocket,
			"lua-cjson == 2.1.0":   cjson,
			"lpeg 1.1.0":           lpeg,
			"luaposix >= 35":       {Ecosystem: ecoRocks, Package: "luaposix", Version: ">= 35"},
			"busted":               {Ecosystem: ecoRocks, Package: "busted", Floating: true},
			"modules[shop]":        {Local: "src/shop/init.lua"},
			"modules[shop.cart]":   {Local: "src/shop/cart.lua"},
			"modules[shop.native]": {Local: "csrc/native.c"},
		},
		"src/shop/init.lua": {
			`require("shop.cart")`:           {Local: "src/shop/cart.lua"},
			`require 'pl.utils'`:             penlight,
			`require "socket.http"`:          luasocket,
			`pcall(require, "cjson")`:        cjson,
			`require("lpeg")`:                lpeg,
			`require("string")`:              {Ecosystem: ecoStd, Package: "string"},
			`require("jit.opt")`:             {Ecosystem: ecoStd, Package: "jit"},
			`require("lfs")`:                 {Ecosystem: ecoRocks, Package: "luafilesystem", Version: "1.8.0-1", Pinned: true},
			`require("shop.util")`:           {Local: "src/shop/util.lua"},
			`require("ssl")`:                 {Ecosystem: ecoRocks, Package: "luasec", Unresolved: true},
			`require("resty.http")`:          {Ecosystem: ecoRocks, Package: "lua-resty-http", Unresolved: true},
			`require("ngx.ssl")`:             {Ecosystem: ecoRuntime, Package: "openresty"},
			`require("unknownmod.x")`:        {Ecosystem: ecoRocks, Package: "unknownmod", Unresolved: true},
			`require [[shop.native]]`:        {Local: "csrc/native.c"},
			`dofile("scripts/setup.lua")`:    {Local: "scripts/setup.lua"},
			`loadfile("scripts/absent.lua")`: {},
			`require("extra.mod")`:           {Local: "third_party/lib/extra/mod.lua"},
		},
		"nvim/myplugin/lua/myplugin/init.lua": {
			`require("myplugin.config")`: {Local: "nvim/myplugin/lua/myplugin/config.lua"},
			`require("vim.lsp")`:         {Ecosystem: ecoRuntime, Package: "neovim"},
			`require("plenary.async")`:   {Ecosystem: ecoRocks, Package: "plenary.nvim", Unresolved: true},
			`require("love.graphics")`:   {Ecosystem: ecoRuntime, Package: "love2d"},
		},
		"game/default.project.json": {
			"$path: src/shared": {Local: "game/src/shared"},
			"$path: Packages":   {},
		},
		"game/wally.toml": {
			`Roact = "roblox/roact@1.4.0"`:      roact,
			`Promise = "evaera/promise@=4.0.0"`: promise,
			`TestEZ = "roblox/testez@0.4.1"`:    {Ecosystem: ecoWally, Package: "roblox/testez", Version: "0.4.1"},
		},
		"game/src/shared/Game/init.luau": {
			"require(script.Parent.Util)":                                                 {Local: "game/src/shared/Util.lua"},
			"require(script.Child)":                                                       {Local: "game/src/shared/Game/Child.luau"},
			"require(ReplicatedStorage.Shared.Types)":                                     {Local: "game/src/shared/Types.luau"},
			"require(ReplicatedStorage.Packages.Roact)":                                   roact,
			`require(ReplicatedStorage:WaitForChild("Packages"):WaitForChild("Promise"))`: promise,
			"require(Shared.Util)":                                                        {Local: "game/src/shared/Util.lua"},
			`require("./Util")`:                                                           {Local: "game/src/shared/Util.lua"},
			`require("@self/Child")`:                                                      {Local: "game/src/shared/Game/Child.luau"},
			`require("@Shared/Types")`:                                                    {Local: "game/src/shared/Types.luau"},
			`require("@lune/fs")`:                                                         {Ecosystem: ecoRuntime, Package: "lune"},
			`require("../missing")`:                                                       {},
			"require(script.Parent.Nope)":                                                 {},
		},
	}
	for file, want := range imports {
		t.Run(file, func(t *testing.T) { langtest.CheckImports(t, res[file], want) })
	}
	for _, f := range []string{"lua_modules/share/lua/5.1/pl.lua", "csrc/native.c", "game/wally.lock", "luarocks.lock", ".luarc.json"} {
		if _, ok := res[f]; ok {
			t.Errorf("%s: claimed", f)
		}
	}
}

// Module tables, functions and methods (a.b:c), fields the module exports, Luau
// types and if-expressions, and Teal records, enums and function types - top-level
// only.
//
// Verifies: REQ-LUA-003
func TestDefinitions(t *testing.T) {
	res := langtest.Analyze(t, Plugin{}, "testdata/repo")
	symbols := map[string]map[string]string{
		"src/shop/init.lua": {
			"M": "table", "M.version": "var", "M.add": "function", "M.checkout": "method",
			"M.handler": "method", "helper": "function", "M.maybe": "function",
		},
		"src/shop/cart.lua":                     {"cart": "table", "cart.add": "function"},
		"nvim/myplugin/lua/myplugin/config.lua": {"M": "table", "M.defaults": "table", "M.apply": "function"},
		"game/src/shared/Types.luau": {
			"Item": "type", "Pair": "type", "M": "table", "describe": "function", "M.after": "function",
		},
		"game/src/shared/Game/init.luau": {"Game": "table", "Game.start": "function"},
		"tl/shapes.tl": {
			"Point": "class", "Color": "enum", "Callback": "type", "make": "function", "helper": "function",
		},
		"scripts/setup.lua": {},
	}
	for file, want := range symbols {
		langtest.CheckSymbols(t, res[file], want)
	}
}

// wally.lock records what every locked package depends on.
//
// Verifies: REQ-LUA-009
func TestWallyLockDependencies(t *testing.T) {
	root := "testdata/repo"
	r := newResolver(root, langtest.Files(t, root))
	got := r.Dependencies(lang.Target{Ecosystem: ecoWally, Package: "roblox/roact", Version: "1.4.4"})
	want := []lang.Target{{Ecosystem: ecoWally, Package: "evaera/promise", Version: "4.0.0", Pinned: true}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if got := r.Dependencies(lang.Target{Ecosystem: ecoWally, Package: "evaera/promise"}); len(got) != 0 {
		t.Errorf("promise: %+v", got)
	}
	if got := r.Dependencies(lang.Target{Ecosystem: ecoRocks, Package: "penlight"}); got != nil {
		t.Errorf("luarocks.lock records no edges: %+v", got)
	}
}

// Verifies: REQ-LUA-001
func TestClaimsAndClasses(t *testing.T) {
	claims := map[string]string{
		"a.lua": "", "b.luau": "", "c.tl": "", "shop-1.0-1.rockspec": "", "wally.toml": "wally",
		"default.project.json": "rojo", "place.project.json": "rojo",
	}
	for p, class := range claims {
		f := &scan.File{Path: p}
		if !(Plugin{}).Claims(f) {
			t.Errorf("%s: not claimed", p)
		}
		if got := (Plugin{}).Class(f); got != class {
			t.Errorf("%s: class %q, want %q", p, got, class)
		}
	}
	for _, p := range []string{"project.json", "Cargo.toml", "lua_modules/share/lua/5.1/x.lua", "Packages/Roact.lua",
		"x/.luarocks/y.lua", "a.c"} {
		if (Plugin{}).Claims(&scan.File{Path: p}) {
			t.Errorf("%s: claimed", p)
		}
	}
}

// The scanner reads what Lua writes around a require without losing its place:
// long strings and comments of any level, escapes, Luau interpolation with nested
// braces and backticks, if-expressions, and a require after all of them.
//
// Verifies: REQ-LUA-002, REQ-LUA-012
func TestScanner(t *testing.T) {
	src := "#!/usr/bin/env lua\n" +
		"local s = [==[ ]] require('no') ]==] --[[ require('no')\n]]\n" +
		"local e = \"\\\"\\z\n   \\x41\\u{48}\\065\" -- require('no')\n" +
		"local i = `{ {a = `}`} } }` .. 'x'\n" +
		"local v = if a then function() end else nil\n" +
		"local w = x // 2 ~= 3 and 0x1p4 or 1e-3\n" +
		"repeat local r = 1 until r\n" +
		"local d = pcall(require, 'drivers.' .. name)\n" +
		"function late() return require 'yes' end\n"
	ex := readSource([]byte(src), false)
	if len(ex.Imports) != 1 || ex.Imports[0].Module != "yes" || ex.Imports[0].Line != 11 {
		t.Errorf("imports: %+v", ex.Imports)
	}
	if len(ex.Symbols) != 1 || ex.Symbols[0].Name != "late" || ex.Symbols[0].Line != 11 {
		t.Errorf("symbols: %+v", ex.Symbols)
	}
}

// Every prefix of every fixture file, and inputs cut in the middle of each
// construct, extract without panicking (a panic would stop the whole analysis).
//
// Verifies: REQ-LUA-012
func TestTruncated(t *testing.T) {
	cuts := []string{"--[==[", "[[", "\"\\", "\"\\u{", "\"\\x", "`{", "`{ \"", "require(", "require(script.",
		"require(game:GetService(", "local x: ", "local x <", "function a.", "local record", "type X<",
		"pcall(require,", "0x", "1e", "--[", "\"\\z"}
	for _, c := range cuts {
		readSource([]byte(c), false)
		readSource([]byte(c), true)
	}
	filepath.WalkDir("testdata/repo", func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		src, _ := os.ReadFile(p)
		f := &scan.File{Path: filepath.ToSlash(p)}
		for i := range src {
			if _, err := (Plugin{}).Extract(f, src[:i]); err != nil {
				t.Errorf("%s[:%d]: %v", p, i, err)
			}
		}
		return nil
	})
	deep := strings.Repeat("{", 100000) + strings.Repeat("(", 100000)
	readSource([]byte(deep), false)
	extractRockspec([]byte("dependencies = " + deep))
}

// A rock installed in the repository's LuaRocks tree depends on what its
// rockspec's dependencies name (not Lua, not its test dependencies), each pinned
// to the newest version the tree holds; luasocket, in the .luarocks tree only,
// floats there. A rockspec that is not Lua names none, and a rock no tree holds
// is not answered.
//
// Verifies: REQ-LUA-013
func TestInstalledRocks(t *testing.T) {
	const rocks = "lua_modules/lib/luarocks/rocks-5.1/"
	files := map[string]string{
		"app-1.0-1.rockspec": "package = \"app\"\nversion = \"1.0-1\"\ndependencies = { \"lua-resty-http >= 0.17\" }\n",
		rocks + "manifest":   "repository = {}\n",
		rocks + "lua-resty-http/0.17.1-0/lua-resty-http-0.17.1-0.rockspec": "rockspec_format = \"3.0\"\n" +
			"package = \"lua-resty-http\"\nversion = \"0.17.1-0\"\n" +
			"dependencies = { \"lua >= 5.1\", \"lua-resty-openssl >= 0.9\", \"luasocket\" }\n" +
			"test_dependencies = { \"busted\" }\n",
		rocks + "lua-resty-openssl/0.9.0-1/lua-resty-openssl-0.9.0-1.rockspec":          "package = \"lua-resty-openssl\"\n",
		rocks + "lua-resty-openssl/0.10.0-1/lua-resty-openssl-0.10.0-1.rockspec":        "package = \"lua-resty-openssl\"\n",
		".luarocks/lib/luarocks/rocks-5.4/luasocket/3.1.0-1/luasocket-3.1.0-1.rockspec": "dependencies = { \"lua >= 5.1\" }\n",
	}
	root := langtest.Write(t, files)
	r := newResolver(root, langtest.Files(t, root))
	http := lang.Target{Ecosystem: ecoRocks, Package: "lua-resty-http", Version: "0.17.1-0", Pinned: true}
	want := []lang.Target{
		{Ecosystem: ecoRocks, Package: "lua-resty-openssl", Version: "0.10.0-1", Requested: ">= 0.9", Pinned: true},
		{Ecosystem: ecoRocks, Package: "luasocket", Floating: true},
	}
	if got := r.Dependencies(http); !reflect.DeepEqual(got, want) || !r.Installed(http) {
		t.Errorf("lua-resty-http depends on %+v", got)
	}
	if got := r.Dependencies(want[1]); got == nil || len(got) != 0 || !r.Installed(want[1]) {
		t.Errorf("luasocket depends on %+v", got)
	}
	busted := lang.Target{Ecosystem: ecoRocks, Package: "busted"}
	if got := r.Dependencies(busted); got != nil || r.Installed(busted) {
		t.Errorf("busted is not installed: %+v", got)
	}

	files[rocks+"lua-resty-http/0.17.1-0/lua-resty-http-0.17.1-0.rockspec"] = "dependencies = {{{ \x00"
	root = langtest.Write(t, files)
	if got := newResolver(root, langtest.Files(t, root)).Dependencies(http); len(got) != 0 {
		t.Errorf("garbage rockspec: %+v", got)
	}
}

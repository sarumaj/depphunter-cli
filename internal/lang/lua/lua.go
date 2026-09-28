// Package lua analyzes Lua (5.1 to 5.4 and LuaJIT), Luau - Roblox's, with Rojo
// and Wally - and Teal, and LuaRocks. A require of a module name resolves to the
// project file a rockspec's build.modules maps it to, else to the file
// package.path's usual templates (?.lua, ?/init.lua) find from the requiring
// file's directory and its ancestors and their lua/, src/ and lib/ directories (a
// Neovim plugin's lua/), else to the hidden lua-std island (the standard library and
// LuaJIT's modules), a rock the rockspecs declare or luarocks.lock pins (by name, an
// alias table - lfs is luafilesystem - or its usual spellings), the hidden
// lua-runtime island (Neovim's vim.*, LÖVE's love.*, OpenResty's ngx.* and bundled
// resty libraries, Lune's @lune/*), or an unresolved rock named after the module.
// Luau's requires by path ("./x", "@self/x", .luaurc aliases) and by Roblox instance
// (script.Parent.X, game:GetService("ReplicatedStorage").Shared through a Rojo
// place project) resolve to files, and through a Packages folder to the Wally
// package wally.toml names so. Rockspecs, wally.toml and Rojo projects are read as
// imports of what they declare.
//
// Sources are read by a scanner of their own (source.go, on the luarocks package's
// lexer), not the tree-sitter grammar: the grammar took 3-5 ms per file and failed on
// Roblox's .lua files, which are Luau.
package lua

import (
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecosystemRocks   = "luarocks"
	ecosystemWally   = "wally"
	ecosystemStd     = "lua-std"
	ecosystemRuntime = "lua-runtime"
)

// Implements: REQ-LUA-001
type Plugin struct{}

func (Plugin) Name() string { return "lua" }
func (Plugin) Version() int { return 1 }

// Claims takes Lua, Luau and Teal sources, rockspecs, wally.toml and Rojo project
// files, except what LuaRocks and Wally install into the project (lua_modules,
// .luarocks, Packages).
//
// Implements: REQ-LUA-001
func (Plugin) Claims(f *scan.File) bool {
	if f.Binary || ignored(f.Path) {
		return false
	}
	switch base := path.Base(f.Path); {
	case base == "wally.toml":
		return true
	case strings.HasSuffix(base, ".project.json") && len(base) > len(".project.json"):
		return true
	}
	switch strings.ToLower(path.Ext(f.Path)) {
	case ".lua", ".luau", ".tl", ".rockspec":
		return true
	}
	return false
}

// Class tells wally.toml and Rojo projects from other .toml and .json files.
//
// Implements: REQ-LUA-001
func (Plugin) Class(f *scan.File) string {
	switch base := path.Base(f.Path); {
	case base == "wally.toml":
		return "wally"
	case strings.HasSuffix(base, ".project.json"):
		return "rojo"
	}
	return ""
}

func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecosystemRocks, Name: "LuaRocks"},
		{ID: ecosystemWally, Name: "Wally"},
		{ID: ecosystemStd, Name: "Lua standard library", Std: true},
		{ID: ecosystemRuntime, Name: "Lua host runtimes", Std: true},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all), nil
}

// Extract reads a source's definitions and requires, or a manifest's dependencies.
//
// Implements: REQ-LUA-002, REQ-LUA-003, REQ-LUA-006, REQ-LUA-009, REQ-LUA-010
func (p Plugin) Extract(f *scan.File, source []byte) (*lang.Extraction, error) {
	switch p.Class(f) {
	case "wally":
		return extractWally(source), nil
	case "rojo":
		return extractRojo(source), nil
	}
	switch strings.ToLower(path.Ext(f.Path)) {
	case ".rockspec":
		return extractRockspec(source), nil
	case ".tl":
		return readSource(source, true), nil
	}
	return readSource(source, false), nil
}

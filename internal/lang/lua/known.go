package lua

import "strings"

// std are the modules Lua and LuaJIT preload, which require returns without
// searching: the standard library, and LuaJIT's jit, ffi, bit and their
// submodules (jit.opt, table.new, string.buffer). They are the lua-std island.
var std = map[string]bool{
	"string": true, "table": true, "math": true, "io": true, "os": true, "coroutine": true,
	"debug": true, "utf8": true, "package": true, "bit32": true, "jit": true, "ffi": true,
	"bit": true, "_G": true,
}

// runtimes are the modules a host program provides rather than a rock: Neovim's
// runtime (require("vim.lsp")), LÖVE's love.*, OpenResty's ngx.* and the libraries
// it bundles, Lune's @lune/*. They are the lua-runtime island, named by the host.
//
// Implements: REQ-LUA-005
func runtime(module string) string {
	first, _, _ := strings.Cut(module, ".")
	switch first {
	case "vim":
		return "neovim"
	case "love":
		return "love2d"
	case "ngx", "ndk", "tablepool":
		return "openresty"
	case "resty":
		for _, p := range openrestyBundled {
			if module == p || strings.HasPrefix(module, p+".") {
				return "openresty"
			}
		}
	}
	return ""
}

// openrestyBundled are the lua-resty libraries OpenResty ships with, so a project
// running on it requires them without declaring a rock.
var openrestyBundled = []string{
	"resty.core", "resty.lrucache", "resty.string", "resty.sha1", "resty.sha224", "resty.sha256",
	"resty.sha384", "resty.sha512", "resty.md5", "resty.random", "resty.aes", "resty.lock",
	"resty.limit", "resty.upload", "resty.dns", "resty.mysql", "resty.redis", "resty.memcached",
	"resty.websocket", "resty.shell", "resty.signal", "resty.upstream", "resty.base",
}

// aliases are rocks whose modules are not named after them, by module name or its
// first segment: require("lfs") loads luafilesystem.
var aliases = map[string][]string{
	"lfs":         {"luafilesystem"},
	"socket":      {"luasocket"},
	"mime":        {"luasocket"},
	"ltn12":       {"luasocket"},
	"ssl":         {"luasec"},
	"cjson":       {"lua-cjson"},
	"posix":       {"luaposix"},
	"re":          {"lpeg"},
	"pl":          {"penlight"},
	"lxp":         {"luaexpat"},
	"zip":         {"luazip"},
	"system":      {"luasystem"},
	"term":        {"lua-term"},
	"mediator":    {"mediator_lua"},
	"zlib":        {"lua-zlib"},
	"luasql":      {"luasql-sqlite3", "luasql-postgres", "luasql-mysql", "luasql-odbc"},
	"pb":          {"lua-protobuf"},
	"protoc":      {"lua-protobuf"},
	"MessagePack": {"lua-messagepack"},
	"lsyslog":     {"luasyslog"},
	"crypto":      {"luacrypto"},
	"uuid":        {"uuid", "lua-uuid"},
	"plenary":     {"plenary.nvim"},
	"nui":         {"nui.nvim"},
	"lunit":       {"lunitx"},
	"rex_pcre":    {"lrexlib-pcre"},
	"rex_pcre2":   {"lrexlib-pcre2"},
	"rex_posix":   {"lrexlib-posix"},
	"sqlite3":     {"lsqlite3"},
	"ffi-zlib":    {"lua-ffi-zlib"},
	"yaml":        {"lyaml", "yaml"},
}

// candidates are the rock names a module may come from, most likely first: the
// alias table, lua-resty-<x> for resty.<x>, then the usual spellings of its first
// segment.
func candidates(module string) []string {
	first, rest, _ := strings.Cut(module, ".")
	var out []string
	if a, ok := aliases[module]; ok {
		out = append(out, a...)
	}
	if a, ok := aliases[first]; ok {
		out = append(out, a...)
	}
	if first == "resty" && rest != "" {
		second, _, _ := strings.Cut(rest, ".")
		out = append(out, "lua-resty-"+second)
	}
	f := strings.ToLower(first)
	return append(out, f, "lua-"+f, f+"-lua", "lua"+f, f+".nvim", "nvim-"+f)
}

// fold is a rock name for matching: lower case without - _ and dots.
func fold(s string) string { return folder.Replace(strings.ToLower(s)) }

var folder = strings.NewReplacer("-", "", "_", "", ".", "")

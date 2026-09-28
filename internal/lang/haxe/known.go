package haxe

import "strings"

// stdPackages are the top-level packages of Haxe's standard library: the
// cross-platform haxe and sys packages and each target's own.
var stdPackages = map[string]bool{
	"haxe": true, "sys": true, "js": true, "flash": true, "cpp": true, "cs": true, "java": true, "jvm": true,
	"python": true, "lua": true, "php": true, "hl": true, "neko": true, "eval": true,
}

// stdTypes are the standard library's top-level modules (the empty package).
var stdTypes = map[string]bool{
	"Any": true, "Array": true, "ArrayAccess": true, "Bool": true, "Class": true, "Date": true, "DateTools": true,
	"Dynamic": true, "EReg": true, "Enum": true, "EnumValue": true, "Float": true, "Int": true, "IntIterator": true,
	"Iterable": true, "Iterator": true, "KeyValueIterable": true, "KeyValueIterator": true, "Lambda": true,
	"List": true, "Map": true, "Math": true, "Null": true, "Reflect": true, "Single": true, "Std": true,
	"StdTypes": true, "String": true, "StringBuf": true, "StringTools": true, "Sys": true, "Type": true,
	"UInt": true, "UnicodeString": true, "ValueType": true, "Void": true, "Xml": true, "XmlType": true,
}

// known maps a package (or module) prefix to the haxelib library that
// provides it, for libraries whose packages do not spell their names. A
// longer prefix wins, so the entries under std packages (js.node, haxe.ui)
// take those modules out of the standard library.
var known = map[string]string{
	"openfl": "openfl", "lime": "lime", "flixel": "flixel", "flixel.addons": "flixel-addons",
	"flixel.ui": "flixel-ui", "flixel.tools": "flixel-tools",
	"hxd": "heaps", "h2d": "heaps", "h3d": "heaps", "hxsl": "heaps",
	"hxbit": "hxbit", "domkit": "domkit", "hxcpp": "hxcpp", "format": "format", "utest": "utest",
	"buddy": "buddy", "haxe.ui": "haxeui-core", "kha": "kha", "ceramic": "ceramic", "hscript": "hscript",
	"hxparse": "hxparse", "haxeparser": "haxeparser", "massive.munit": "munit", "mcover": "mcover",
	"massive.mcover": "mcover", "org.hamcrest": "hamcrest", "hamcrest": "hamcrest", "mockatoo": "mockatoo",
	"mcli": "mcli", "hxargs": "hxargs", "json2object": "json2object", "hx.strings": "haxe-strings",
	"hx.concurrent": "haxe-concurrent", "hx.files": "haxe-files", "hx.doctest": "haxe-doctest",
	"hx.ws": "hxWebSockets", "polygonal.ds": "polygonal-ds", "de.polygonal.ds": "polygonal-ds",
	"nape": "nape-haxe4", "box2D": "box2d", "echo": "echo", "nme": "nme", "feathers": "feathersui",
	"away3d": "away3d", "starling": "starling", "coconut.ui": "coconut.ui", "coconut.data": "coconut.data",
	"coconut.vdom": "coconut.vdom", "js.node": "hxnodejs", "js.npm": "hxnodejs", "yaml": "yaml",
	"hxjsonast": "hxjsonast", "markdown": "markdown", "hxmath": "hxmath", "datetime": "datetime",
	"hxp": "hxp", "checkstyle": "checkstyle", "formatter": "formatter", "tokentree": "tokentree",
	"vshaxe": "vshaxe", "vscode": "vscode", "msignal": "msignal", "promhx": "promhx", "slambda": "slambda",
	"thx": "thx.core", "sdl": "hlsdl", "dx": "hldx", "openal": "hlopenal",
	"sys.db.Object": "record-macros", "sys.db.Manager": "record-macros", "sys.db.Types": "record-macros",
	"sys.db.RecordMacros": "record-macros", "sys.db.RecordInfos": "record-macros", "sys.db.TableCreate": "record-macros",
	"haxe.remoting": "hxremoting", "hxcoro": "hxcoro", "safety": "safety",
	"cdb": "castle", "hrt": "hide", "hide": "hide", "haxeLanguageServer": "haxe-language-server", "jsasync": "jsasync",
	"hxWidgets": "hxWidgets", "hxtelemetry": "hxtelemetry", "deepequal": "deep_equal",
	"ansi": "ansi", "snow": "snow", "motion": "actuate", "hxnodejs": "hxnodejs", "haxe.ui.backend": "haxeui-core",
}

// tinkSpecial names the tink libraries whose package differs from the
// library's suffix.
var tinkSpecial = map[string]string{"coreapi": "tink_core", "macroapi": "tink_macro", "unit": "tink_unittest"}

// table is the library a module's leading segments name, how many segments
// matched, and whether it is the standard library ("" and std true). tink's
// libraries follow a rule: tink.core and tink.CoreApi are tink_core,
// tink.Json tink_json.
//
// Implements: REQ-HAXE-007
func table(segments []string) (library string, k int, std bool) {
	for n := min(len(segments), 4); n >= 1; n-- {
		if l, ok := known[strings.Join(segments[:n], ".")]; ok && l != "" {
			return l, n, false
		}
	}
	if segments[0] == "tink" && len(segments) > 1 {
		s := strings.ToLower(segments[1])
		if l, ok := tinkSpecial[s]; ok {
			return l, 2, false
		}
		return "tink_" + strings.TrimSuffix(s, "api"), 2, false
	}
	if stdPackages[segments[0]] || stdTypes[segments[0]] {
		return "", 1, true
	}
	return "", 0, false
}

// stdName is the standard library package a module belongs to: the first two
// segments when both are packages (haxe.ds, js.html), else the first (haxe
// for haxe.Json, StringTools for itself).
func stdName(segments []string) string {
	if len(segments) > 2 && lower(segments[1]) {
		return segments[0] + "." + segments[1]
	}
	return segments[0]
}

package ocaml

import "strings"

// stdModules are the modules of OCaml's standard library (4.14 to 5.4, with ones
// since removed that old code still names) and of the libraries the compiler ships
// with it, by the library they are in: stdlib is always linked; unix, str,
// threads, dynlink and runtime_events have to be named in libraries.
var stdModules = map[string]string{}

func init() {
	for _, m := range strings.Fields(`Stdlib Arg Array ArrayLabels Atomic Bigarray Bool Buffer Bytes BytesLabels
		Callback Char Complex Condition Digest Domain Dynarray Effect Either Ephemeron Filename Float
		Format Fun Gc Hashtbl In_channel Int Int32 Int64 Lazy Lexing List ListLabels Map Marshal
		MoreLabels Mutex Nativeint Obj Oo Option Out_channel Pair Parsing Printexc Printf Pqueue Queue
		Random Repr Result Scanf Semaphore Seq Set Stack StdLabels String StringLabels Sys Type Uchar
		Unit Weak Iarray Genlex Stream Pervasives Sort CamlinternalFormat CamlinternalFormatBasics
		CamlinternalLazy CamlinternalMod CamlinternalOO Std_exit`) {
		stdModules[m] = "stdlib"
	}
	for m, library := range map[string]string{
		"Unix": "unix", "UnixLabels": "unix", "Str": "str", "Thread": "threads", "Event": "threads",
		"ThreadUnix": "threads", "Dynlink": "dynlink", "Runtime_events": "runtime_events",
		"Graphics": "graphics", "Num": "num", "Big_int": "num", "Ratio": "num", "Arith_status": "num",
	} {
		stdModules[m] = library
	}
}

// stdLibraries are the findlib libraries the compiler installs: a dune libraries
// entry or #require naming one is the standard library, not a package.
var stdLibraries = map[string]bool{
	"stdlib": true, "unix": true, "str": true, "threads": true, "dynlink": true, "bigarray": true,
	"runtime_events": true, "compiler-libs": true, "ocamldoc": true, "raw_spacetime": true,
	"std_exit": true, "ocaml": true,
}

// compilerLibraries are the modules of compiler-libs a program reads OCaml with. Many
// projects have modules of these names of their own, so they are the standard
// library only for a component that uses compiler-libs, and ppxlib's (which
// re-exports them) for one that uses ppxlib.
var compilerLibraries = map[string]bool{}

func init() {
	for _, m := range strings.Fields(`Parsetree Asttypes Ast_helper Ast_mapper Ast_iterator Location
		Longident Pprintast Printast Parse Lexer Syntaxerr Typedtree Types Typemod Typecore Env Ident
		Path Predef Printtyp Clflags Compenv Compmisc Config Misc Warnings Toploop Topdirs Outcometree
		Docstrings Builtin_attributes Lambda Cmi_format Cmt_format Load_path Persistent_env Tast_iterator
		Tast_mapper Untypeast Btype Ctype Oprint Main_args`) {
		compilerLibraries[m] = true
	}
}

// stdlibReplacements are packages whose top module a file opens to use them in
// place of the standard library (open Core, open Base): after such an open, List
// or String is theirs.
var stdlibReplacements = map[string]bool{
	"base": true, "core": true, "core_kernel": true, "batteries": true, "containers": true,
	"stdcompat": true, "extlib": true,
}

// libraryPackages names the opam package of findlib libraries not named after
// their package's name up to the first dot.
var libraryPackages = map[string]string{
	"findlib": "ocamlfind", "zip": "camlzip", "oUnit": "ounit", "oUnitAdvanced": "ounit",
	"ctypes.foreign": "ctypes-foreign", "bytes": "base-bytes", "graph": "ocamlgraph",
}

// modulePackages names the package of top modules not named after their library
// or package (Z is zarith's) and of common libraries' modules, for a module a
// component uses without declaring the library.
var modulePackages = map[string]string{
	"Z": "zarith", "Q": "zarith", "Soup": "lambdasoup", "Graph": "ocamlgraph", "MenhirLib": "menhirLib",
	"Sedlexing": "sedlex", "OUnit": "ounit", "OUnit2": "ounit2", "QCheck": "qcheck-core", "QCheck2": "qcheck-core",
	"QCheck_alcotest": "qcheck-alcotest", "QCheck_base_runner": "qcheck-core", "Sexplib": "sexplib",
	"Sexplib0": "sexplib0", "Sexp": "sexplib", "Findlib": "ocamlfind", "Fl_dynload": "ocamlfind",
	"Toml": "toml", "Yaml": "yaml", "Ezjsonm": "ezjsonm", "Jsonm": "jsonm", "Yojson": "yojson",
	"Cmdliner": "cmdliner", "Fmt": "fmt", "Fmt_tty": "fmt", "Fmt_cli": "fmt", "Logs": "logs",
	"Logs_fmt": "logs", "Logs_lwt": "logs", "Logs_cli": "logs", "Re": "re", "Re_str": "re", "Re_pcre": "re",
	"Lwt": "lwt", "Lwt_unix": "lwt", "Lwt_io": "lwt", "Lwt_main": "lwt", "Lwt_list": "lwt",
	"Lwt_stream": "lwt", "Lwt_mutex": "lwt", "Lwt_condition": "lwt", "Lwt_result": "lwt",
	"Lwt_process": "lwt", "Lwt_bytes": "lwt", "Lwt_mvar": "lwt", "Lwt_pool": "lwt", "Lwt_switch": "lwt",
	"Lwt_seq": "lwt", "Lwt_preemptive": "lwt", "Lwt_engine": "lwt", "Lwt_gc": "lwt", "Lwt_sequence": "lwt",
	"Alcotest": "alcotest", "Alcotest_lwt": "alcotest-lwt", "Alcotest_engine": "alcotest",
	"Core": "core", "Core_kernel": "core_kernel", "Core_unix": "core_unix", "Command_unix": "core_unix",
	"Base": "base", "Stdio": "stdio", "Async": "async", "Async_kernel": "async_kernel",
	"Async_unix": "async_unix", "Ppxlib": "ppxlib", "Ppx_deriving": "ppx_deriving",
	"Ppx_deriving_runtime": "ppx_deriving", "Uri": "uri", "Cstruct": "cstruct", "Angstrom": "angstrom",
	"Faraday": "faraday", "Bigstringaf": "bigstringaf", "Astring": "astring", "Fpath": "fpath",
	"Bos": "bos", "Rresult": "rresult", "Ptime": "ptime", "Ptime_clock": "ptime", "Mtime": "mtime",
	"Mtime_clock": "mtime", "Uutf": "uutf", "Uucp": "uucp", "Uunf": "uunf", "Uuseg": "uuseg",
	"Uuidm": "uuidm", "Digestif": "digestif", "Base64": "base64", "Hex": "hex", "Eqaf": "eqaf",
	"Ipaddr": "ipaddr", "Macaddr": "macaddr", "Domain_name": "domain-name", "Cohttp": "cohttp",
	"Cohttp_lwt": "cohttp-lwt", "Cohttp_lwt_unix": "cohttp-lwt-unix", "Cohttp_async": "cohttp-async",
	"Cohttp_eio": "cohttp-eio", "Conduit": "conduit", "Conduit_lwt_unix": "conduit-lwt-unix",
	"Eio": "eio", "Eio_main": "eio_main", "Eio_unix": "eio", "Dream": "dream", "Opium": "opium",
	"Tyxml": "tyxml", "Js_of_ocaml": "js_of_ocaml", "Js_of_ocaml_lwt": "js_of_ocaml-lwt", "Brr": "brr",
	"Irmin": "irmin", "Git": "git", "Caqti": "caqti", "Caqti_lwt": "caqti-lwt", "Caqti_type": "caqti",
	"Caqti_request": "caqti", "Sqlite3": "sqlite3", "Postgresql": "postgresql", "Mysql": "mysql",
	"Batteries": "batteries", "BatList": "batteries", "BatString": "batteries", "Containers": "containers",
	"CCList": "containers", "CCString": "containers", "CCOpt": "containers", "CCHashtbl": "containers",
	"CCVector": "containers", "CCIO": "containers", "CCFormat": "containers", "Iter": "iter",
	"Gen": "gen", "Ctypes": "ctypes", "Foreign": "ctypes-foreign", "Stdint": "stdint",
	"Integers": "integers", "Hmap": "hmap", "Notty": "notty", "Notty_unix": "notty",
	"LTerm": "lambda-term", "Zed": "zed", "Xmlm": "xmlm", "Markup": "markup", "Csv": "csv",
	"Camlzip": "camlzip", "Zip": "camlzip", "Gzip": "camlzip", "Zlib": "camlzip", "Decompress": "decompress",
	"Checkseum": "checkseum", "Tls": "tls", "X509": "x509", "Mirage_crypto": "mirage-crypto",
	"Mirage_crypto_rng": "mirage-crypto-rng", "Ca_certs": "ca-certs", "Ssl": "ssl", "Lwt_ssl": "lwt_ssl",
	"Ounit2": "ounit2", "Crowbar": "crowbar", "Bisect": "bisect_ppx", "Dune_build_info": "dune-build-info",
	"Dune_site": "dune-site", "Odoc_model": "odoc", "Merlin_kernel": "merlin-lib", "Printbox": "printbox",
	"Progress": "progress", "Terminal": "terminal", "Stdune": "stdune", "Ocamlformat_lib": "ocamlformat-lib",
	"Lsp": "lsp", "Jsonrpc": "jsonrpc", "Graphics": "graphics", "Num": "num", "Unix_errno": "unix-errno",
	"Expect_test_helpers_core": "expect_test_helpers_core", "Base_quickcheck": "base_quickcheck",
	"Core_bench": "core_bench", "Ppx_inline_test_lib": "ppx_inline_test", "Ppx_expect": "ppx_expect",
	"Ppx_sexp_conv_lib": "ppx_sexp_conv", "Ppx_yojson_conv_lib": "ppx_yojson_conv_lib", "Repr": "repr",
	"Fiber": "fiber", "Spawn": "spawn", "Ordering": "ordering", "Dyn": "dyn", "Csexp": "csexp",
	"Pp": "pp", "Chrome_trace": "chrome-trace", "Xdg": "xdg", "Optint": "optint", "Ke": "ke",
	"Fmt_lwt": "fmt", "Lru": "lru", "Psq": "psq", "Hashset": "hashset", "Bheap": "bheap",
	"Mirage_kv": "mirage-kv", "Mirage_clock": "mirage-clock", "Mirage_time": "mirage-time",
	"Lwt_eio": "lwt_eio", "Domainslib": "domainslib", "Saturn": "saturn", "Kcas": "kcas",
	"Ppx_blob": "ppx_blob", "Ppx_let": "ppx_let", "Ppx_repr_lib": "ppx_repr", "Index": "index",
	"Cmarkit": "cmarkit", "Omd": "omd", "Cmdliner_term": "cmdliner", "Jingoo": "jingoo",
	"Irmin_pack": "irmin-pack", "Irmin_git": "irmin-git", "Irmin_test": "irmin-test",
}

// fold makes library, package and module names comparable: Cohttp_lwt_unix,
// cohttp-lwt-unix and cohttp-lwt.unix all fold to cohttp_lwt_unix.
func fold(s string) string {
	return strings.ToLower(folder.Replace(s))
}

var folder = strings.NewReplacer("-", "_", ".", "_")

// libraryPackage is the opam package providing a findlib library: the part before
// the first dot, unless the table says otherwise. "" is the compiler's own.
func libraryPackage(library string) string {
	if p, ok := libraryPackages[library]; ok {
		return p
	}
	first, _, _ := strings.Cut(library, ".")
	if stdLibraries[first] {
		return ""
	}
	if p, ok := libraryPackages[first]; ok {
		return p
	}
	return first
}

// stdLibrary is the standard library a findlib library is part of, "" if none.
func stdLibrary(library string) string {
	first, _, _ := strings.Cut(library, ".")
	if stdLibraries[first] {
		return first
	}
	return ""
}

// provides reports how well a library provides a module: 2 for a match by name
// (the library's main module, Lwt_unix of lwt.unix), 1 for a module prefixed by
// the package's name (Lwt_io of lwt.unix), 0 for none.
func provides(library, module string) int {
	packageName := libraryPackage(library)
	if packageName == "" {
		return 0
	}
	if p, ok := modulePackages[module]; ok {
		if p == packageName {
			return 2
		}
		return 0
	}
	m := fold(module)
	last := library
	if i := strings.LastIndexByte(library, '.'); i >= 0 {
		last = library[i+1:]
	}
	if m == fold(library) || m == fold(last) || m == fold(packageName) {
		return 2
	}
	if strings.HasPrefix(m, fold(packageName)+"_") {
		return 1
	}
	return 0
}

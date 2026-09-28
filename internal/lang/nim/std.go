package nim

import "strings"

// stdModules are the modules of Nim's standard library a bare import names
// (`import os, strutils`): the modules of lib/pure, lib/core, lib/impure,
// lib/wrappers, lib/posix, lib/windows, lib/js, lib/deprecated/pure and
// lib/pure/collections and concurrency (the compiler's stdlibDirs), from Nim
// 2.2's sources, plus system and modules of older releases.
var stdModules = func() map[string]bool {
	m := map[string]bool{}
	for _, name := range strings.Fields(`
		algorithm async asyncdispatch asyncfile asyncfutures asynchttpserver asyncjs asyncmacro
		asyncnet asyncstreams atomics base64 bitops browsers cgi chains colors complex cookies coro
		cpuinfo cpuload critbits cstrutils deques distros dom dynlib encodings endians epoll fenv
		future hashcommon hashes heapqueue hotcodereloading htmlgen htmlparser httpclient httpcore
		inotify intsets jsconsole jscore jsffi json jsre kqueue lenientops lexbase linenoise linux
		lists locks logging macrocache macros marshal math md5 memfiles mersenne mimetypes
		nativesockets net nimprof nre oids openssl options os ospaths osproc oswalkdir parsecfg
		parsecsv parsejson parseopt parsesql parseutils parsexml pathnorm pcre pegs posix
		posix_utils prelude random rationals rdstdin re registry reservedmem rlocks ropes rtarrays
		segfaults selectors sequtils setimpl sets sharedlist sharedtables ssl_certs ssl_config stats
		streams streamwrapper strformat strmisc strscans strtabs strutils sugar sums tableimpl
		tables terminal termios threadpool times tinyc typeinfo typetraits unicode unidecode
		unittest uri volatile winlean xmlparser xmltree
		system nimhcr nimrtl threads channels_builtin
		asyncftpclient smtp punycode db_common db_mysql db_postgres db_sqlite db_odbc events
		securehash sharedstrings subexes scgi fsmonitor matchers parseurl sockets rawsockets
		asyncio ftpclient httpserver actors oids
	`) {
		m[name] = true
	}
	return m
}()

// stdPrefixes are directories of the library an import may name with their
// path: experimental/diff, packages/docutils/rst, system/ansi_c.
var stdPrefixes = []string{"experimental/", "packages/docutils/", "system/", "pure/", "impure/", "wrappers/", "posix/", "windows/", "deprecated/"}

// movedModules are standard modules that left the library for a nimble package
// in Nim 2: a project that declares the package imports it from there.
var movedModules = map[string]string{
	"db_common": "db_connector", "db_mysql": "db_connector", "db_postgres": "db_connector",
	"db_sqlite": "db_connector", "db_odbc": "db_connector",
	"asyncftpclient": "asyncftpclient", "smtp": "smtp", "punycode": "punycode",
	"parsesql": "parsesql",
}

// stdlibDirs are the directories under the library root the compiler searches
// for a bare module name and for std/x, in its order (compiler/options.nim).
var stdlibDirs = []string{
	"pure", "core", "arch", "pure/collections", "pure/concurrency", "pure/unidecode", "impure",
	"wrappers", "wrappers/linenoise", "windows", "posix", "js", "deprecated/pure",
}

// isStd reports whether a bare module path is the standard library's.
func isStd(module string) bool {
	if stdModules[module] {
		return true
	}
	for _, p := range stdPrefixes {
		if strings.HasPrefix(module, p) {
			return true
		}
	}
	return false
}

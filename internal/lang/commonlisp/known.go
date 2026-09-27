package commonlisp

import "strings"

// knownPackages maps a package name whose system is named differently to
// that system. A package the table does not list is looked for among the
// declared systems by its own name.
var knownPackages = map[string]string{
	"bt": "bordeaux-threads", "bt2": "bordeaux-threads", "bordeaux-threads-2": "bordeaux-threads",
	"ppcre": "cl-ppcre", "dex": "dexador", "json": "cl-json", "jzon": "com.inuoe.jzon",
	"5am": "fiveam", "it.bese.fiveam": "fiveam", "cl-test-more": "prove", "test-more": "prove",
	"lt": "local-time", "flex": "flexi-streams", "base64": "cl-base64",
	"iter": "iterate", "fad": "cl-fad", "c2mop": "closer-mop", "c2cl": "closer-mop",
	"c2cl-user": "closer-mop", "interpol": "cl-interpol", "tbnl": "hunchentoot",
	"crypto": "ironclad", "log": "log4cl", "bind": "metabang-bind", "metabang.bind": "metabang-bind",
	"who": "cl-who", "dbi": "cl-dbi", "dbi.driver": "cl-dbi", "pomo": "postmodern",
	"yaml": "cl-yaml", "mustache": "cl-mustache", "as": "cl-async", "bb": "blackbird",
	"annot": "cl-annot", "syntax": "cl-syntax", "tg": "trivial-garbage",
	"org.mapcar.parse-number": "parse-number", "clim": "mcclim", "clim-lisp": "mcclim",
	"docs": "documentation-utils", "editor-hints.named-readtables": "named-readtables",
	"alexandria-2": "alexandria", "str": "str", "cl-str": "str",
	"cffi-sys": "cffi", "cffi-features": "cffi", "babel-encodings": "babel",
	"flexi-streams": "flexi-streams", "usocket": "usocket", "cl+ssl": "cl+ssl",
	"sqlite": "sqlite", "zip": "zip", "chipz": "chipz", "salza2": "salza2",
	"plump": "plump", "clss": "clss", "lquery": "lquery", "lparallel": "lparallel",
	"trivia": "trivia", "optima": "optima", "serapeum": "serapeum", "esrap": "esrap",
	"swank": "swank", "slynk": "slynk", "quri": "quri", "fast-http": "fast-http",
	"yason": "yason", "jsown": "jsown", "st-json": "st-json", "shasht": "shasht",
	"cl-json": "cl-json", "jonathan": "jonathan", "parachute": "parachute", "rove": "rove",
	"prove": "prove", "fiveam": "fiveam", "lisp-unit": "lisp-unit", "1am": "1am",
	"clack": "clack", "lack": "lack", "ningle": "ningle", "caveman2": "caveman2",
	"djula": "djula", "spinneret": "spinneret", "sxql": "sxql", "mito": "mito",
	"drakma": "drakma", "hunchentoot": "hunchentoot", "woo": "woo", "chunga": "chunga",
	"puri": "puri", "cxml": "cxml", "xmls": "xmls", "uuid": "uuid", "md5": "md5",
	"anaphora": "anaphora", "let-plus": "let-plus", "named-readtables": "named-readtables",
	"split-sequence": "split-sequence", "cl-cron": "cl-cron", "cl-smtp": "cl-smtp",
	"trivial-features": "trivial-features", "global-vars": "global-vars", "atomics": "atomics",
	"static-vectors": "static-vectors", "fast-io": "fast-io", "nibbles": "nibbles",
	"cl-base64": "cl-base64", "cl-fad": "cl-fad", "osicat": "osicat", "cl-async": "cl-async",
	"float-features": "float-features", "parse-number": "parse-number", "queues": "queues",
	"chanl": "chanl", "calispel": "calispel", "cl-colors": "cl-colors", "3bmd": "3bmd",
	"cl-markdown": "cl-markdown", "local-time": "local-time", "ironclad": "ironclad",
	"closer-mop": "closer-mop", "iterate": "iterate", "cl-who": "cl-who",
	"cl-interpol": "cl-interpol", "cl-unicode": "cl-unicode", "log4cl": "log4cl",
	"postmodern": "postmodern", "cl-postgres": "cl-postgres", "s-sql": "s-sql",
	"cl-yaml": "cl-yaml", "cl-mustache": "cl-mustache", "cl-annot": "cl-annot",
	"cl-syntax": "cl-syntax", "trivial-garbage": "trivial-garbage",
	"documentation-utils": "documentation-utils", "mcclim": "mcclim",
	"metabang-bind": "metabang-bind", "cl-dbi": "cl-dbi", "com.inuoe.jzon": "com.inuoe.jzon",
	"hu.dwim.stefil": "hu.dwim.stefil", "stefil": "stefil", "clunit": "clunit", "clunit2": "clunit2",
	"cl-ppcre": "cl-ppcre", "alexandria": "alexandria", "bordeaux-threads": "bordeaux-threads",
	"dexador": "dexador", "cffi": "cffi", "babel": "babel",
	"mimes": "trivial-mimes", "org.shirakumo.trivial-mimes": "trivial-mimes",
}

// implementationModules are modules require loads from an implementation
// rather than systems: CMUCL's and ABCL's Gray streams, ABCL's contribs,
// Allegro's modules.
var implementationModules = map[string]string{
	"gray-streams": "implementation", "clx": "implementation", "defsystem": "implementation",
	"abcl-contrib": "abcl", "jss": "abcl", "asdf-jar": "abcl", "java": "abcl",
	"osi": "allegro", "acache": "allegro", "aserve": "allegro", "regexp2": "allegro", "sock": "allegro",
	"cmucl-contribs": "cmucl", "unix": "cmucl", "simple-streams": "implementation",
}

// knownProjects maps a system to the Quicklisp project that releases it
// when the names differ: most projects release one system of their own name,
// or secondary systems named project/part, which need no entry.
var knownProjects = map[string]string{
	"cl-ppcre-unicode": "cl-ppcre", "cl-ppcre-test": "cl-ppcre",
	"fiveam-asdf": "fiveam", "prove-asdf": "prove", "cl-test-more": "prove",
	"str": "cl-str", "usocket-server": "usocket",
	"cffi-grovel": "cffi", "cffi-libffi": "cffi", "cffi-toolchain": "cffi", "cffi-uffi-compat": "cffi",
	"babel-streams": "babel", "cl+ssl": "cl-plus-ssl", "cl+ssl.test": "cl-plus-ssl",
	"swank": "slime", "swank-listener-hooks": "slime", "slynk": "sly",
	"optima.ppcre": "optima", "cl-postgres": "postmodern", "s-sql": "postmodern",
	"simple-date": "postmodern", "dbi": "cl-dbi", "mito-core": "mito", "mito-migration": "mito",
	"com.inuoe.jzon": "jzon", "log4cl.log4slime": "log4cl", "log4cl.log4sly": "log4cl",
	"sqlite": "cl-sqlite", "3bmd-ext-code-blocks": "3bmd", "3bmd-ext-tables": "3bmd",
	"clim": "mcclim", "clim-lisp": "mcclim", "clim-core": "mcclim", "cl-async-ssl": "cl-async",
	"cl-syntax-annot": "cl-syntax", "cl-syntax-interpol": "cl-syntax", "caveman2": "caveman",
	"cxml-stp": "cxml-stp", "closure-common": "closure-common", "lisp-unit2": "lisp-unit2",
	"yason": "yason", "metabang-bind": "metabang-bind", "cl-libyaml": "cl-libyaml",
	"trivial-utf-8": "trivial-utf-8", "hu.dwim.stefil": "hu.dwim.stefil",
	"lparallel": "lparallel", "cl-unicode": "cl-unicode", "flexi-streams": "flexi-streams",
}

// projectPrefixes map system name prefixes to their projects: clack's
// handlers, lack's middlewares, cl-dbi's drivers, trivia's parts.
var projectPrefixes = []struct{ prefix, project string }{
	{"clack-", "clack"}, {"lack-", "lack"}, {"dbd-", "cl-dbi"}, {"trivia.", "trivia"},
	{"queues.", "queues"}, {"cl-async-", "cl-async"}, {"mcclim-", "mcclim"}, {"clim-", "mcclim"},
	{"swank-", "slime"}, {"cffi-", "cffi"}, {"iolib/", "iolib"}, {"iolib.", "iolib"},
	{"hunchentoot-", "hunchentoot"}, {"drakma-", "drakma"}, {"cl-json.", "cl-json"},
}

// projectOf is the Quicklisp project that releases a system: a secondary
// system (ironclad/digests/sha256) is its primary system's, then the tables.
//
// Implements: REQ-COMMONLISP-007
func projectOf(sys string) string {
	primary, _, _ := strings.Cut(sys, "/")
	if p, ok := knownProjects[sys]; ok {
		return p
	}
	if p, ok := knownProjects[primary]; ok {
		return p
	}
	for _, pp := range projectPrefixes {
		if strings.HasPrefix(primary, pp.prefix) && primary != pp.project {
			return pp.project
		}
	}
	return primary
}

// packageSystem is the system a curated table names for a package: the
// package itself, then its dotted or slashed prefixes (lack.request is
// lack's, trivia.level2 trivia's).
func packageSystem(pkg string) (string, bool) {
	for p := pkg; p != ""; {
		if s, ok := knownPackages[p]; ok {
			return s, true
		}
		i := strings.LastIndexAny(p, "./")
		if i <= 0 {
			break
		}
		p = p[:i]
	}
	return "", false
}

// stdPackage is the node of the hidden cl-std island a package belongs to:
// the standard's COMMON-LISP (and CL-USER, KEYWORD), an implementation's own
// packages (SB-EXT, CCL, EXCL ...), ASDF and UIOP, which every implementation
// ships, and the Quicklisp client.
//
// Implements: REQ-COMMONLISP-008
func stdPackage(pkg string) (string, bool) {
	first, _, _ := strings.Cut(pkg, "/")
	switch first {
	case "cl", "common-lisp", "cl-user", "common-lisp-user", "keyword":
		return "common-lisp", true
	case "uiop":
		return "uiop", true
	case "asdf", "asdf-user":
		return "asdf", true
	case "ql", "quicklisp", "ql-dist", "ql-util", "ql-setup", "ql-impl", "ql-impl-util":
		return "quicklisp", true
	case "future-common-lisp", "scl", "symbolics-common-lisp", "zl":
		return "genera", true
	case "ccl", "openmcl":
		return "ccl", true
	case "excl", "net.uri", "excl.osi":
		return "allegro", true
	case "lispworks", "lw", "hcl", "capi":
		return "lispworks", true
	case "ext", "si", "sys", "ffi", "mp", "clos", "gray", "mop", "threads", "java", "jss", "custom", "socket", "posix", "gc":
		return "implementation", true
	}
	if strings.HasPrefix(first, "sb-") || strings.HasPrefix(first, "sb!") {
		return "sbcl", true
	}
	return "", false
}

// stdSystem is the cl-std node of a system an implementation provides:
// ASDF and UIOP, and SBCL's contrib modules (sb-posix, sb-bsd-sockets ...).
func stdSystem(sys string) (string, bool) {
	switch sys {
	case "asdf", "uiop", "asdf-package-system":
		if sys == "asdf-package-system" {
			return "asdf", true
		}
		return sys, true
	case "quicklisp":
		return "quicklisp", true
	}
	if strings.HasPrefix(sys, "sb-") {
		return "sbcl", true
	}
	return "", false
}

package racket

import "strings"

// baseModules lists the modules (and, for a directory, the module paths under
// it) of Racket's main collection tree, racket/collects, which the "base"
// package installs everywhere Racket is: two segments deep, private/
// directories left out (a private module under a base collection is base
// too). Taken from racket/racket at 9.x.
const baseModules = `
acks/acks compiler/cm compiler/cm-accomplice compiler/compilation-path compiler/compile-file
compiler/compiler compiler/cross compiler/depend compiler/distribute compiler/embed
compiler/exe-dylib-path compiler/find-exe compiler/module-suffix compiler/option data/bit-vector
data/integer-set data/queue dynext/file dynext/filename-version ffi/com ffi/com-registry ffi/cvector
ffi/file ffi/objc ffi/unsafe ffi/vcruntime ffi/vector ffi/winapi file/cache file/convertible
file/glob file/gunzip file/gzip file/ico file/md5 file/resource file/sha1 file/tar file/untar
file/untgz file/unzip file/zip info/main json/main launcher/info launcher/launcher launcher/main
net/base64 net/git-checkout net/head net/http-client net/osx-ssl net/platform-ssl net/uri-codec
net/url net/url-connect net/url-exception net/url-string net/url-structs net/win32-ssl
openssl/legacy openssl/libcrypto openssl/libssl openssl/main openssl/md5 openssl/mzssl
openssl/openssl openssl/sha1 pkg/commands pkg/db pkg/dirs-catalog pkg/info pkg/lib pkg/main pkg/name
pkg/path pkg/raco pkg/strip planet/cachepath planet/config planet/planet-archives planet/resolver
planet/terse-info racket/async-channel racket/base racket/block racket/bool racket/bytes racket/case
racket/class racket/cmdline racket/contract racket/control racket/date racket/deprecation
racket/dict racket/engine racket/enter racket/exn racket/extflonum racket/fasl racket/file
racket/fixnum racket/flonum racket/for-clause racket/format racket/function racket/future
racket/generator racket/generic racket/gui/dynamic racket/hash racket/hash-code racket/help
racket/include racket/info racket/init racket/interaction-info racket/interactive racket/kernel
racket/keyword racket/keyword-transform racket/lang racket/language-info racket/lazy-require
racket/linklet racket/list racket/load racket/local racket/logging racket/main racket/match
racket/math racket/mutability racket/mutable-treelist racket/os racket/path racket/performance-hint
racket/phase+space racket/place racket/port racket/prefab racket/pretty racket/promise
racket/provide racket/provide-syntax racket/provide-transform racket/random racket/repl
racket/require racket/require-syntax racket/require-transform racket/rerequire racket/runtime-config
racket/runtime-path racket/sequence racket/serialize racket/serialize-structs racket/set
racket/shared racket/signature racket/splicing racket/stream racket/string racket/struct
racket/struct-info racket/stxparam racket/stxparam-exptime racket/surrogate racket/symbol
racket/syntax racket/syntax-srcloc racket/system racket/tcp racket/trace racket/trait
racket/treelist racket/udp racket/undefined racket/unit racket/unit-exptime racket/unreachable
racket/unsafe racket/vector raco/all-tools raco/command-name raco/info raco/main raco/raco
reader/lang s-exp/lang setup/collection-name setup/collection-search setup/collects setup/commands
setup/cross-system setup/dirs setup/doc-db setup/getinfo setup/info setup/infotab
setup/language-family setup/link setup/main setup/main-collects setup/main-doc
setup/matching-platform setup/option setup/pack setup/parallel-build setup/parallel-do
setup/path-relativize setup/path-to-relative setup/plt-single-installer setup/setup
setup/setup-cmdline setup/setup-core setup/setup-go setup/unixstyle-install setup/unpack
setup/variant setup/winstrip setup/winvers setup/winvers-change syntax/apply-transformer
syntax/boundmap syntax/context syntax/contract syntax/datum syntax/define syntax/docprovide
syntax/flatten-begin syntax/for-body syntax/free-vars syntax/id-set syntax/id-table syntax/intdef
syntax/kerncase syntax/keyword syntax/location syntax/macro-testing syntax/modcode
syntax/modcollapse syntax/moddep syntax/modread syntax/modresolve syntax/module-reader syntax/name
syntax/parse syntax/path-spec syntax/primitives syntax/quote syntax/readerr syntax/srcloc
syntax/strip-context syntax/struct syntax/stx syntax/template syntax/to-string syntax/toplevel
syntax/transformer syntax/trusted-xforms syntax/unsafe syntax/wrap-modbeg version/check
version/patchlevel version/utils xml/main xml/path xml/plist xml/xexpr xml/xml
`

var (
	baseSet  = map[string]bool{}
	baseTops = map[string]bool{}
)

func init() {
	for _, m := range strings.Fields(baseModules) {
		baseSet[m] = true
		first, _, _ := strings.Cut(m, "/")
		baseTops[first] = true
	}
}

// baseModule reports whether a collection-based module path is one the base
// package provides.
//
// Implements: REQ-RACKET-007
func baseModule(segs []string) bool {
	if !baseTops[segs[0]] {
		return false
	}
	if len(segs) == 1 {
		return baseSet[segs[0]+"/main"]
	}
	return segs[1] == "private" || baseSet[segs[0]+"/"+segs[1]]
}

// knownCollections maps collection paths (the longest listed prefix of a
// module path wins) to the catalog package that provides them, for the main
// distribution's packages whose names do not simply spell the collection
// (rackunit is in rackunit-lib, typed/racket in typed-racket-lib) and for
// collections split between base and other packages (net/smtp is net-lib's,
// racket/gui gui-lib's).
//
// Implements: REQ-RACKET-007
var knownCollections = map[string]string{
	"racket/gui": "gui-lib", "racket/draw": "draw-lib", "racket/snip": "snip-lib", "racket/sandbox": "sandbox-lib",
	"racket/gui/dynamic": "base",
	"mred":               "gui-lib", "framework": "gui-lib", "mrlib": "gui-lib", "embedded-gui": "gui-lib",
	"typed/racket": "typed-racket-lib", "typed-racket": "typed-racket-lib", "typed": "typed-racket-more",
	"typed-scheme": "typed-racket-compatibility",
	"rackunit":     "rackunit-lib", "rackunit/gui": "rackunit-gui",
	"scribble": "scribble-lib", "scribblings": "racket-doc",
	"plot": "plot-lib", "pict": "pict-lib", "texpict": "pict-lib", "slideshow": "slideshow-lib",
	"web-server": "web-server-lib", "db": "db-lib", "data": "data-lib",
	"2htdp": "htdp-lib", "htdp": "htdp-lib", "lang": "htdp-lib", "teachpack": "htdp-lib", "test-engine": "htdp-lib",
	"threading": "threading-lib", "net": "net-lib", "syntax/color": "syntax-color-lib",
	"math": "math-lib", "images": "images-lib", "redex": "redex-lib", "parser-tools": "parser-tools-lib",
	"srfi": "srfi-lib", "mzlib": "compatibility-lib", "compatibility": "compatibility-lib",
	"scheme": "scheme-lib", "mzscheme": "scheme-lib", "xrepl": "xrepl-lib",
	"macro-debugger": "macro-debugger-text-lib", "string-constants": "string-constants-lib",
	"unstable": "unstable-lib", "profile": "profile-lib", "errortrace": "errortrace-lib",
	"html": "html-lib", "at-exp": "at-exp-lib", "wxme": "wxme-lib", "compiler": "compiler-lib",
	"setup/xref": "racket-index", "file/gif": "draw-lib",
	"graph": "graph-lib", "gregor": "gregor-lib", "sgl": "sgl", "frtime": "frtime", "lazy": "lazy",
	"datalog": "datalog", "racklog": "racklog", "deinprogramm": "deinprogramm", "pollen": "pollen",
	"drracket": "drracket", "syntax-color": "syntax-color-lib", "simple-tree-text-markup": "simple-tree-text-markup-lib",
	"readline": "readline-lib", "ffi2": "ffi2-lib", "2d": "2d-lib", "expeditor": "expeditor-lib",
	"pconvert": "pconvert-lib", "version/check": "base",
	"openssl/libcrypto": "base", "file/sha1": "base",
	"typed/rackunit": "rackunit-typed", "typed/test-engine": "htdp-lib", "rnrs": "r6rs-lib", "r6rs": "r6rs-lib",
	"scriblib": "scribble-lib", "net/cookies": "net-cookies-lib", "json/format": "json-format",
}

// knownPackage returns the catalog package of a module path from
// knownCollections: the longest listed prefix.
func knownPackage(segs []string) string {
	for k := min(len(segs), 3); k > 0; k-- {
		if p, ok := knownCollections[strings.Join(segs[:k], "/")]; ok {
			return p
		}
	}
	return ""
}

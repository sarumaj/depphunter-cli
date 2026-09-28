package commonlisp

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// The fixture is a system "shop" (shop.asd: :depends-on with (:version),
// (:feature) and (:require) entries, entries hidden behind #+nil and #+(or),
// a :pathname, nested :module components, a module with a :pathname of its
// own, a static file, a cffi-grovel file, a component that does not exist,
// and the secondary systems shop/core and shop/tests). Its qlfile and
// qlfile.lock pin alexandria and cl-ppcre by dist version, dexador (github)
// and str (git) by commit and a private tree-sitter-cl by commit; every other
// Quicklisp project comes from the lock's quicklisp dist. src/package.lisp
// defines the package shop with :use, :import-from, :shadowing-import-from,
// :local-nicknames and :nicknames; the model files are in-package shop and
// write qualified symbols (through the local nicknames too); the reader file
// hides fake forms in comments, strings, characters, quoted data and false
// reader conditionals. pis/ is a package-inferred system whose packages are
// its files; ocicl-app/ is pinned by its ocicl.csv (its systems/ directory is
// ocicl's); tools/ has a qlfile without a lock (ql :all, git refs, a tag, a
// branch, an http tarball, ultralisp, a local directory). scripts/ has a
// build script quickloading and requiring systems, opencl/kernel.cl is an
// OpenCL kernel (not claimed) and lisp/ has .cl and .lsp sources.

var (
	alexandria      = lang.Target{Ecosystem: ecosystemQuicklisp, Package: "alexandria", Version: "2023-10-21", Requested: "latest", Pinned: true}
	ppcre           = lang.Target{Ecosystem: ecosystemQuicklisp, Package: "cl-ppcre", Version: "2023-06-18", Pinned: true}
	dexador         = lang.Target{Ecosystem: ecosystemQuicklisp, Package: "github.com/fukamachi/dexador", Version: "1c2b3a4d5e6f708192a3b4c5d6e7f8091a2b3c4d", Requested: "master", Pinned: true}
	str             = lang.Target{Ecosystem: ecosystemQuicklisp, Package: "github.com/vindarel/cl-str", Version: "aaaabbbbccccddddeeeeffff0000111122223333", Requested: "0.21", Pinned: true}
	treeSitter      = lang.Target{Ecosystem: ecosystemQuicklisp, Package: "git.acme.dev/lisp/tree-sitter-cl", Version: "0123456789abcdef0123456789abcdef01234567", Pinned: true, Origin: "https://git.acme.dev/lisp/tree-sitter-cl.git"}
	ociclAlexandria = lang.Target{Ecosystem: ecosystemQuicklisp, Package: "alexandria", Version: "20240503-8514d8e", Pinned: true}
	ociclPpcre      = lang.Target{Ecosystem: ecosystemQuicklisp, Package: "cl-ppcre", Version: "20240423-80fb19d", Pinned: true}
)

func local(p string) lang.Target { return lang.Target{Local: p} }
func std(p string) lang.Target   { return lang.Target{Ecosystem: ecosystemStd, Package: p} }

// dist is a Quicklisp project the lock's quicklisp dist pins.
func dist(p string) lang.Target {
	return lang.Target{Ecosystem: ecosystemQuicklisp, Package: p, Version: "2023-10-21", Pinned: true}
}

func analyze(t *testing.T) map[string]*lang.FileResult {
	return langtest.Analyze(t, Plugin{}, "testdata/repo")
}

// Verifies: REQ-COMMONLISP-002, REQ-COMMONLISP-005, REQ-COMMONLISP-007
func TestSystems(t *testing.T) {
	results := analyze(t)
	langtest.CheckImports(t, results["shop.asd"], map[string]lang.Target{
		"use cl":                                 std("common-lisp"),
		"use asdf":                               std("asdf"),
		"in-package shop-asd":                    {},
		"defsystem-depends-on cffi-grovel":       dist("cffi"),
		"depends-on alexandria":                  alexandria,
		"depends-on cl-ppcre":                    ppcre,
		"depends-on sb-posix":                    std("sbcl"),
		"depends-on osicat":                      dist("osicat"),
		"require sb-introspect":                  std("sbcl"),
		"depends-on shop/core":                   {},
		"depends-on uiop":                        std("uiop"),
		"depends-on dexador":                     dexador,
		"depends-on str":                         str,
		"depends-on cl-json":                     dist("cl-json"),
		"depends-on bordeaux-threads":            dist("bordeaux-threads"),
		"depends-on tree-sitter-cl":              treeSitter,
		"depends-on cl-ppcre-unicode":            ppcre,
		"component src/package.lisp":             local("src/package.lisp"),
		"component src/model/item.lisp":          local("src/model/item.lisp"),
		"component src/model/cart.lisp":          local("src/model/cart.lisp"),
		"component src/input-output/reader.lisp": local("src/input-output/reader.lisp"),
		"component README.md":                    local("README.md"),
		"component src/grovel.lisp":              local("src/grovel.lisp"),
		"component src/old/legacy.lisp":          local("src/old/legacy.lisp"),
		"component src/missing.lisp":             {},
		"depends-on local-time":                  dist("local-time"),
		"component src/core.lisp":                local("src/core.lisp"),
		"depends-on shop":                        {},
		"depends-on rove":                        dist("rove"),
		"component t/main.lisp":                  local("t/main.lisp"),
	})
	langtest.CheckImports(t, results["pis/acme.asd"], map[string]lang.Target{
		"depends-on acme/main": local("pis/src/main.lisp"),
		"asdf:":                std("asdf"),
	})
	langtest.CheckImports(t, results["ocicl-app/app.asd"], map[string]lang.Target{
		"depends-on alexandria":       ociclAlexandria,
		"depends-on cl-ppcre-unicode": ociclPpcre,
		"depends-on fiveam":           {Ecosystem: ecosystemQuicklisp, Package: "fiveam", Version: ">= 1.4", Floating: true},
		"component app.lisp":          local("ocicl-app/app.lisp"),
	})
	langtest.CheckImports(t, results["tools/tools.asd"], map[string]lang.Target{
		"depends-on fiveam":                  {Ecosystem: ecosystemQuicklisp, Package: "fiveam", Floating: true},
		"depends-on bar":                     {Ecosystem: ecosystemQuicklisp, Package: "github.com/foo/bar", Version: "89abcdef0123456789abcdef0123456789abcdef", Pinned: true},
		"depends-on widgets":                 {Ecosystem: ecosystemQuicklisp, Package: "github.com/acme/widgets", Version: "v1.2"},
		"depends-on gitlib":                  {Ecosystem: ecosystemQuicklisp, Package: "gitlab.com/x/gitlib", Version: "dev", Floating: true},
		"depends-on yason":                   {Ecosystem: ecosystemQuicklisp, Package: "yason", Version: "0123456789abcdef0123456789abcdef", Origin: "http://netzhansa.com/yason.tar.gz", Pinned: true},
		"depends-on cl-foo":                  {Ecosystem: ecosystemQuicklisp, Package: "cl-foo", Floating: true},
		"depends-on shop-vendor":             local("third_party/shop-vendor"),
		"depends-on local-time":              {Ecosystem: ecosystemQuicklisp, Package: "local-time", Version: "2022-01-01", Pinned: true},
		"depends-on ironclad/digests/sha256": {Ecosystem: ecosystemQuicklisp, Package: "ironclad", Version: "2022-01-01", Pinned: true},
	})
}

// Verifies: REQ-COMMONLISP-002, REQ-COMMONLISP-004, REQ-COMMONLISP-007, REQ-COMMONLISP-008
func TestSources(t *testing.T) {
	results := analyze(t)
	langtest.CheckImports(t, results["lisp/autolisp.lsp"], map[string]lang.Target{
		"c:": {},
	})
	langtest.CheckImports(t, results["lisp/old.cl"], map[string]lang.Target{
		"in-package shop": local("src/package.lisp"),
	})
	langtest.CheckImports(t, results["ocicl-app/app.lisp"], map[string]lang.Target{
		"use cl":            std("common-lisp"),
		"import-from ppcre": ociclPpcre,
		"import-from 5am":   {Ecosystem: ecosystemQuicklisp, Package: "fiveam", Floating: true},
		"in-package app":    {},
	})
	langtest.CheckImports(t, results["pis/src/extra.lisp"], map[string]lang.Target{
		"in-package cl-user": std("common-lisp"),
	})
	langtest.CheckImports(t, results["pis/src/main.lisp"], map[string]lang.Target{
		"use cl":                  std("common-lisp"),
		"import-from acme/util":   local("pis/src/util.lisp"),
		"import-from acme/extra":  local("pis/src/extra.lisp"),
		"import-from alexandria":  alexandria,
		"import-from c2mop":       dist("closer-mop"),
		"import-from mystery-lib": dist("mystery-lib"),
		"in-package acme/main":    {},
	})
	langtest.CheckImports(t, results["pis/src/util.lisp"], map[string]lang.Target{
		"use cl":               std("common-lisp"),
		"mix uiop/utility":     std("uiop"),
		"in-package acme/util": {},
		"uiop:":                std("uiop"),
	})
	langtest.CheckImports(t, results["scripts/build.lisp"], map[string]lang.Target{
		`load "~/quicklisp/setup.lisp"`: {},
		`load "helpers.lisp"`:           local("scripts/helpers.lisp"),
		"quickload shop":                local("shop.asd"),
		"quickload unknown-lib":         dist("unknown-lib"),
		"load-system shop/tests":        local("shop.asd"),
		"operate shop/core":             local("shop.asd"),
		"require sb-posix":              std("sbcl"),
		"require gray-streams":          std("implementation"),
		"require streamc.fasl":          {},
		"quickload cl-ppcre-unicode":    ppcre,
		"ql:":                           std("quicklisp"),
		"asdf:":                         std("asdf"),
	})
	langtest.CheckImports(t, results["scripts/helpers.lisp"], nil)
	langtest.CheckImports(t, results["src/core.lisp"], map[string]lang.Target{
		"use cl":                 std("common-lisp"),
		"use undeclared-pkg":     {Ecosystem: ecosystemQuicklisp, Package: "undeclared-pkg", Unresolved: true},
		"import-from local-time": dist("local-time"),
		"import-from 5am":        {Ecosystem: ecosystemQuicklisp, Package: "fiveam", Unresolved: true},
		"in-package shop.core":   {},
	})
	langtest.CheckImports(t, results["src/grovel.lisp"], map[string]lang.Target{
		"in-package shop": local("src/package.lisp"),
	})
	langtest.CheckImports(t, results["src/input-output/reader.lisp"], map[string]lang.Target{
		"in-package shop": local("src/package.lisp"),
	})
	langtest.CheckImports(t, results["src/model/cart.lisp"], map[string]lang.Target{
		"in-package shop": local("src/package.lisp"),
		"bt:":             dist("bordeaux-threads"),
		"dex:":            dexador,
		"str:":            str,
		"shop.core:":      local("src/core.lisp"),
		"local-time:":     dist("local-time"),
		"uiop:":           std("uiop"),
		"sb-ext:":         std("sbcl"),
		"mystery:":        {},
		"ppcre:":          ppcre,
	})
	langtest.CheckImports(t, results["src/model/item.lisp"], map[string]lang.Target{
		"in-package shop": local("src/package.lisp"),
		"a:":              alexandria,
	})
	langtest.CheckImports(t, results["src/old/legacy.lisp"], map[string]lang.Target{
		"in-package shop": local("src/package.lisp"),
	})
	langtest.CheckImports(t, results["src/package.lisp"], map[string]lang.Target{
		"in-package cl-user":               std("common-lisp"),
		"use cl":                           std("common-lisp"),
		"use alexandria":                   alexandria,
		"import-from cl-ppcre":             ppcre,
		"shadowing-import-from json":       dist("cl-json"),
		"local-nicknames alexandria":       alexandria,
		"local-nicknames bordeaux-threads": dist("bordeaux-threads"),
		"local-nicknames tree-sitter":      treeSitter,
	})
	langtest.CheckImports(t, results["t/main.lisp"], map[string]lang.Target{
		"use cl":                std("common-lisp"),
		"use rove":              dist("rove"),
		"use shop":              local("src/package.lisp"),
		"in-package shop/tests": {},
	})
	langtest.CheckImports(t, results["third_party/shop-vendor/vendored.lisp"], nil)
}

// Verifies: REQ-COMMONLISP-006
func TestManifests(t *testing.T) {
	results := analyze(t)
	langtest.CheckImports(t, results["qlfile"], map[string]lang.Target{
		"ql alexandria":      alexandria,
		"ql cl-ppcre":        ppcre,
		"github dexador":     dexador,
		"git str":            str,
		"git tree-sitter-cl": treeSitter,
		"local shop-vendor":  local("third_party/shop-vendor"),
	})
	langtest.CheckImports(t, results["qlfile.lock"], map[string]lang.Target{
		"lock alexandria":     alexandria,
		"lock cl-ppcre":       ppcre,
		"lock dexador":        dexador,
		"lock str":            str,
		"lock tree-sitter-cl": treeSitter,
	})
	langtest.CheckImports(t, results["tools/qlfile"], map[string]lang.Target{
		"ql fiveam":         {Ecosystem: ecosystemQuicklisp, Package: "fiveam", Floating: true},
		"github bar":        {Ecosystem: ecosystemQuicklisp, Package: "github.com/foo/bar", Version: "89abcdef0123456789abcdef0123456789abcdef", Pinned: true},
		"github widgets":    {Ecosystem: ecosystemQuicklisp, Package: "github.com/acme/widgets", Version: "v1.2"},
		"git gitlib":        {Ecosystem: ecosystemQuicklisp, Package: "gitlab.com/x/gitlib", Version: "dev", Floating: true},
		"http yason":        {Ecosystem: ecosystemQuicklisp, Package: "yason", Version: "0123456789abcdef0123456789abcdef", Origin: "http://netzhansa.com/yason.tar.gz", Pinned: true},
		"ultralisp cl-foo":  {Ecosystem: ecosystemQuicklisp, Package: "cl-foo", Floating: true},
		"local shop-vendor": local("third_party/shop-vendor"),
	})
	langtest.CheckImports(t, results["ocicl-app/ocicl.csv"], map[string]lang.Target{
		"ocicl alexandria":       ociclAlexandria,
		"ocicl cl-ppcre-unicode": ociclPpcre,
	})
}

// Verifies: REQ-COMMONLISP-003
func TestSymbols(t *testing.T) {
	results := analyze(t)
	langtest.CheckSymbols(t, results["lisp/autolisp.lsp"], map[string]string{
		"hello": "func",
	})
	langtest.CheckSymbols(t, results["lisp/old.cl"], map[string]string{
		"old-cl": "func",
	})
	langtest.CheckSymbols(t, results["ocicl-app/app.asd"], map[string]string{
		"app": "system",
	})
	langtest.CheckSymbols(t, results["ocicl-app/app.lisp"], map[string]string{
		"app": "package",
	})
	langtest.CheckSymbols(t, results["ocicl-app/ocicl.csv"], map[string]string{})
	langtest.CheckSymbols(t, results["pis/acme.asd"], map[string]string{
		"acme": "system",
	})
	langtest.CheckSymbols(t, results["pis/src/extra.lisp"], map[string]string{
		"acme-extra": "func",
	})
	langtest.CheckSymbols(t, results["pis/src/main.lisp"], map[string]string{
		"acme/main": "package",
	})
	langtest.CheckSymbols(t, results["pis/src/util.lisp"], map[string]string{
		"acme/util": "package",
		"helper":    "func",
	})
	langtest.CheckSymbols(t, results["qlfile"], map[string]string{})
	langtest.CheckSymbols(t, results["qlfile.lock"], map[string]string{})
	langtest.CheckSymbols(t, results["scripts/build.lisp"], map[string]string{})
	langtest.CheckSymbols(t, results["scripts/helpers.lisp"], map[string]string{
		"build-all": "func",
	})
	langtest.CheckSymbols(t, results["shop.asd"], map[string]string{
		"shop-asd":   "package",
		"shop":       "system",
		"shop/core":  "system",
		"shop/tests": "system",
	})
	langtest.CheckSymbols(t, results["src/core.lisp"], map[string]string{
		"shop.core": "package",
	})
	langtest.CheckSymbols(t, results["src/grovel.lisp"], map[string]string{})
	langtest.CheckSymbols(t, results["src/input-output/reader.lisp"], map[string]string{
		"*doc*":        "var",
		"*paren*":      "var",
		"*semi*":       "var",
		"*quote*":      "var",
		"*space*":      "var",
		"weird (name":  "var",
		"*path*":       "var",
		"*data*":       "var",
		"*read-time*":  "var",
		"after-double": "func",
		"*vec*":        "var",
		"*bits*":       "var",
		"*hex*":        "var",
		"*label*":      "var",
		"read-prices":  "func",
	})
	langtest.CheckSymbols(t, results["src/model/cart.lisp"], map[string]string{
		"helper":   "func",
		"next-id":  "func",
		"platform": "func",
		"checkout": "func",
	})
	langtest.CheckSymbols(t, results["src/model/item.lisp"], map[string]string{
		"item":                  "struct",
		"cart":                  "class",
		"total":                 "func",
		"total (cart)":          "method",
		"total :around (cart)":  "method",
		"print-object (item t)": "method",
		"(setf total) (t cart)": "method",
		"(setf item-label)":     "func",
		"out-of-stock":          "class",
		"price":                 "type",
		"*carts*":               "var",
		"*tax*":                 "var",
		"+max+":                 "const",
		"with-cart":             "macro",
	})
	langtest.CheckSymbols(t, results["src/old/legacy.lisp"], map[string]string{
		"legacy": "func",
	})
	langtest.CheckSymbols(t, results["src/package.lisp"], map[string]string{
		"shop": "package",
	})
	langtest.CheckSymbols(t, results["t/main.lisp"], map[string]string{
		"shop/tests": "package",
	})
	langtest.CheckSymbols(t, results["third_party/shop-vendor/vendored.lisp"], map[string]string{
		"vendored": "func",
	})
	langtest.CheckSymbols(t, results["tools/qlfile"], map[string]string{})
	langtest.CheckSymbols(t, results["tools/tools.asd"], map[string]string{
		"tools": "system",
	})
}

// Verifies: REQ-COMMONLISP-010
func TestReader(t *testing.T) {
	forms := Read([]byte("; c\n#| a #| nested |# b |# (a . b) \"s\\\"q\" #\\( #\\Space #\\; |a b| a\\ b pkg:sym pkg::int :kw #:un 1+ 1.5d0 -1/2 " +
		"'q `(qq ,u ,@us ,.nu) #'f #(v 1) #S(point :x 1) #p\"p.lisp\" #.(eval) #1=(c . #1#) #*101 #x1F #36rZ " +
		"#+sbcl yes #-sbcl also #+nil gone #-(and) gone #+(or) gone #+(and) kept #-(or a) kept2 #+(not nil) kept3 #?\"interp\" [x] {y} #<skipped"))
	var got []string
	for _, f := range forms {
		got = append(got, show(f))
	}
	want := []string{"(a . b)", `"s"q"`, `#\(`, `#\Space`, `#\;`, "a b", "a b", "pkg:sym", "pkg:int", ":kw", ":un", "1+", "1.5d0", "-1/2",
		"(quote q)", "(quasiquote (qq (unquote u) (unquote-splicing us) (unquote-splicing nu)))", "(function f)", "vector:(v 1)",
		"s:(point :x 1)", `"p.lisp"`, "eval:((eval))", "(c . #1#)", "#*101", "#x1F", "#36rZ",
		"yes", "also", "kept", "kept2", "kept3", `"interp"`, "[x]", "{y}", "skipped"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("forms\n%q\nwant\n%q", got, want)
	}
	if forms[0].Line != 2 {
		t.Errorf("line %d", forms[0].Line)
	}
	// A stray closer is ignored, an unclosed list closes at the end.
	got = nil
	for _, f := range Read([]byte(") (a (b")) {
		got = append(got, show(f))
	}
	if !reflect.DeepEqual(got, []string{"(a (b))"}) {
		t.Errorf("unbalanced: %q", got)
	}
}

// show writes a datum back for TestReader.
func show(n *Node) string {
	switch n.Kind {
	case List:
		var parts []string
		for _, k := range n.Kids {
			parts = append(parts, show(k))
		}
		s := "(" + strings.Join(parts, " ") + ")"
		if n.Tag != "" {
			s = n.Tag + ":" + s
		}
		return s
	case Keyword:
		return ":" + n.Text
	case String:
		return `"` + n.Text + `"`
	case Symbol:
		if n.Package != "" {
			return n.Package + ":" + n.Text
		}
	}
	return n.Text
}

// Verifies: REQ-COMMONLISP-006
func TestManifestReaders(t *testing.T) {
	q := readQlfile([]byte("ql :all 2021-12-30\ngithub old-name owner/repo :tag v1 # a comment\n# ql commented 2020-01-01\n" +
		"ql-dist ultralisp cl-foo 2024-01-01\ndist ultralisp http://dist.ultralisp.org/\ndist http://beta.quicklisp.org/dist/quicklisp.txt 2022-02-02\nasdf 3.3.7\n"))
	if q.dist != "2022-02-02" || len(q.entries) != 2 {
		t.Fatalf("qlfile %+v", q)
	}
	if e := q.entries[0]; e.name != "old-name" || e.url != "https://github.com/owner/repo" || e.tag != "v1" || e.line != 2 {
		t.Errorf("github %+v", e)
	}
	if e := q.entries[1]; e.name != "cl-foo" || e.version != "2024-01-01" {
		t.Errorf("ql-dist %+v", e)
	}
	l := readLock([]byte(`("quicklisp" . (:class qlot/source/dist:source-dist :version "2023-01-01"))
("shirakumo" . (:class qlot/source/dist:source-dist :version "2024-10-01"))
("trial" . (:class qlot/source/ql-dist:source-ql-dist :initargs (:dist-name "shirakumo" :project-name "trial" :%version :latest) :version "ql-dist-2024-10-01"))
("mito" . (:class qlot/source/ql:source-ql-upstream :initargs nil :version "ql-upstream-8c795b7b4de7dc635f1d2442ef1faf8f23d283e6" :remote-url "https://github.com/fukamachi/mito.git"))`))
	if len(l.dists) != 2 || l.dists[1].version != "2024-10-01" || len(l.entries) != 2 {
		t.Fatalf("lock %+v", l)
	}
	if got := lockTarget(l.entries[0], nil, "trial"); got != (lang.Target{Ecosystem: ecosystemQuicklisp, Package: "trial", Version: "2024-10-01", Requested: "latest", Pinned: true}) {
		t.Errorf("ql-dist %+v", got)
	}
	if got := lockTarget(l.entries[1], nil, "mito"); got != (lang.Target{Ecosystem: ecosystemQuicklisp, Package: "mito", Version: "8c795b7b4de7dc635f1d2442ef1faf8f23d283e6", Pinned: true}) {
		t.Errorf("upstream %+v", got)
	}
	o := readOcicl([]byte("# comment\nstr, ghcr.io/ocicl/cl-str@sha256:ab, cl-str-20240101-abc1234/str.asd\nshort, ghcr.io/ocicl/short:latest\n\n"))
	if len(o) != 2 || o[0].project != "cl-str" || o[0].version() != "20240101-abc1234" || o[1].project != "short" || o[1].digest != "" {
		t.Errorf("ocicl %+v", o)
	}
	if got := ociclTarget(o[1]); got.Pinned {
		t.Errorf("a tag pinned: %+v", got)
	}
}

// Verifies: REQ-COMMONLISP-007
func TestProjects(t *testing.T) {
	for system, want := range map[string]string{
		"cl-ppcre-unicode": "cl-ppcre", "ironclad/digests/sha256": "ironclad", "str": "cl-str", "cl+ssl": "cl-plus-ssl",
		"clack-handler-hunchentoot": "clack", "lack-middleware-session": "lack", "dbd-sqlite3": "cl-dbi",
		"alexandria": "alexandria", "trivial-gray-streams": "trivial-gray-streams", "swank": "slime", "clack": "clack",
	} {
		if got := projectOf(system); got != want {
			t.Errorf("%s: %s, want %s", system, got, want)
		}
	}
	for packageName, want := range map[string]string{"bt2": "bordeaux-threads", "lack.request": "lack", "trivia.level2": "trivia", "c2mop": "closer-mop"} {
		if got, _ := packageSystem(packageName); got != want {
			t.Errorf("%s: %s, want %s", packageName, got, want)
		}
	}
	if _, ok := packageSystem("mystery"); ok {
		t.Error("mystery has a system")
	}
}

// Verifies: REQ-COMMONLISP-001
func TestClaims(t *testing.T) {
	var claimed []string
	classes := map[string]string{}
	for _, f := range langtest.Files(t, "testdata/repo") {
		if (Plugin{}).Claims(f) {
			claimed = append(claimed, f.Path)
			classes[f.Path] = (Plugin{}).Class(f)
		}
	}
	sort.Strings(claimed)
	want := []string{"lisp/autolisp.lsp", "lisp/old.cl", "ocicl-app/app.asd", "ocicl-app/app.lisp", "ocicl-app/ocicl.csv",
		"pis/acme.asd", "pis/src/extra.lisp", "pis/src/main.lisp", "pis/src/util.lisp", "qlfile", "qlfile.lock",
		"scripts/build.lisp", "scripts/helpers.lisp", "shop.asd", "src/core.lisp", "src/grovel.lisp", "src/input-output/reader.lisp",
		"src/model/cart.lisp", "src/model/item.lisp", "src/old/legacy.lisp", "src/package.lisp", "t/main.lisp",
		"third_party/shop-vendor/vendored.lisp", "tools/qlfile", "tools/tools.asd"}
	if !reflect.DeepEqual(claimed, want) {
		t.Errorf("claimed %q\nwant %q", claimed, want)
	}
	if classes["qlfile"] != classQlfile || classes["qlfile.lock"] != classLock || classes["ocicl-app/ocicl.csv"] != classOcicl || classes["shop.asd"] != "" {
		t.Errorf("classes %v", classes)
	}
	absolute, _ := filepath.Abs("testdata/repo")
	for _, f := range []*scan.File{{Path: "a.lisp", Binary: true}, {Path: "k.cl", Language: "OpenCL"}, {Path: "k.cl"},
		{Path: ".qlot/dists/quicklisp/software/x/x.lisp"}, {Path: "app/.qlot/local-projects/y.asd"},
		{Path: "ocicl-app/systems/alexandria-20240503-8514d8e/alexandria.asd", AbsolutePath: filepath.Join(absolute, "ocicl-app/systems/alexandria-20240503-8514d8e/alexandria.asd")},
		{Path: "Qlfile"}, {Path: "x.csv"}} {
		if (Plugin{}).Claims(f) {
			t.Errorf("%s claimed", f.Path)
		}
	}
	for _, f := range []*scan.File{{Path: "k.cl", Language: "Common Lisp"}, {Path: "systems/x.lisp", AbsolutePath: filepath.Join(absolute, "systems/x.lisp")}, {Path: "X.LISP"}, {Path: "y.ASD"}} {
		if !(Plugin{}).Claims(f) {
			t.Errorf("%s not claimed", f.Path)
		}
	}
}

// Verifies: REQ-COMMONLISP-009
func TestIslands(t *testing.T) {
	ids := map[string]bool{}
	for _, e := range (Plugin{}).Ecosystems() {
		ids[e.ID] = e.Std
	}
	if std, ok := ids[ecosystemStd]; !ok || !std || len(ids) != 2 || ids[ecosystemQuicklisp] {
		t.Fatalf("ecosystems: %v", ids)
	}
	for f, r := range analyze(t) {
		for _, imported := range r.Imports {
			if e := imported.Target.Ecosystem; e != "" && e != ecosystemQuicklisp && e != ecosystemStd {
				t.Errorf("%s: %s -> %s", f, imported.Spec, e)
			}
		}
	}
}

// Every prefix of every fixture file, and long runs of each construct, are
// read without a panic and in linear time. Cut off, a file ends inside a
// string, a block comment, an escaped symbol, a reader conditional or a list.
//
// Verifies: REQ-COMMONLISP-010
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
		for i := 0; i <= len(source); i++ {
			read(source[:i])
			readQlfile(source[:i])
			readLock(source[:i])
			readOcicl(source[:i])
		}
	}
	for _, unit := range []string{"(", ")", "\"", "#|", "|#", "|", "\\", "#\\", "#\\a", "#+", "#-", "#+nil ", "#+(or) ", "#.", "#'", "'", "`",
		",", ",@", "#(", "#S(", "#p", "#1=", "#1#", "#*", "#x", "#:", ":", "pkg:", "a::b ", ";", "#", "#<", "é",
		"(defpackage p (:use ", "(defpackage p (:local-nicknames (a ", "(defsystem s :components (", "(:module m :components (",
		"(defsystem s :depends-on ((:feature :x ", "(in-package ", "(progn ", "(eval-when () ", "(let () ", "(defmethod m ((a ",
		"(ql:quickload '(", "(load \"", "(\"x\" . (:class ", "ql x\n", "a, b, c\n"} {
		source := []byte(strings.Repeat(unit, 200_000/len(unit)+1))
		start := time.Now()
		read(source)
		readLock(source)
		readQlfile(source)
		readOcicl(source)
		if d := time.Since(start); d > 5*time.Second {
			t.Errorf("%q x %d: %v", unit, len(source)/len(unit), d)
		}
	}
}

// What Qlot installed into .qlot/ answers for a project: dexador's system
// depends on cffi-grovel (the cffi project), fast-http, quri and, on Windows,
// flexi-streams, each pinned by the lock's dist; its own secondary system, an
// implementation module, UIOP and a weak dependency are not dependencies. A git
// source is found by its repository's name, cl-str by its system str. What
// ocicl installed into systems/ answers likewise, pinned by ocicl.csv's digest
// where it lists the system. A tree that is missing or not Lisp says nothing.
//
// Verifies: REQ-COMMONLISP-012
func TestInstalledSystems(t *testing.T) {
	const software = ".qlot/dists/quicklisp/software/"
	root := langtest.Write(t, map[string]string{
		"qlfile": "ql dexador :latest\n",
		"qlfile.lock": "(\"quicklisp\" .\n (:class qlot/source/dist:source-dist\n  :initargs (:distribution \"https://beta.quicklisp.org/dist/quicklisp.txt\" :%version :latest)\n" +
			"  :version \"2023-10-21\"))\n",
		software + "dexador-20231021-git/dexador.asd": "(defsystem \"dexador\"\n :defsystem-depends-on (\"cffi-grovel\")\n" +
			" :depends-on (\"fast-http\" \"quri\" (:feature :windows \"flexi-streams\") \"dexador/util\" (:require \"sb-bsd-sockets\") \"uiop\")\n" +
			" :weakly-depends-on (\"cl+ssl\"))\n(defsystem \"dexador/util\" :depends-on (\"babel\"))\n",
		software + "cl-str-20231021-git/str.asd": "(defsystem \"str\" :depends-on (\"cl-ppcre\"))\n",
		software + "broken/broken.asd":           "((((( \x00 #+",
	})
	r := newResolver(root, langtest.Files(t, root))
	ql := func(name string) lang.Target {
		return lang.Target{Ecosystem: ecosystemQuicklisp, Package: name, Version: "2023-10-21", Pinned: true}
	}
	for packageName, want := range map[string][]lang.Target{
		"dexador":                      {ql("cffi"), ql("fast-http"), ql("quri"), ql("flexi-streams")},
		"github.com/fukamachi/dexador": {ql("cffi"), ql("fast-http"), ql("quri"), ql("flexi-streams")},
		"cl-str":                       {ql("cl-ppcre")},
		"fast-http":                    nil,
	} {
		dependencyTarget := lang.Target{Ecosystem: ecosystemQuicklisp, Package: packageName}
		if got := r.Dependencies(dependencyTarget); !reflect.DeepEqual(got, want) || !r.Installed(dependencyTarget) {
			t.Errorf("%s depends on %+v, want %+v", packageName, got, want)
		}
	}

	root = langtest.Write(t, map[string]string{
		"app.asd": "(defsystem \"app\" :depends-on (\"dexador\"))\n",
		"ocicl.csv": "dexador, ghcr.io/ocicl/dexador@sha256:aaaa, dexador-20240101-abc1234/dexador.asd\n" +
			"quri, ghcr.io/ocicl/quri@sha256:bbbb, quri-20231001-def5678/quri.asd\n",
		"systems/dexador-20240101-abc1234/dexador.asd": "(defsystem \"dexador\" :depends-on (\"quri\" \"fast-http\"))\n",
	})
	r = newResolver(root, langtest.Files(t, root))
	want := []lang.Target{
		{Ecosystem: ecosystemQuicklisp, Package: "quri", Version: "20231001-def5678", Pinned: true},
		{Ecosystem: ecosystemQuicklisp, Package: "fast-http", Floating: true},
	}
	if got := r.Dependencies(lang.Target{Ecosystem: ecosystemQuicklisp, Package: "dexador"}); !reflect.DeepEqual(got, want) {
		t.Errorf("ocicl: dexador depends on %+v, want %+v", got, want)
	}
	if got := newResolver("", langtest.Files(t, root)).Dependencies(want[0]); got != nil {
		t.Errorf("no root: %+v", got)
	}
}

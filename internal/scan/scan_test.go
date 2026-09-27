package scan

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Verifies: REQ-LANG-015, REQ-LANG-016, REQ-LANG-018, REQ-LANG-019, REQ-LANG-021
func TestScanMeasuresAndExcludes(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"a.go":                                                  "package a\n\nfunc A() {}\n",
		"no_newline.py":                                         "x = 1\ny = 2",
		"gen/skip.go":                                           "package gen\n",
		"node_modules/x/i.js":                                   "ignored by default when git is unavailable\n",
		".build/checkouts/nio/Package.swift":                    "// SwiftPM's build directory\n",
		".dart_tool/package_config.json":                        "{}\n",
		"_build/dev/lib/shop/ebin/shop.app":                     "{application, shop, []}.\n",
		"dist-newstyle/cache/plan.json":                         "{}\n",
		".stack-work/dist/x/Paths_shop.hs":                      "module Paths_shop where\n",
		".terraform/modules/vpc/main.tf":                        "variable \"x\" {}\n",
		".terragrunt-cache/a/b/main.tf":                         "variable \"x\" {}\n",
		"_opam/lib/lwt/lwt.mli":                                 "val return : 'a -> 'a t\n",
		".zig-cache/o/1/cimport.zig":                            "pub const x = 1;\n",
		"zig-cache/h/timestamp.zig":                             "pub const x = 1;\n",
		"zig-out/bin/gen.zig":                                   "pub const x = 1;\n",
		"zig-pkg/x-0.1.0-AAAA/build.zig.zon":                    ".{}\n",
		".cpcache/1234.basis":                                   "{}\n",
		".shadow-cljs/builds/app/x.edn":                         "{}\n",
		"elm-stuff/0.19.1/Main.elm":                             "module Main exposing (main)\n",
		".spago/p/prelude-6.0.1/src/Prelude.purs":               "module Prelude where\n",
		"bower_components/purescript-maybe/src/Data/Maybe.purs": "module Data.Maybe where\n",
		".crystal/cache/macro.cr":                               "module M\nend\n",
		".fake/build.fsx/intellisense.fsx":                      "#r \"x.dll\"\n",
		".dub/packages/leftpad/1.0.0/leftpad/source/leftpad.d":  "module leftpad;\n",
		"img.bin": "\x00\x01\x02",
	}
	for p, c := range files {
		abs := filepath.Join(root, p)
		os.MkdirAll(filepath.Dir(abs), 0o755)
		os.WriteFile(abs, []byte(c), 0o644)
	}

	got, err := Scan(context.Background(), root, Options{Exclude: []string{"gen"}})
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]*File{}
	for _, f := range got {
		byPath[f.Path] = f
	}
	if len(byPath) != 3 {
		t.Fatalf("got files %v, want a.go, no_newline.py, img.bin", byPath)
	}
	if f := byPath["a.go"]; f.LOC != 3 || f.Lang != "Go" {
		t.Errorf("a.go: %+v", f)
	}
	if f := byPath["no_newline.py"]; f.LOC != 2 || f.Lang != "Python" {
		t.Errorf("no_newline.py: %+v", f)
	}
	if f := byPath["img.bin"]; !f.Binary || f.LOC != 0 {
		t.Errorf("img.bin: %+v", f)
	}
}

// A PureScript project's output/ is what the compiler wrote, a shard's lib/ what
// shards installed, and packages/ and paket-files/ beside a paket.dependencies
// what Paket installed; another output/, lib/ or packages/ directory is kept.
//
// Verifies: REQ-LANG-018
func TestScanSkipsGeneratedBesideManifest(t *testing.T) {
	root := t.TempDir()
	for p, c := range map[string]string{
		"app/spago.yaml":                         "package:\n  name: app\n",
		"app/src/Main.purs":                      "module Main where\n",
		"app/output/Main/index.js":               "export const main = 1;\n",
		"legacy/spago.dhall":                     "{ name = \"legacy\", dependencies = [] : List Text, sources = [] : List Text }\n",
		"legacy/output/Main/index.js":            "export const main = 1;\n",
		"report/output/summary.md":               "# kept\n",
		"shop/shard.yml":                         "name: shop\n",
		"shop/lib/kemal/src/kemal.cr":            "module Kemal\nend\n",
		"tools/lib/helper.cr":                    "module Helper\nend\n",
		"fs/paket.dependencies":                  "nuget Argu\n",
		"fs/packages/Argu/tools/x.fsx":           "let x = 1\n",
		"fs/paket-files/fsharp/FAKE/Globbing.fs": "module Globbing\n",
		"web/packages/app.fs":                    "module App\n",
	} {
		abs := filepath.Join(root, p)
		os.MkdirAll(filepath.Dir(abs), 0o755)
		os.WriteFile(abs, []byte(c), 0o644)
	}
	got, err := Scan(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, f := range got {
		paths = append(paths, f.Path)
	}
	want := []string{"app/spago.yaml", "app/src/Main.purs", "fs/paket.dependencies", "legacy/spago.dhall",
		"report/output/summary.md", "shop/shard.yml", "tools/lib/helper.cr", "web/packages/app.fs"}
	if !reflect.DeepEqual(paths, want) {
		t.Errorf("got %v, want %v", paths, want)
	}
}

func TestScanSkipsSymlinksInGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	outside := filepath.Join(t.TempDir(), "secret.txt")
	os.WriteFile(outside, []byte("not part of the repository\n"), 0o644)
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o644)
	if err := os.Symlink(outside, filepath.Join(root, "notes.txt")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	got, err := Scan(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range got {
		if f.Path == "notes.txt" {
			t.Errorf("a tracked symlink to %s was scanned as a file", outside)
		}
	}
	if len(got) != 1 {
		t.Errorf("got %d files, want a.go alone", len(got))
	}
}

// Verifies: REQ-LANG-018
func TestWalkSkipsUnreadableDirectories(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o644)
	locked := filepath.Join(root, "locked")
	os.MkdirAll(locked, 0o755)
	os.WriteFile(filepath.Join(locked, "b.go"), []byte("package b\n"), 0o644)
	if err := os.Chmod(locked, 0); err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })
	if _, err := os.ReadDir(locked); err == nil {
		t.Skip("the directory is readable anyway (running as root, or on Windows)")
	}

	got, err := walkFiles(context.Background(), root)
	if err != nil {
		t.Fatalf("one unreadable directory failed the walk: %v", err)
	}
	if len(got) != 1 || got[0] != "a.go" {
		t.Errorf("got %v, want [a.go]", got)
	}
}

// git decides what a repository contains: whatever .gitignore excludes is left out,
// an untracked file nothing ignores is kept, and a file in conflict - which git lists
// once per merge stage - comes back once.
//
// Verifies: REQ-LANG-017
func TestScanListsWhatGitDoes(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		// No user configuration is assumed: identity and signing are set here.
		base := []string{"-C", root, "-c", "user.name=test", "-c", "user.email=test@example.com",
			"-c", "commit.gpgsign=false", "-c", "core.autocrlf=false"}
		if out, err := exec.Command("git", append(base, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(p, content string) {
		t.Helper()
		abs := filepath.Join(root, filepath.FromSlash(p))
		os.MkdirAll(filepath.Dir(abs), 0o755)
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q")
	write(".gitignore", "*.log\nout/\n")
	write("conflict.txt", "base\n")
	git("add", ".")
	git("commit", "-q", "-m", "base")
	git("checkout", "-q", "-b", "other")
	write("conflict.txt", "other\n")
	git("commit", "-q", "-am", "other")
	git("checkout", "-q", "-")
	write("conflict.txt", "mine\n")
	git("commit", "-q", "-am", "mine")
	// The merge fails by design, leaving conflict.txt in three stages.
	exec.Command("git", "-C", root, "-c", "user.name=test", "-c", "user.email=test@example.com",
		"merge", "-q", "other").Run()

	write("debug.log", "ignored\n")
	write("out/app.bin", "ignored\n")
	write("new.go", "package a\n") // untracked, and nothing ignores it

	got, err := Scan(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	count := map[string]int{}
	for _, f := range got {
		count[f.Path]++
	}
	want := map[string]int{".gitignore": 1, "conflict.txt": 1, "new.go": 1}
	if !reflect.DeepEqual(count, want) {
		t.Errorf("scanned %v, want %v", count, want)
	}
}

// Verifies: REQ-LANG-020
func TestScanMeasuresEveryFilesSize(t *testing.T) {
	// Bytes are measured whether or not the lines are counted: a text file, a binary
	// and a file over the size limit all carry their size on disk.
	root := t.TempDir()
	files := map[string]string{
		"a.go":    "package a\n\nfunc A() {}\n",
		"img.bin": "\x00\x01\x02\x03",
		"big.txt": strings.Repeat("x", 100) + "\n",
	}
	for p, c := range files {
		os.WriteFile(filepath.Join(root, p), []byte(c), 0o644)
	}
	got, err := Scan(context.Background(), root, Options{MaxFileSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(files) {
		t.Fatalf("got %d files, want %d", len(got), len(files))
	}
	for _, f := range got {
		st, err := os.Stat(f.Abs)
		if err != nil {
			t.Fatal(err)
		}
		if f.Size != st.Size() || f.Size != int64(len(files[f.Path])) {
			t.Errorf("%s: size %d, want %d", f.Path, f.Size, st.Size())
		}
	}
}

// A script without an extension is labelled by the shell its "#!" line runs, read
// through env; other interpreters are recorded but label nothing.
//
// Verifies: REQ-LANG-015, REQ-SHELL-001
func TestScanReadsShebangs(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"bin/deploy":   "#!/usr/bin/env -S bash -e\necho hi\n",
		"bin/install":  "#! /bin/sh\n",
		"bin/tool":     "#!/usr/bin/python3\nprint(1)\n",
		"lib/x.py":     "#!/bin/sh\n",
		"README":       "no shebang\n",
		"bin/env-only": "#!/usr/bin/env\n",
	}
	for p, c := range files {
		abs := filepath.Join(root, p)
		os.MkdirAll(filepath.Dir(abs), 0o755)
		os.WriteFile(abs, []byte(c), 0o644)
	}
	got, err := Scan(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][2]string{
		"bin/deploy":   {"bash", "Shell"},
		"bin/install":  {"sh", "Shell"},
		"bin/tool":     {"python3", ""},
		"lib/x.py":     {"sh", "Python"},
		"README":       {"", ""},
		"bin/env-only": {"", ""},
	}
	for _, f := range got {
		if w := want[f.Path]; f.Interpreter != w[0] || f.Lang != w[1] {
			t.Errorf("%s: interpreter %q, lang %q; want %q, %q", f.Path, f.Interpreter, f.Lang, w[0], w[1])
		}
	}
}

// A Qt Linguist translation shares TypeScript's ".ts" extension; its XML
// declaration or document type says which one a file is.
//
// Verifies: REQ-LANG-015
func TestScanTellsQtLinguistFromTypeScript(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"src/app.ts":           "import { x } from './x';\n",
		"src/cast.ts":          "<Shape>thing;\n",
		"i18n/app_de.ts":       "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<!DOCTYPE TS>\n<TS version=\"2.1\" language=\"de\">\n</TS>\n",
		"i18n/app_fr.ts":       "\xef\xbb\xbf\n<!DOCTYPE TS><TS version=\"2.0\"></TS>\n",
		"i18n/notes.tsx":       "<?xml version=\"1.0\"?>\n",
		"i18n/app_de.ts.xml":   "<?xml version=\"1.0\"?>\n",
		"src/component.mts":    "export const a = 1;\n",
		"src/declarations.cts": "<?xml version=\"1.0\"?>\n",
	}
	for p, c := range files {
		abs := filepath.Join(root, p)
		os.MkdirAll(filepath.Dir(abs), 0o755)
		os.WriteFile(abs, []byte(c), 0o644)
	}
	got, err := Scan(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"src/app.ts":     "TypeScript",
		"src/cast.ts":    "TypeScript",
		"i18n/app_de.ts": "XML",
		"i18n/app_fr.ts": "XML",
		// Only ".ts" is shared with Qt Linguist; the others keep their name's word.
		"i18n/notes.tsx":       "TypeScript",
		"i18n/app_de.ts.xml":   "XML",
		"src/component.mts":    "TypeScript",
		"src/declarations.cts": "TypeScript",
	}
	for _, f := range got {
		if f.Lang != want[f.Path] {
			t.Errorf("%s: lang %q, want %q", f.Path, f.Lang, want[f.Path])
		}
	}
}

// ".m" is Objective-C's, MATLAB's and Mercury's, and ".h" C's, C++'s and
// Objective-C's: what the head of the file writes says which. A ".m" without any
// directive, comment or keyword of Objective-C is not Objective-C; a ".h" with a
// keyword of Objective-C or #import is.
//
// Verifies: REQ-LANG-015, REQ-OBJC-001
func TestScanTellsObjectiveC(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"App/Cart.m":       "//  Cart.m\n#import \"Cart.h\"\n@implementation Cart\n@end\n",
		"App/main.m":       "#include <stdio.h>\nint main(void) { return 0; }\n",
		"App/Bare.m":       "\xef\xbb\xbf@implementation Bare\n@end\n",
		"App/Cart.h":       "/* Cart */\n#import <Foundation/Foundation.h>\n",
		"App/Proto.h":      "#pragma once\n@protocol Shop <NSObject>\n@end\n",
		"App/Fwd.h":        "@class Cart;\n",
		"lib/util.h":       "#ifndef UTIL_H\n#define UTIL_H\nint add(int, int);\n#endif\n",
		"lib/comment.h":    "// uses @interface in a comment? no: @interfaces\nint x;\n",
		"matlab/smooth.m":  "% SMOOTH moving average\nfunction y = smooth(x)\ny = x;\nend\n",
		"matlab/script.m":  "x = linspace(0, 1);\nplot(x, x.^2)\n",
		"mercury/solver.m": "%---%\n:- module solver.\n:- interface.\n",
		"App/Store.mm":     "x = 1;\n",
	}
	for p, c := range files {
		abs := filepath.Join(root, p)
		os.MkdirAll(filepath.Dir(abs), 0o755)
		os.WriteFile(abs, []byte(c), 0o644)
	}
	got, err := Scan(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"App/Cart.m": "Objective-C", "App/main.m": "Objective-C", "App/Bare.m": "Objective-C",
		"App/Cart.h": "Objective-C", "App/Proto.h": "Objective-C", "App/Fwd.h": "Objective-C",
		"lib/util.h": "C", "lib/comment.h": "C",
		"matlab/smooth.m": "MATLAB", "matlab/script.m": "MATLAB", "mercury/solver.m": "Mercury",
		"App/Store.mm": "Objective-C++",
	}
	for _, f := range got {
		if f.Lang != want[f.Path] {
			t.Errorf("%s: lang %q, want %q", f.Path, f.Lang, want[f.Path])
		}
	}
}

// ".pl" is Perl's and Prolog's, and ".t" Perl's tests only by convention: a perl
// "#!" line or a line starting as only Perl starts one (use, package, sub, my...)
// makes a file Perl; a ".pl" with Prolog directives or clauses and nothing of Perl
// is Prolog, a ".t" with nothing of Perl has no language. A script without an
// extension whose "#!" line runs perl (not perl6) is Perl.
//
// Verifies: REQ-LANG-015, REQ-PERL-001
func TestScanTellsPerl(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"bin/tool.pl":       "#!/usr/bin/perl -w\nprint 1;\n",
		"lib/helper.pl":     "use strict;\nour %DATA;\n1;\n",
		"prolog/family.pl":  "% family\n:- module(family, [parent/2]).\nparent(tom, bob).\n",
		"prolog/rules.pl":   "ancestor(X, Y) :- parent(X, Y).\n",
		"prolog/facts.pl":   "parent(tom, bob).\n",
		"t/basic.t":         "#!perl -T\nuse Test::More;\n",
		"t/plain.t":         "\nuse strict;\n",
		"templates/page.t":  "<h1>[% title %]</h1>\n",
		"script/shop":       "#!/usr/bin/env perl\nuse strict;\n",
		"script/bbtool":     "#!/usr/bin/env bb\n(println 1)\n",
		"script/raku":       "#!/usr/bin/env perl6\nsay 1;\n",
		"cgi-bin/index.cgi": "#!/usr/bin/perl5.36.0\nprint 1;\n",
		"lib/Shop.pm":       "package Shop;\n1;\n",
		"app.psgi":          "my $app = sub { [200, [], ['ok']] };\n",
		"cpanfile":          "requires 'Plack';\n",
		"cpanfile.snapshot": "# carton snapshot format: version 1.0\n",
		"dist.ini":          "name = Shop\n",
		"Makefile.PL":       "use ExtUtils::MakeMaker;\n",
	}
	for p, c := range files {
		abs := filepath.Join(root, p)
		os.MkdirAll(filepath.Dir(abs), 0o755)
		os.WriteFile(abs, []byte(c), 0o644)
	}
	got, err := Scan(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"bin/tool.pl": "Perl", "lib/helper.pl": "Perl", "prolog/family.pl": "Prolog", "prolog/rules.pl": "Prolog",
		// A fact alone reads as neither; the extension's language stays.
		"prolog/facts.pl": "Perl",
		"t/basic.t":       "Perl", "t/plain.t": "Perl", "templates/page.t": "",
		"script/shop": "Perl", "script/raku": "", "script/bbtool": "Clojure", "cgi-bin/index.cgi": "Perl",
		"lib/Shop.pm": "Perl", "app.psgi": "Perl", "cpanfile": "Perl", "cpanfile.snapshot": "Carton",
		"dist.ini": "Dist::Zilla", "Makefile.PL": "Perl",
	}
	for _, f := range got {
		if f.Lang != want[f.Path] {
			t.Errorf("%s: lang %q, want %q", f.Path, f.Lang, want[f.Path])
		}
	}
}

// ".fs" is F#'s, a GLSL fragment shader's and Forth's: a shader's #version,
// uniform or void main, and Forth's \ comments and colon definitions, say which.
// F#'s own directives (#if, #load, #r) and a comment mentioning main do not.
//
// Verifies: REQ-LANG-015, REQ-FSHARP-001
func TestScanTellsFSharpFromShaders(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"src/Cart.fs":          "module Shop.Cart\n\n#if DEBUG\nopen System.Diagnostics\n#endif\n// void main() is C\nlet total = 0\n",
		"src/Types.fs":         "\xef\xbb\xbfnamespace Shop\n\ntype Item = { Sku: string }\n",
		"shaders/blur.fs":      "#version 330 core\nout vec4 color;\nvoid main() { color = vec4(1.0); }\n",
		"shaders/old.fs":       "precision mediump float;\nuniform sampler2D tex;\nvoid main() { gl_FragColor = texture2D(tex, vec2(0.0)); }\n",
		"shaders/bare.fs":      "void main()\n{\n    gl_FragColor = vec4(1.0);\n}\n",
		"forth/hello.fs":       "\\ greet\n: hello .\" Hello\" cr ;\n",
		"forth/square.fs":      ": square ( n -- n^2 ) dup * ;\n",
		"scripts/build.fsx":    "#r \"nuget: Fake.Core.Target\"\nopen Fake.Core\n",
		"src/Shop.fsproj":      "<Project Sdk=\"Microsoft.NET.Sdk\" />\n",
		"paket.dependencies":   "nuget Argu\n",
		"paket.lock":           "NUGET\n",
		"src/paket.references": "Argu\n",
	}
	for p, c := range files {
		abs := filepath.Join(root, p)
		os.MkdirAll(filepath.Dir(abs), 0o755)
		os.WriteFile(abs, []byte(c), 0o644)
	}
	got, err := Scan(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"src/Cart.fs": "F#", "src/Types.fs": "F#", "scripts/build.fsx": "F#", "src/Shop.fsproj": "F#",
		"shaders/blur.fs": "GLSL", "shaders/old.fs": "GLSL", "shaders/bare.fs": "GLSL",
		"forth/hello.fs": "Forth", "forth/square.fs": "Forth",
		"paket.dependencies": "Paket", "paket.lock": "Paket", "src/paket.references": "Paket",
	}
	for _, f := range got {
		if f.Lang != want[f.Path] {
			t.Errorf("%s: lang %q, want %q", f.Path, f.Lang, want[f.Path])
		}
	}
}

// ".d" is D's, a make dependency file's (gcc -MD, dmd -makedeps) and a DTrace
// script's: a first line `target: prerequisites`, or a probe description, a
// provider block, #pragma D or a C preprocessor directive, say which. A D module
// whose first line imports with bindings (`import std.stdio : writeln;`) or is
// an attribute label (`@safe:`) stays D.
//
// Verifies: REQ-LANG-015, REQ-DLANG-001
func TestScanTellsDFromDepfilesAndDTrace(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"source/app.d":      "module app;\n\nimport std.stdio;\n",
		"source/bind.d":     "import std.stdio : writeln;\n",
		"source/safe.d":     "@safe:\nvoid f() {}\n",
		"source/private.d":  "private:\nvoid f() {}\n",
		"source/label.di":   "module label;\n",
		"source/script.d":   "#!/usr/bin/env rdmd\nimport std.stdio;\nvoid main() {}\n",
		"deps/app.d":        "app.o: source/app.d /usr/include/dmd/phobos/std/stdio.d \\\n source/bind.d\n",
		"deps/two.d":        "a.o b.o: a.c\n",
		"dtrace/open.d":     "#!/usr/sbin/dtrace -s\nsyscall::open:entry { trace(copyinstr(arg0)); }\n",
		"dtrace/begin.d":    "dtrace:::BEGIN\n{\n  printf(\"hi\");\n}\n",
		"dtrace/pragma.d":   "#pragma D option quiet\nprofile-97 { @[execname] = count(); }\n",
		"dtrace/provider.d": "provider shop {\n\tprobe order__placed(int);\n};\n",
		"dtrace/pid.d":      "pid$target::malloc:entry\n/arg0 > 100/\n{ }\n",
	}
	for p, c := range files {
		abs := filepath.Join(root, p)
		os.MkdirAll(filepath.Dir(abs), 0o755)
		os.WriteFile(abs, []byte(c), 0o644)
	}
	got, err := Scan(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"source/app.d": "D", "source/bind.d": "D", "source/safe.d": "D", "source/private.d": "D",
		"source/label.di": "D", "source/script.d": "D",
		"deps/app.d": "Make", "deps/two.d": "Make",
		"dtrace/open.d": "DTrace", "dtrace/begin.d": "DTrace", "dtrace/pragma.d": "DTrace",
		"dtrace/provider.d": "DTrace", "dtrace/pid.d": "DTrace",
	}
	if len(got) != len(want) {
		t.Errorf("scanned %d files, want %d", len(got), len(want))
	}
	for _, f := range got {
		if f.Lang != want[f.Path] {
			t.Errorf("%s: lang %q, want %q", f.Path, f.Lang, want[f.Path])
		}
	}
}

// ".f" and ".for" are fixed-form Fortran's and sometimes Forth's: Forth's \
// comments and colon definitions say which. A Fortran comment line starts with
// C, * or ! and code starts in column 7.
//
// Verifies: REQ-LANG-015, REQ-FORTRAN-001
func TestScanTellsFortranFromForth(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"legacy/dgemm.f":  "C     Level 3 BLAS.\n      SUBROUTINE DGEMM(A)\n      END\n",
		"legacy/star.for": "*     A comment.\n      PROGRAM MAIN\n      END\n",
		"legacy/free.f":   "module free_f\nend module free_f\n",
		"forth/words.f":   "\\ A Forth vocabulary.\n: square dup * ;\n",
		"forth/defs.for":  ": cube dup dup * * ;\n",
	}
	for p, c := range files {
		abs := filepath.Join(root, p)
		os.MkdirAll(filepath.Dir(abs), 0o755)
		os.WriteFile(abs, []byte(c), 0o644)
	}
	got, err := Scan(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"legacy/dgemm.f": "Fortran", "legacy/star.for": "Fortran", "legacy/free.f": "Fortran",
		"forth/words.f": "Forth", "forth/defs.for": "Forth",
	}
	for _, f := range got {
		if f.Lang != want[f.Path] {
			t.Errorf("%s: lang %q, want %q", f.Path, f.Lang, want[f.Path])
		}
	}
}

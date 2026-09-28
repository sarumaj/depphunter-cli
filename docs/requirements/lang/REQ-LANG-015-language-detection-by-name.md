---
id: REQ-LANG-015
title: Language detection by name
scope: lang
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** determine a file's language from its extension,
case-insensitively, or from well-known file names (for example `Dockerfile`,
`Makefile`, `go.mod`, `Gemfile`, `Rakefile`, `rebar.config`, an OTP
`*.app.src`, R's `DESCRIPTION`, `NAMESPACE`, `renv.lock` and `packrat.lock`,
Haskell's `cabal.project`, `stack.yaml` and `package.yaml`, Terraform's
`.terraform.lock.hcl` and `*.tf.json`, Terragrunt's `terragrunt.hcl`, Buf's
`buf.yaml`, `buf.work.yaml`, `buf.lock` and `buf.gen.yaml`, CMake's
`CMakeLists.txt`, `*.cmake.in`, `CMakePresets.json` and
`CMakeUserPresets.json`, CocoaPods' `Podfile`, `Podfile.lock` and
`*.podspec`, Carthage's `Cartfile`, `Cartfile.private` and
`Cartfile.resolved`, LuaRocks' `luarocks.lock`, luacheck's `.luacheckrc` and
busted's `.busted`, Perl's `cpanfile`, Carton's `cpanfile.snapshot` and
Dist::Zilla's `dist.ini`, dune's `dune`, `dune-project` and
`dune-workspace`, opam's `opam`, `opam.locked` and `*.opam.locked`, Julia
Pkg's `Project.toml`, `JuliaProject.toml`, `Manifest.toml`,
`JuliaManifest.toml`, versioned `Manifest-vX.Y.toml` and `Artifacts.toml`,
Clojure's `project.clj`, `build.boot`, `deps.edn`, `bb.edn` and
`shadow-cljs.edn`, Bazel's `BUILD`, `WORKSPACE`, `WORKSPACE.bzlmod`
(Starlark, as `.bzl` and `.bazel` files are) and `MODULE.bazel.lock`
(JSON), Nix's `flake.lock` (Nix, as `.nix` files are), Gleam's `gleam.toml`
(Gleam, as `.gleam` files are), shards' `shard.lock` (YAML, as `shard.yml`
is), Paket's `paket.dependencies`, `paket.lock` and `paket.references`
(Paket; `.fs`, `.fsi`, `.fsx`, `.fsscript` and `.fsproj` files are F#),
dub's `dub.sdl` (SDLang; `.d` and `.di` files are D), Fortran's
extensions (`.f90`, `.f95`, `.f03`, `.f08`, `.f18`, `.f`, `.for`, `.ftn`,
`.f77`, `.fpp` and fypp's `.fypp`, in either case), Haxe's `.hx` and
`.hxml`, Ada's `.ads`, `.adb` and `.ada` and GNAT's project files (`.gpr`,
GPR), Racket's `.rkt`, `.rktl` and `.rktd`, Scribble's `.scrbl` and
Scheme's `.scm` and `.ss`, Common Lisp's `.lisp`, `.lsp`, `.cl` and `.asd`,
Qlot's `qlfile` and `qlfile.lock` (Qlot) and ocicl's `ocicl.csv` (ocicl),
Solidity's `.sol`, Foundry's `foundry.toml` and `remappings.txt` (Foundry)
and Soldeer's `soldeer.lock` (Soldeer), Nim's `.nim`, `.nims`, `.nimble`
and `nim.cfg` (Nim), nimble's `nimble.lock` (Nimble) and Atlas's
`atlas.lock` (Atlas), Jsonnet's `.jsonnet` and `.libsonnet`,
jsonnet-bundler's `jsonnetfile.json` and `jsonnetfile.lock.json`
(jsonnet-bundler), CUE's `.cue`, Puppet's `.pp` and `Puppetfile`,
Rego's `.rego`,
the shells' and direnv's startup files (`.bashrc`,
`.zshrc`, `.profile`, `.envrc` and the others of REQ-SHELL-001), and the
Dockerfile names of REQ-DOCKER-001), or, for a file neither names, from a
`#!` line running a shell (REQ-SHELL-001), perl (REQ-PERL-001), babashka
(`bb`, Clojure: REQ-CLOJURE-001) or racket (Racket: REQ-RACKET-001), and
**shall** leave the language empty when none is known. A `.ts` file whose
first bytes, after an optional byte order mark and whitespace, are an XML
declaration (`<?xml`) or a document type (`<!DOCTYPE`) is a Qt Linguist
translation and **shall** be XML rather than TypeScript. A `.m` file is
Objective-C only when its head shows it, else MATLAB or Mercury, and a `.h`
file whose head shows Objective-C **shall** be Objective-C rather than C
(REQ-OBJC-001); `.mm` is Objective-C++. A `.pl` file whose head shows Prolog
and nothing of Perl **shall** be Prolog, and a `.t` file whose head shows
nothing of Perl **shall** have no language (REQ-PERL-001). A `.fs` file whose
head shows a GLSL fragment shader **shall** be GLSL, and one whose head shows
Forth **shall** be Forth, rather than F# (REQ-FSHARP-001). A `.d` file whose
head is a make dependency file (a first line `target: prerequisites`)
**shall** be Make, and one whose head shows DTrace (a probe description, a
`provider` block, `#pragma D`, a C preprocessor directive or a `#!` line
running dtrace) **shall** be DTrace, rather than D (REQ-DLANG-001). A `.f`
or `.for` file whose head shows Forth (a `\` comment line or a `: name ... ;`
definition in column 1) **shall** be Forth, rather than Fortran
(REQ-FORTRAN-001). A `.scm` or `.ss` file whose first line, after an
optional `#!` line, blank lines and `;` comments, is a `#lang` line (or
`#!racket/base`, not `#!r6rs`) **shall** be Racket rather than Scheme
(REQ-RACKET-001). A `.cl` file whose head has a line starting with a
preprocessor directive (`#include`, `#define`, `#pragma`, `#if`, `#ifdef`,
`#ifndef`, `#endif`) or declares an OpenCL kernel (`__kernel`, `kernel
void`, `__global`) **shall** be OpenCL rather than Common Lisp
(REQ-COMMONLISP-001).

## Rationale

Detecting by name needs no reading of the file and is enough for the language
colors and filters. An extensionless script says what it is only in its `#!`
line, which the scan reads from the bytes it already peeks at to tell binary
files apart. Qt Linguist shares TypeScript's `.ts` extension; the same bytes
tell the two apart, since neither opening is valid TypeScript, and they tell
Objective-C from MATLAB, Mercury and C.

## Acceptance criteria

1. `a.go` is Go, `x.py` is Python, `Dockerfile`, `Containerfile`,
   `Dockerfile.dev` and `api.Dockerfile` are Docker; `Gemfile`,
   `lib/tasks/db.rake` and `app.gemspec` are Ruby; `mix.exs` is Elixir,
   `include/state.hrl`, `rebar.config` and `src/shop.app.src` are Erlang;
   `R/cart.R`, `DESCRIPTION` and `renv.lock` are R, `vignettes/intro.Rmd` is
   R Markdown and `report.qmd` is Quarto; `src/Data/Shop.hs`, `.hs-boot`,
   `doc/Tutorial.lhs` and `stack.yaml` are Haskell, `shop.cabal` and
   `cabal.project` are Cabal; `infra/main.tf`, `main.tf.json`, `prod.tfvars`
   and `.terraform.lock.hcl` are Terraform, `main.tofu` is OpenTofu,
   `terragrunt.hcl` is Terragrunt and `root.hcl` is HCL; `shop.proto` is
   Protobuf and `buf.yaml`, `buf.work.yaml`, `buf.lock` and `buf.gen.yaml`
   are Buf; `CMakeLists.txt`, `cmake/Deps.cmake`, `shopConfig.cmake.in` and
   `CMakePresets.json` are CMake; `scripts/build.sh`, `test/app.bats`, a
   `.zsh-theme`, `.envrc`, `.zshrc` and `.bash_profile` are Shell, and so is
   an extensionless file starting `#!/usr/bin/env -S bash -e` or `#! /bin/sh`,
   while one starting `#!/usr/bin/python3` has no language and `lib/x.py`
   stays Python.
2. A file with an unknown extension has no `lang`.
3. A `.ts` file starting `<?xml version="1.0" encoding="utf-8"?>` or, after a
   byte order mark, `<!DOCTYPE TS>` is XML; one starting `<Shape>thing;` and a
   `.tsx`, `.mts` or `.cts` file starting `<?xml` stay TypeScript.
4. `ios/Podfile` and `Shop.podspec` are Ruby, `Podfile.lock` YAML,
   `Cartfile` Carthage and `Store.mm` Objective-C++; a `.m` file of MATLAB
   code is MATLAB and one declaring `:- module` Mercury.
5. `lib/cart.ml`, `lib/cart.mli` and `lib/lexer.mll` are OCaml,
   `lib/parser.mly` is Menhir, `lib/dune` and `dune-project` are Dune, and
   `shop.opam` and `shop.opam.locked` are opam.
6. `src/Shop.jl`, `Project.toml`, `docs/JuliaProject.toml`,
   `Manifest.toml`, `Manifest-v1.11.toml`, `JuliaManifest-v1.12.toml` and
   `Artifacts.toml` are Julia.
7. `nix/modules/default.nix` and `flake.lock` are Nix.
8. `src/shop/cart.cr` is Crystal, and `shard.yml` and `shard.lock` are
   YAML.
9. `src/Cart.fs`, `scripts/build.fsx` and `src/Shop.fsproj` are F#,
   `paket.dependencies`, `paket.lock` and `src/paket.references` are Paket,
   `shaders/blur.fs` starting `#version 330 core` is GLSL and
   `forth/hello.fs` starting with a `\` comment is Forth.
10. `source/shop/app.d` and `import/shop/cart.di` are D, `dub.sdl` is
    SDLang and `dub.json` and `dub.selections.json` are JSON; a `.d` file
    starting `app.o: source/app.d \` is Make, one starting
    `syscall::open:entry`, `dtrace:::BEGIN`, `#pragma D option quiet` or
    `provider shop {` is DTrace, and one starting `import std.stdio :
    writeln;` or `@safe:` stays D.
11. `src/shop.f90`, `src/cart.F90`, `legacy/dgemm.f`, `legacy/DGEMV.F`,
    `legacy/old.for` and `src/sort.fypp` are Fortran and `fpm.toml` is
    TOML; a `.f` file starting with a `\` comment and a `.for` file starting
    `: cube` are Forth.
12. `src/shop/Main.hx`, `build.hxml` and `haxe_libraries/tink_core.hxml`
    are Haxe, and `haxelib.json` is JSON.
13. `src/shop-cart.ads`, `src/shop-cart.adb` and `legacy/SHOP.ADA` are
    Ada, `shop.gpr` is GPR and `alire.toml` is TOML.
14. `shop/main.rkt`, `tests/load-me.rktl` and `data/prices.rktd` are
    Racket, `docs/manual.scrbl` is Scribble and `chez/main.ss` and
    `guile/hello.scm` are Scheme; a `.scm` starting `#lang racket` after a
    comment, `.ss` files starting `#!/usr/bin/env racket` then `#lang` or
    `#!racket/base`, and an extensionless file whose `#!` line runs racket
    are Racket, and a `.scm` starting `#!r6rs` stays Scheme.
15. `src/shop.lisp`, `legacy/tools.lsp`, `old/reader.cl` and `shop.asd`
    are Common Lisp, `qlfile` and `qlfile.lock` are Qlot and `ocicl.csv` is
    ocicl; `.cl` files starting `__kernel void`, `#pragma OPENCL` or
    `#include` are OpenCL.
16. `src/Counter.sol` is Solidity, `foundry.toml` and `remappings.txt`
    are Foundry and `soldeer.lock` is Soldeer.
17. `src/shop.nim`, `config.nims`, `shop.nimble` and `nim.cfg` are Nim,
    `nimble.lock` is Nimble and `atlas.lock` is Atlas.
18. `main.jsonnet` and `lib/k.libsonnet` are Jsonnet, `jsonnetfile.json`
    and `jsonnetfile.lock.json` are jsonnet-bundler, and `schema/a.cue` is
    CUE.
19. `manifests/site.pp` and `Puppetfile` are Puppet, `policy/deny.rego` is
    Rego and `types/Deployment.dhall` is Dhall.

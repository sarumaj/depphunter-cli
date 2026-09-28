---
id: REQ-LSP-002
title: Language server client for installed servers
scope: lsp
type: functional
priority: must
status: implemented
verification:
  - integration
---

## Statement

With `--lsp`, the system **shall** drive, as a JSON-RPC client over standard
input and output (sourcegraph/jsonrpc2), each installed language server for the
files it covers: gopls (Go), typescript-language-server (TypeScript,
JavaScript), pyright, basedpyright or pylsp (Python, first installed),
rust-analyzer (Rust), jdtls (Java), kotlin-language-server (Kotlin), metals
(Scala), csharp-ls (C#), FsAutoComplete (`fsautocomplete
--adaptive-lsp-server-enabled`; `.fs`, `.fsi` and `.fsx` files), clangd (C,
C++, Objective-C and Objective-C++), intelephense or phpactor (PHP,
first installed), ruby-lsp or solargraph (Ruby, first installed),
sourcekit-lsp (Swift), `dart language-server` (Dart), elixir-ls
(`elixir-ls` or `language_server.sh`), Lexical or Next LS (Elixir, first
installed), ELP or erlang_ls (Erlang, first installed), the R
languageserver package (`R --slave -e languageserver::run()`, R and R
Markdown) and haskell-language-server (`haskell-language-server-wrapper
--lsp` or `haskell-language-server --lsp`, Haskell and literate Haskell) and
terraform-ls or tofu-ls (`terraform-ls serve` or `tofu-ls serve`, first
installed; Terraform, OpenTofu and variable files) and Buf's language server,
bufls or protols (`buf lsp serve`, `bufls serve` or `protols`, first
installed; Protocol Buffers) and bash-language-server
(`bash-language-server start`; `.sh`, `.bash`, `.ksh`, `.bats` and `.envrc`
files, not zsh) and neocmakelsp or cmake-language-server (`neocmakelsp
--stdio` or `cmake-language-server`, first installed; `CMakeLists.txt` by
name and `.cmake` files) and lua-language-server (`lua-language-server`, Lua)
and luau-lsp (`luau-lsp lsp`, Luau) and Perl Navigator, PLS or
Perl::LanguageServer (`perlnavigator --stdio`, `pls` or `perl
-MPerl::LanguageServer -e Perl::LanguageServer::run`, first installed; `.pl`,
`.pm`, `.t` and `.psgi` files) and ocaml-lsp-server (`ocamllsp`; `.ml`,
`.mli`, `.mll` and `.mly` files) and LanguageServer.jl (`julia
--startup-file=no --history-file=no -e "using LanguageServer; runserver()"`;
`.jl` files) and zls (`zls`; `.zig` files) and clojure-lsp (`clojure-lsp`;
`.clj`, `.cljs`, `.cljc` and `.bb` files) and starpls, bazel-lsp or bzl
(`starpls server`, `bazel-lsp` or `bzl lsp serve`, first installed; `BUILD`,
`WORKSPACE` and `WORKSPACE.bzlmod` by name, `.bzl` and `.bazel` files)
and nil or nixd (`nil` or `nixd`, first installed; `.nix` files), the
Gleam compiler's language server (`gleam lsp`; `.gleam` files),
elm-language-server (`elm-language-server --stdio`; `.elm` files),
purescript-language-server (`purescript-language-server --stdio`; `.purs`
files), crystalline (`crystalline`; `.cr` files), serve-d (`serve-d`;
`.d` and `.di` files), fortls (`fortls`; `.f90`, `.f95`, `.f03`, `.f08`,
`.f18`, `.f`, `.for`, `.ftn`, `.f77` and `.fpp` files in either case),
the Haxe language server (`haxe-language-server`; `.hx` files), the Ada
Language Server (`ada_language_server`; `.ads`, `.adb` and `.ada` files),
racket-langserver (`racket -l racket-langserver`; `.rkt`, `.rktl` and
`.scrbl` files), cl-lsp (`cl-lsp`; `.lisp`, `.lsp`, `.cl` and `.asd`
files), the Nomic Foundation Solidity server or solidity-ls
(`nomicfoundation-solidity-language-server --stdio` or `solidity-ls
--stdio`, first installed; `.sol` files), nimlangserver or nimlsp
(`nimlangserver` or `nimlsp`, first installed; `.nim`, `.nims` and
`.nimble` files), jsonnet-language-server (`jsonnet-language-server`;
`.jsonnet` and `.libsonnet` files) and CUE's own server (`cue lsp`; `.cue`
files), skipping a server that is not installed.

## Rationale

Language servers are the one source of precise symbol-level usage for every
ecosystem, and they are already installed where the ecosystem is used.

## Acceptance criteria

1. With gopls installed, a Go project yields reference edges and `servers` lists
   `gopls`.
2. A server that is not on `PATH` (or in the Go binary directories, for gopls)
   is logged as skipped and the run continues.
3. The CMake server answers for `src/CMakeLists.txt` and `cmake/Deps.cmake` as
   `cmake`, and not for `notes.txt`.
4. clangd answers for `.m` files as `objective-c` and `.mm` files as
   `objective-cpp`.

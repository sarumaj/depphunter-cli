---
id: REQ-SUP-035
title: Ecosystem prefix on a pattern
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A private pattern **may** be limited to one ecosystem by a prefix naming it
(`npm:@acme/*`), in any case; such a pattern **shall** match only that
ecosystem's packages.

## Rationale

An npm scope and a Maven group are strings that may occur elsewhere.

## Acceptance criteria

1. `npm:@acme/*` matches the npm package `@acme/ui` and not a package of the
   same name in another ecosystem.
2. `localhost:5000/*` is a host and port, not an ecosystem prefix.
3. `NPM:@acme/*` matches the npm package `@acme/ui` as `npm:@acme/*` does.

## Notes

The PowerShell Gallery's ecosystem id `psgallery` was missing from the prefixes,
so `psgallery:Acme.*` was read as a name for every ecosystem; it is now one of
them.

The C/C++ package managers' ids `vcpkg` and `conan` are prefixes as well, and
so are PHP's `composer`, Ruby's `rubygems`, Swift's `swiftpm`, Dart's `pub`,
Elixir's and Erlang's `hex`, R's `cran` and `bioconductor`, Haskell's
`hackage`, Terraform's `terraform-module` and `terraform-provider`, the
Buf Schema Registry's `buf`, CMake's `cmake-fetch` and `pkg-config`,
`cocoapods` and `carthage`, Lua's `luarocks` and `wally`, Perl's
`cpan`, OCaml's `opam`, Julia's `julia`, Zig's `zig`, Bazel's `bazel`
(modules) and `bazel-repo` (WORKSPACE downloads), Nix's `nix` (flake
inputs and pinned sources) and `nixpkgs` (nixpkgs packages), Elm's
`elm`, PureScript's `purescript`, Crystal's `shards`, Paket's `paket`
(GitHub, git and HTTP dependencies), D's `dub`, Fortran's `fpm` and
`fortran-external` (modules no project or known package provides),
Haxe's `haxelib`, Alire's `alire`, Racket's `raco` (catalog, git and
PLaneT packages), and Common Lisp's `quicklisp` (Quicklisp projects and
Qlot's git sources).

A Clojure dependency is a Maven package named `group:artifact`; a `maven:`
(or unscoped) pattern matches it by its group (`maven:com.acme.*` matches
`com.acme.billing:api`) or by `group.artifact` (`com.acme:billing`).

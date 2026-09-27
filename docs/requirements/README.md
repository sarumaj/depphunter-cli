# depphunter requirements specification

This directory holds the normative requirements of depphunter. Each requirement
is a single file, written in formal language, identified by a human-readable
identifier and a UUID, and traced to the source code that implements it and the
tests that verify it.

## Purpose and scope of the product

depphunter is a command-line tool that analyzes the project in a directory and
presents an interactive, isometric "archipelago" map of its code base in the
default web browser. The product has the following goals:

1. It shall enable a user unfamiliar with a repository to establish, within one
   minute, what the repository contains, how large its parts are, what depends
   on what, and what it draws in from outside.
2. It shall allow the user to determine interactively what is shown, and in how
   much detail, without re-running the tool.
3. It shall support any language ecosystem through a pluggable analysis layer.
4. It shall be distributed as a single, pure-Go, cross-platform binary that
   operates offline.

The following are outside the scope of the product at present:

- precise symbol-level call graphs, other than through the optional language
  server layer ([REQ-LSP-001](lsp/REQ-LSP-001-symbol-references-are-opt-in.md));
- editing or refactoring code, and running builds;
- serving the view to remote users; the server is local-only by design
  ([scope `sec`](sec/)).

## Keywords

The keywords **shall**, **shall not**, **should**, **should not** and **may**
are to be interpreted as described in RFC 2119 and RFC 8174 when, and only
when, they appear in bold.

## Requirement file template

Every requirement is one Markdown file under a directory named after its scope,
named `<ID>-<slug>.md`, and follows [TEMPLATE.md](TEMPLATE.md):

```markdown
---
id: REQ-SRV-004
uuid: 5b0e5c8e-6f0c-4a39-9a2e-0d8e1f1d8c7a
title: Graph document carries an entity tag
scope: srv
type: functional
priority: must
status: implemented
verification:
  - unit
  - integration
---

## Statement

The server **shall** ...

## Rationale

...

## Acceptance criteria

1. ...

## Notes

...
```

### Front matter fields

| Field           | Meaning                                                                                 |
|-----------------|-----------------------------------------------------------------------------------------|
| `id`            | `REQ-<SCOPE>-<NNN>`. Stable, never reused. Used in code annotations.                    |
| `uuid`          | Random (version 4) UUID. Immutable, survives a renumbering or a move to another scope.  |
| `title`         | A short noun phrase naming the capability or constraint.                                |
| `scope`         | The functional area; one of the scopes below, equal to the directory name.              |
| `type`          | `functional`, `non-functional`, `interface`, `constraint` or `limitation`.              |
| `priority`      | `must`, `should` or `may`, matching the strongest keyword of the statement.             |
| `status`        | `implemented`, `partial`, `not-implemented`, `superseded` or `withdrawn`.               |
| `superseded_by` | For `superseded` requirements, the identifier(s) of the requirement(s) that replace it. |
| `source`        | Optional. The section(s) of the user documentation that also describe the requirement.  |
| `verification`  | The kinds of test that verify the requirement; see below.                               |

### Verification (test) types

| Type          | Meaning                                                                                                                        |
|---------------|--------------------------------------------------------------------------------------------------------------------------------|
| `unit`        | Go unit tests (`go test ./...`) or Node tests of a single module, without I/O beyond fixtures.                                 |
| `integration` | Tests that exercise several components together: the HTTP server, `git`, the file system, a language server, the built binary. |
| `ui`          | Headless Node tests of the browser modules (`web/uitest/*.test.mjs`).                                                          |
| `extension`   | Node tests of the VS Code extension (`extension/test/*.test.js`).                                                              |
| `e2e`         | Manual or scripted end-to-end runs of the command and the map in a real browser.                                               |
| `manual`      | Human judgement of a visual or interactive quality that no automated test captures.                                            |
| `inspection`  | Review of source, configuration, CI workflows or release artifacts.                                                            |

### Scopes

| Scope        | Area                                                                                                    |
|--------------|---------------------------------------------------------------------------------------------------------|
| `cli`        | The command, its flags, arguments, help, version, logging and exit behavior.                            |
| `cfg`        | Configuration sources, precedence, environment variables and saved view settings.                       |
| `sec`        | Security of the local server and of executed commands.                                                  |
| `dist`       | Distribution, licensing, release engineering, CI and dependency maintenance.                            |
| `mod`        | The graph data model exchanged between analysis, UI, exports and extension.                             |
| `lang`       | The language plugin contract, file scanning, extraction cache and analysis pipeline.                    |
| `go`         | The Go plugin.                                                                                          |
| `js`         | The JavaScript and TypeScript plugin, with Vue, Svelte and Astro components.                            |
| `py`         | The Python plugin.                                                                                      |
| `rs`         | The Rust plugin.                                                                                        |
| `java`       | The Java plugin.                                                                                        |
| `kt`         | The Kotlin plugin.                                                                                      |
| `scala`      | The Scala plugin.                                                                                       |
| `cs`         | The C# plugin.                                                                                          |
| `cpp`        | The C and C++ plugin.                                                                                   |
| `cmake`      | The CMake plugin, presets, content fetched (FetchContent, ExternalProject, CPM), pkg-config.            |
| `php`        | The PHP plugin and Composer.                                                                            |
| `ruby`       | The Ruby plugin and Bundler.                                                                            |
| `swift`      | The Swift plugin, SwiftPM and Xcode's package references.                                               |
| `objc`       | The Objective-C plugin, CocoaPods (Podfile, Podfile.lock, podspecs) and Carthage.                       |
| `dart`       | The Dart plugin, Flutter and pub.                                                                       |
| `beam`       | The Elixir and Erlang plugin, Mix, rebar3 and Hex.                                                      |
| `r`          | The R plugin, R Markdown and Quarto documents, renv, packrat, CRAN and Bioconductor.                    |
| `haskell`    | The Haskell plugin, cabal, hpack, stack and Hackage.                                                    |
| `lua`        | The Lua, Luau and Teal plugin, LuaRocks (rockspecs, luarocks.lock), Wally and Rojo projects.            |
| `perl`       | The Perl plugin, CPAN manifests, Carton's snapshot and MetaCPAN.                                        |
| `ocaml`      | The OCaml plugin, dune, opam manifests and locks, and opam-repository.                                  |
| `julia`      | The Julia plugin, Pkg's projects, manifests and artifacts, and the General registry.                    |
| `zig`        | The Zig plugin, build.zig's module wiring and build.zig.zon packages.                                   |
| `clojure`    | The Clojure, ClojureScript and babashka plugin, deps.edn, Leiningen, shadow-cljs, bb.edn and Clojars.   |
| `bazel`      | The Bazel plugin: BUILD, .bzl, MODULE.bazel and WORKSPACE files, registries and hub repositories.       |
| `nix`        | The Nix plugin: expressions, flakes and flake.lock, niv and npins pins, and nixpkgs packages.           |
| `gleam`      | The Gleam plugin: modules, gleam.toml and manifest.toml, as Hex packages.                               |
| `elm`        | The Elm plugin: modules, elm.json, installed packages in ELM_HOME and the package site.                 |
| `purescript` | The PureScript plugin: modules, spago.yaml, spago.lock, spago.dhall, packages.dhall and bower.json.     |
| `crystal`    | The Crystal plugin: requires, shard.yml, shard.lock, shard.override.yml and shards installed in lib/.   |
| `fsharp`     | The F# plugin: sources and scripts, .fsproj compile order, Paket, and NuGet shared with C#.             |
| `dlang`      | The D plugin: modules, dub.json, dub.sdl, dub.selections.json and packages dub fetched.                 |
| `fortran`    | The Fortran plugin: free and fixed form, fypp, fpm.toml and dependencies fpm fetched.                   |
| `haxe`       | The Haxe plugin: modules, .hxml, haxelib.json, lix pins, Lime project files and installed haxelibs.     |
| `ada`        | The Ada plugin: specs and bodies, GNAT project files, alire.toml, Alire's lock file and fetched crates. |
| `racket`     | The Racket plugin: modules, Scribble documents, info.rkt packages and collections, raco packages.       |
| `terraform`  | The Terraform and OpenTofu plugin, lock files, Terragrunt and the module registries.                    |
| `proto`      | The Protocol Buffers plugin, Buf's configuration and lock files and the Buf Schema Registry.            |
| `shell`      | The shell script plugin (sh, Bash, zsh, bats), direnv and packages scripts install.                     |
| `ps`         | The PowerShell plugin.                                                                                  |
| `ci`         | The continuous-integration plugin (GitHub Actions, GitLab CI, container images).                        |
| `docker`     | The Dockerfile and Compose plugin, and the container-image references it shares with `ci`.              |
| `md`         | Markdown documents as dependencies, and broken-link findings.                                           |
| `sup`        | Supply chain: pinning, transitive resolution, package indexes, private packages.                        |
| `auth`       | Registry credentials.                                                                                   |
| `trc`        | The resolution report.                                                                                  |
| `fnd`        | Findings from scanner reports and the vulnerability database.                                           |
| `hist`       | The git history overlay.                                                                                |
| `lsp`        | Symbol references through language servers.                                                             |
| `srv`        | The local HTTP API and the event stream.                                                                |
| `watch`      | Watch mode and incremental re-analysis.                                                                 |
| `exp`        | Exports: JSON, DOT, GraphML, HTML, PNG and the backpack.                                                |
| `map`        | The isometric map: layout, navigation, selection, side panel, search, filters, colors, styles.          |
| `ui`         | Page-level behavior: introductions, help, reconnection, menus.                                          |
| `a11y`       | Accessibility.                                                                                          |
| `perf`       | Performance.                                                                                            |
| `walk`       | Walk mode: movement, camera, pointer capture, planet, water, health and wind.                           |
| `city`       | The procedural city shared by both views: streets, ramps, bridges, vegetation, facades.                 |
| `tool`       | Walk-mode tools, hands, gestures, projectiles, the tool row and the wheel.                              |
| `hunt`       | The dependency hunt: tagging, bugs, catches, beacons, tracker, backpack, photographs, HUD.              |
| `ext`        | The VS Code extension.                                                                                  |

## Traceability

Code that implements a requirement carries an annotation naming it, in the
comment syntax of its language, directly above the declaration or block it
concerns:

```go
// Implements: REQ-SEC-001, REQ-SEC-002
func (s *Server) ListenAndServe() error {
```

A test that verifies a requirement carries a `Verifies:` annotation instead:

```js
// Verifies: REQ-WALK-012
it('ignores a short drop and kills on a long one', () => {
```

The same form is used in YAML and shell (`# Implements: ...`), in CSS
(`/* Implements: ... */`) and in HTML (`<!-- Implements: ... -->`). Vendored
code, generated files and test data are never annotated.

A step of a CI workflow (`.github/workflows/`) may carry `# Verifies: ...`
where the job is the automated check, such as building and running every
release target.

`node scripts/reqtrace.mjs` reads the requirement files and the annotations and
writes [TRACEABILITY.md](TRACEABILITY.md): for each requirement, where it is
implemented and where it is verified. `node scripts/reqtrace.mjs --check` fails
when an annotation names an unknown requirement, when a requirement file is
malformed or duplicates an identifier or a UUID, or when TRACEABILITY.md is out
of date.

## Known limitations

The following limitations are part of the specification and are recorded as
requirements of type `limitation` in their scopes:

- Java imports name packages rather than artifacts, so the Maven artifact an
  import comes from is found heuristically (package prefixes, naming rules and
  a table of well-known libraries).
- Kotlin and Scala declarations are read from the text of their files, so a
  definition that does not begin in the first column cannot be imported from
  another file.
- GitHub Actions references from github.com and from GitHub Enterprise are one
  ecosystem.
- Build arguments, environment variables and Compose `.env` files are not
  read, so an image reference that depends on one is left as written.
- C and C++ preprocessor conditions other than a literal 0 or 1 are not
  evaluated, so the includes of every platform branch are recorded.
- vcpkg and Conan manifests are read as text: a `conanfile.py` is not run, the
  versions a vcpkg baseline selects are not known, and headers are matched to
  packages by name.
- PHP is read without running Composer: autoloaders, `files` helpers and
  include paths configured at run time are not evaluated, and a package that
  autoloads only by classmap is matched to a namespace by name.
- Ruby is read without running Bundler: run-time `$LOAD_PATH` changes and
  computed requires are not followed, constants resolve only in Rails
  applications, and a require of an undeclared gem is named by heuristics.
- Swift is read without running SwiftPM or Xcode: an Xcode project's targets
  are not read (module directories are guessed by name), `Package.resolved`
  has no package-to-package edges without a checkout, and type references are
  matched by name by a scanner that does not type-check.
- Objective-C is read without Xcode or CocoaPods: header search paths of an
  Xcode project or `.xcconfig` are not read, pods are matched to headers and
  modules by name, private spec repositories are never fetched, and no
  vulnerability database covers CocoaPods or Carthage.
- Dart is read without running pub: generated files that are not committed are
  not seen, and `pubspec.lock` has no package-to-package edges, so only
  `--online` walks past the first level.
- Elixir and Erlang are read without running Mix, rebar3 or the compiler:
  modules that macros define and aliases a package's `__using__` injects are
  not known, calls through variables are not seen, and `rebar.lock` has no
  package-to-package edges.
- R is read without running R: package names and paths computed at run time,
  definitions inside blocks and calls through variables are not seen, calls
  are linked by name, and a Bioconductor package no lock marks is recognized
  only from a curated table.
- Haskell is read without GHC or the C preprocessor: every CPP branch is read,
  Template Haskell is not expanded, a module no project file declares is
  attributed to a package by a curated table and name heuristics, and a
  Stackage snapshot's versions are not known offline.
- Terraform is read without running Terraform, OpenTofu or Terragrunt:
  expressions are not evaluated, so only literal module sources, paths and
  versions resolve, and no vulnerability database covers Terraform modules or
  providers.
- Protocol Buffers are read without protoc or buf: protoc's `-I` flags in
  build scripts are not read (conventional roots stand in for them), and the
  Buf Schema Registry is not asked about a module's dependencies; no
  vulnerability database covers its modules.
- Shell scripts are read without running them: only paths the file itself
  determines are followed (not loops over globs, `eval` or variables set in
  another file), the working directory is guessed, and packages installed with
  system package managers (`apt-get`, `apk`, `brew`) are not read.
- CMake files are read without configuring: conditions, loops and function
  calls are not evaluated, values computed at configure time (fetched
  content's source directories, binary directories) are unknown, and sources'
  includes of fetched content are not attributed to the fetched package.
- Lua is read without running Lua, LuaRocks or Rojo: `package.path` set at run
  time is not read (modules are found under conventional roots), computed
  requires and instances created at run time are not followed, an undeclared
  rock is named by a curated table and heuristics, `luarocks.lock` has no
  rock-to-rock edges, and no vulnerability database covers LuaRocks or Wally.
- Perl is read without running perl: `@INC` changed at run time and computed
  module names are not followed, a distribution without a snapshot is named by
  a curated table and the module's name, `--online` reads the latest release's
  dependencies whatever version is pinned, and no vulnerability database
  covers CPAN.
- OCaml is read without building: names brought into scope by opening a
  submodule or a package's module, by includes of packages or by ppx
  rewriters are not followed, a package's modules are matched by name, dune
  rules are not run, and `--online` reads opam-repository only for packages
  pinned to a version.
- Julia is read without running Julia: includes of computed paths,
  `LOAD_PATH` changes and `@eval`-generated modules are not followed,
  `import A.b` is read as a module path, a version is not resolved without a
  manifest, and registries other than General are known only when installed
  in a depot and hosted on GitHub.
- Zig is read without running the compiler or the build: import names wired
  in loops, by helpers of other packages or to generated modules are dropped,
  there is no Zig registry for `--online`, and no vulnerability database
  covers Zig packages.
- Clojure is read without running Clojure or its build tools: computed
  `project.clj` values and dynamically loaded namespaces are not followed,
  namespaces and classes are attributed to artifacts by a table and naming
  rules, and there is no lock file, so a dependency's own dependencies need
  `--online` (Maven POMs; Clojars after Maven Central).
- Bazel is read without running Bazel: macros are not expanded, computed
  labels and URLs are not evaluated, module extensions other than the Maven,
  pip, Go, npm and crates hubs are not run, a version minimal version
  selection raised is known only from `MODULE.bazel.lock`, and no
  vulnerability database covers Bazel modules or WORKSPACE downloads.
- Nix is read without evaluating it: computed paths, imports of computed
  values, overlays and module options are not followed, nixpkgs packages are
  known only by name and only in package lists, and there is no Nix registry,
  OSV ecosystem or Trivy package type to ask about flake inputs.
- Gleam is read without running gleam: modules of a package that no
  gleam.toml or manifest.toml declares are named by their first segment
  unless gleam downloaded it into build/packages, pre-1.0 `if erlang { }`
  blocks are not read, and an Elixir module named in `@external` is dropped.
- Elm is read without running elm: a module of a package that is not
  installed in ELM_HOME, not in the curated module table and not spelled by
  a listed package's name is dropped, and `elm-tooling.json` is not read.
- PureScript is read without running spago: a module of a package that is
  not installed in `.spago/`, not in the curated table and not spelled by a
  listed package's name is dropped, a package set's versions are not known
  offline, and Dhall beyond records, lists and merges is not evaluated.
- Crystal is read without running the compiler or shards: methods and
  requires a macro writes are not read, only the first branch of a macro
  `{% if %}` decides the nesting after it, `CRYSTAL_PATH` is not read and
  `@[Link]` libraries are not mapped.
- F# is read without type checking or MSBuild evaluation: unqualified names
  an `open` or an `[<AutoOpen>]` module brings in are not linked, MSBuild
  conditions are ignored, an F# `open` of a C# project's namespace is not
  linked to it, and `#r` of an assembly outside the repository is dropped.
- D is read without running the compiler or dub: `mixin` and `static if`
  code is read as written (both branches of `version` blocks count), a
  module of a package dub has not fetched is attributed by a curated table
  or the declared package it spells, every platform's settings count at
  once, dub's settings (`registryUrls`) are not read, and a string import
  needs a literal path.
- Fortran is read without running the compiler, the C preprocessor, fypp or
  fpm: every `#if` branch counts, fypp templates are not expanded (so
  generated module names are unknown), a module of a package fpm has not
  fetched is attributed by a curated table or the declared name it spells,
  and fpm's registry is not asked (no `--online`).
- Haxe is read without running the compiler, haxelib or lix: every `#if`
  branch counts, macros are not run, a qualified name links only to a
  module's file, same-package types used without an import and `import.hx`
  are not linked, a library that is not installed is attributed by declared
  names and a curated table, and lib.haxe.org is not asked (no `--online`).
- Ada is read without running GNAT, GPRbuild or alr: only the first branch
  of a gnatprep `#if` is read, project files are not evaluated (every case
  alternative counts), units generated at build time are dropped or
  unresolved, a unit of a crate Alire has not fetched is attributed by
  declared names and a curated table, and `--online` asks the community
  index only about exact versions.
- Racket is read without running Racket or raco: macros are not expanded,
  `info.rkt` is not evaluated, raco keeps no lock file (only a checksum or a
  git commit pins), a collection no file of the repository has is
  attributed by the base collections, a curated table and the declared
  packages' names, and neither the package catalog nor installed packages
  are read (no `--online`, no `--resolve-depth`).
- pip's keyring is not consulted for credentials.
- Language servers that index slowly may return fewer references within the
  time budget.

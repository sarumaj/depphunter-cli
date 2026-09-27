---
id: REQ-LANG-018
uuid: 58fd56e3-c4f9-45e9-8cea-7268b6bd5069
title: Built-in ignore list
scope: lang
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

When git cannot list the files, the system **shall** walk the directory tree and
**shall** skip directories with the built-in ignored names: `.git`, `.hg`,
`.svn`, `node_modules`, `vendor`, `dist`, `build`, `target`, `bin`, `obj`,
`.venv`, `venv`, `__pycache__`, `.idea`, `.vscode`, `.next`, `.cache`,
`.gradle`, `.tox`, `.mypy_cache`, `.build`, `.dart_tool`, `_build`,
`dist-newstyle`, `.stack-work`, `.terraform`, `.terragrunt-cache`,
`lua_modules`, `_opam`, `.zig-cache`, `zig-cache`, `zig-out`, `zig-pkg`,
`.cpcache`, `.shadow-cljs`, `elm-stuff`, `.spago`, `bower_components` and
`.crystal`, `.fake`, `.dub` and `.haxelib`, an `output` directory beside a
`spago.yaml` or `spago.dhall` (what the PureScript compiler wrote), a `lib`
directory beside a `shard.yml` (what shards installed), `packages` and
`paket-files` directories beside a `paket.dependencies` (what Paket
installed and downloaded), and an `alire` directory beside an `alire.toml`
(Alire's lock file and the crates it fetched); an `output`, `lib`,
`packages` or `alire` directory elsewhere is kept.

## Rationale

Without git's ignore rules a plain walk would otherwise descend into dependency
caches and build output.

## Acceptance criteria

1. Outside a git repository, `node_modules/x/i.js`,
   `.build/checkouts/nio/Package.swift`, `.dart_tool/package_config.json`,
   `_build/dev/lib/shop/ebin/shop.app`, `dist-newstyle/cache/plan.json` and
   `.stack-work/dist/x/Paths_shop.hs`, `.terraform/modules/vpc/main.tf`,
   `.terragrunt-cache/a/b/main.tf`, `_opam/lib/lwt/lwt.mli`,
   `.zig-cache/o/1/cimport.zig`, `zig-out/bin/gen.zig` and
   `zig-pkg/x-0.1.0-AAAA/build.zig.zon`, `.cpcache/1234.basis`,
   `.shadow-cljs/builds/app/x.edn`, `elm-stuff/0.19.1/Main.elm`,
   `.spago/p/prelude-6.0.1/src/Prelude.purs`,
   `bower_components/purescript-maybe/src/Data/Maybe.purs` and
   `.crystal/cache/macro.cr`, `.fake/build.fsx/intellisense.fsx` and
   `.dub/packages/leftpad/1.0.0/leftpad/source/leftpad.d` and
   `.haxelib/format/3,5,0/format/png/Reader.hx` do not
   appear in the graph, nor do `app/output/Main/index.js` beside
   `app/spago.yaml`, `shop/lib/kemal/src/kemal.cr` beside `shop/shard.yml`,
   `fs/packages/Argu/tools/x.fsx` and
   `fs/paket-files/fsharp/FAKE/Globbing.fs` beside `fs/paket.dependencies`,
   and `crate/alire/cache/dependencies/aunit_24.0.0_1a2b3c4d/src/aunit.ads`
   beside `crate/alire.toml`, while `report/output/summary.md`,
   `tools/lib/helper.cr`, `web/packages/app.fs` and `docs/alire/intro.md`
   do.
2. An unreadable subdirectory is skipped without failing the scan.

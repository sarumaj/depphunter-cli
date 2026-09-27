---
id: REQ-LANG-018
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
`.crystal`, `.fake`, `.dub`, `.haxelib`, `compiled` (what `raco make`
writes beside Racket modules) and `.qlot` (what Qlot installs), an
`output` directory beside a
`spago.yaml` or `spago.dhall` (what the PureScript compiler wrote), a `lib`
directory beside a `shard.yml` (what shards installed), `packages` and
`paket-files` directories beside a `paket.dependencies` (what Paket
installed and downloaded), an `alire` directory beside an `alire.toml`
(Alire's lock file and the crates it fetched), a `systems` directory
beside an `ocicl.csv` (the systems ocicl downloaded), `lib`,
`dependencies`, `out` and `cache` directories beside a `foundry.toml` (the
libraries Foundry and Soldeer installed, forge's build output), and
`artifacts`, `cache` and `typechain-types` directories beside a
`hardhat.config.*` (Hardhat's build output, TypeChain's bindings); an
`output`, `lib`, `packages`, `alire`, `systems`, `dependencies`, `out`,
`cache`, `artifacts` or `typechain-types` directory elsewhere is kept.

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
   `.dub/packages/leftpad/1.0.0/leftpad/source/leftpad.d`,
   `.haxelib/format/3,5,0/format/png/Reader.hx`, `compiled/main_rkt.dep`,
   `src/compiled/errortrace/main_rkt.dep` and
   `.qlot/dists/quicklisp/software/alexandria/package.lisp` do not
   appear in the graph, nor do `app/output/Main/index.js` beside
   `app/spago.yaml`, `shop/lib/kemal/src/kemal.cr` beside `shop/shard.yml`,
   `fs/packages/Argu/tools/x.fsx` and
   `fs/paket-files/fsharp/FAKE/Globbing.fs` beside `fs/paket.dependencies`,
   `crate/alire/cache/dependencies/aunit_24.0.0_1a2b3c4d/src/aunit.ads`
   beside `crate/alire.toml`, and
   `cl/systems/alexandria-20240503-8514d8e/alexandria.asd` beside
   `cl/ocicl.csv`, `sol/lib/forge-std/src/Test.sol`,
   `sol/dependencies/forge-std-1.9.2/src/Test.sol`,
   `sol/out/Counter.sol/Counter.json` and
   `sol/cache/solidity-files-cache.json` beside `sol/foundry.toml`, and
   `hh/artifacts/contracts/Token.sol/Token.json`,
   `hh/cache/solidity-files-cache.json` and `hh/typechain-types/index.ts`
   beside `hh/hardhat.config.ts`, while `report/output/summary.md`,
   `tools/lib/helper.cr`, `web/packages/app.fs`, `docs/alire/intro.md`,
   `game/systems/physics.lisp`, `site/cache/page.html` and
   `site/artifacts/report.md` do.
2. An unreadable subdirectory is skipped without failing the scan.

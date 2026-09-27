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
`.cpcache`, `.shadow-cljs`, `elm-stuff`, `.spago` and `bower_components`,
and an `output` directory beside a `spago.yaml` or `spago.dhall` (what the
PureScript compiler wrote; an `output` directory elsewhere is kept).

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
   `.spago/p/prelude-6.0.1/src/Prelude.purs` and
   `bower_components/purescript-maybe/src/Data/Maybe.purs` do not appear in
   the graph, nor does `app/output/Main/index.js` beside `app/spago.yaml`,
   while `report/output/summary.md` does.
2. An unreadable subdirectory is skipped without failing the scan.

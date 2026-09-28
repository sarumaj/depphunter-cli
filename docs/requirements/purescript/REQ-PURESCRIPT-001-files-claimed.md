---
id: REQ-PURESCRIPT-001
title: PureScript files claimed
scope: purescript
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The PureScript plugin **shall** claim PureScript modules (`.purs`), spago's
`spago.yaml` and `spago.lock`, legacy spago's `spago.dhall`, `packages.dhall`
and other Dhall configurations named for spago (`spago-*.dhall`,
`test.dhall`), and `bower.json`, telling them apart from other YAML, JSON
and Dhall files by name (the predicate the Dhall plugin shares, so no
Dhall file is claimed twice), and **shall not** claim anything under
`.spago/` or `bower_components/`, where spago and bower install packages. The scanner
**shall** label `.purs` files PureScript and `.dhall` files Dhall.

## Rationale

spago 0.93 and later configure a project in `spago.yaml` and lock it in
`spago.lock`; spago 0.20 used Dhall, and older projects bower. What is
installed is not part of the project.

## Acceptance criteria

1. `src/Main.purs`, `spago.yaml`, `spago.lock`, `spago.dhall`,
   `packages.dhall`, `test.dhall` and `bower.json` are claimed;
   `config/app.dhall`, `.spago/p/prelude-6.0.1/src/Prelude.purs` and
   `bower_components/purescript-maybe/bower.json` are not.

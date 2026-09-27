---
id: REQ-PURESCRIPT-005
uuid: 910870da-d743-4ee7-9ee7-3df17dcdd662
title: Manifests read
scope: purescript
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The PureScript plugin **shall** read `spago.yaml` (`package.name`,
`package.dependencies` and `package.test.dependencies` as names or
`name: range`, `workspace.packageSet` as `registry:` or `url:`, and
`workspace.extraPackages` from the registry, git or a path), `spago.lock`
(YAML or JSON: each package's type, version, git URL and revision, path and
dependencies, and the workspace's packages), `spago.dhall` and
`packages.dhall` with a Dhall reader of its own, and the `purescript-*`
dependencies of `bower.json`. Each package a manifest lists, and each
package a lock records, **shall** be an import of that package. The Dhall
reader **shall** evaluate records, lists of strings, `let`, field selection,
`//`, `#`, `with`, `?`, `Some`, `mkPackage` and imports of local files, and
treat anything else (functions, unions, interpolated text) as unknown; a
remote import is not fetched and names the package set.

## Rationale

spago 0.93 and later write YAML; spago 0.20 configurations are Dhall
programs, but in practice records, lists and a remote package set extended
with `//` or `with`. The packages a manifest lists are installed whether or
not a module imports them.

## Acceptance criteria

1. The fixture's `spago.yaml`, `spago.lock`, `legacy/spago.dhall`,
   `legacy/packages.dhall`, `legacy/test.dhall` (which extends
   `spago.dhall`) and `bower.json` list the packages of the tests; a
   manifest spago or bower would not read yields nothing.

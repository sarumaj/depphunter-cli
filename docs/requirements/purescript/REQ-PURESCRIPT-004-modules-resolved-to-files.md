---
id: REQ-PURESCRIPT-004
uuid: 8d52321c-043f-42fa-ad90-ebe9fc43e32a
title: Modules resolved to files
scope: purescript
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The PureScript plugin **shall** resolve an imported module to the file that
declares it (by its `module` header, wherever it is), preferring a file of
the importing file's own package, then one of a package of the same
workspace, and **shall** resolve a foreign import to the `.js` file of the
same name beside the module. A package's files are those its source globs
match: `src/**/*.purs` and, for tests, `test/**/*.purs` of a `spago.yaml`
package or a `bower.json`, and the `sources` of a `spago.dhall`, read
relative to the configuration's directory or, when the glob names a
directory only an ancestor has, relative to that ancestor. A module whose
package is a local package (a workspace package, a `packages.dhall` entry
whose repository is a path) **shall** resolve to that package's file.

## Rationale

PureScript does not tie a module's name to its path, and monorepos and
examples read their sources from other directories.

## Acceptance criteria

1. `Shop.Cart` from `src/Main.purs` and from `test/Test/Main.purs` is
   `src/Shop/Cart.purs`; `Widgets.Button` is the workspace package's file
   from the spago.yaml project and from the legacy project; `foreign import`
   in `src/Main.purs` is `src/Main.js`.

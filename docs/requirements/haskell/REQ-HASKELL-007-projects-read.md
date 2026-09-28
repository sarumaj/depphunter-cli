---
id: REQ-HASKELL-007
title: Projects read
scope: haskell
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read `cabal.project` (`packages:`, `optional-packages:`,
`constraints:`, `source-repository-package` stanzas, conditional blocks
included, and the same of the local project files its `import:` fields name,
relative to the importing file, each once; a URL or a path outside the
repository is not read), `cabal.project.freeze` and `stack.yaml` (`resolver:`
or `snapshot:`, `packages:`, `extra-deps:` as `name-version[@sha256|@rev]`,
`git`/`github` with `commit` and `subdirs`, archive URLs and local paths) of
the nearest project directory above a package, a package with none above it
being its own project.

## Rationale

A cabal or stack project, not the package, decides which versions are
installed.

## Acceptance criteria

1. The fixture's `source-repository-package` for `conduit` pins it to its
   commit with its repository as origin; the one at tag `v1.2` floats.
2. A `constraints:` entry `hashable ==1.4.4.0` pins `hashable`.
3. `aeson ==2.2.1.0` in `cabal.project.common`, which `cabal.project`
   imports (and which imports itself), pins `aeson`; its `packages: extra/`
   and the packages `cabal.project`'s globs match are the project's; a
   garbage or missing imported file pins nothing.

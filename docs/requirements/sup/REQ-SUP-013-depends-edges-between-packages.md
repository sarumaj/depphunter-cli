---
id: REQ-SUP-013
title: Package-to-package edges
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

An edge from one package to another **shall** be of kind `depends`, distinct
from the `import` edges from files, so that the importer count of a package
remains a count of files.

## Rationale

Mixing package edges into imports would inflate the count of files importing a
package.

## Acceptance criteria

1. After a walk, file-to-package edges are `import` and package-to-package edges
   are `depends`.
2. The importer count of a package equals the number of files importing it.
3. With `--resolve-depth -1` and no network, a JavaScript project's lock file
   (`package-lock.json`, `yarn.lock` classic or Berry, `pnpm-lock.yaml`)
   yields exactly the `depends` edges between npm packages that the lock
   records, between package names.

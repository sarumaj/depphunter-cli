---
id: REQ-SUP-013
uuid: 01e59710-92c9-4239-9fda-34bbd59f831a
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

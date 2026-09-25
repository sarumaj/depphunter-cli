---
id: REQ-MAP-008
uuid: 7fa2dd23-8e84-4b84-b3a8-44b0e326c691
title: Package height follows number of importers
scope: map
type: functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

The system **shall** draw every external package as a building on its
ecosystem's island whose height is given by the number of files that import it
(package-to-package `depends` edges not counted), on the selected height scale,
with the most imported package at 5.3 units.

## Rationale

On an island, the interesting size is how much of the project leans on a
package, not the package's own code, which is not analyzed.

## Acceptance criteria

1. A package imported by more files is taller than one imported by fewer.
2. Adding a package-to-package `depends` edge does not change a package's
   height.

---
id: REQ-R-007
title: Bioconductor packages kept apart
scope: r
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** put a package into a `bioconductor` ecosystem, apart
from `cran`, when a lock records it as installed from Bioconductor (renv
`Source: Bioconductor` or a Bioconductor repository, packrat
`Source: Bioconductor`), when a `DESCRIPTION`'s `Remotes` names it with
`bioc::`, or when it is one of Bioconductor's infrastructure and most-used
packages (a curated table).

## Rationale

Bioconductor is a repository of its own with its own releases and its own OSV
ecosystem; a CRAN package of the same name would be another package.

## Acceptance criteria

1. `DESeq2` and `S4Vectors` from renv.lock and `BiocGenerics` from the
   table resolve to `bioconductor`; OSV is asked in its Bioconductor
   ecosystem.

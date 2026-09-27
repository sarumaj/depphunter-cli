---
id: REQ-R-008
uuid: 9b95c034-a6c6-42da-a2ac-70e46b51e0a4
title: renv.lock and packrat.lock read
scope: r
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read the nearest `renv.lock` (a project's or a
package's, from the file list or from disk) or `packrat/packrat.lock` above a
file: each package's version and source (repository, Bioconductor, GitHub,
GitLab, Bitbucket or git with its commit, local), and answer `--resolve-depth`
from what the lock records a package requires (`Requirements`; renv 1.1's
`Depends`, `Imports` and `LinkingTo`; packrat's `Requires`), each dependency
as the same lock pinned it, base packages left out.

## Rationale

renv records the whole resolved graph, so no index needs to be asked.

## Acceptance criteria

1. dplyr 1.1.4 depends on R6, cli, glue and rlang as locked and on generics,
   lifecycle, magrittr and tibble unversioned; readr's renv 1.1 `Imports`
   are read; httr from packrat.lock is answered by its commit.

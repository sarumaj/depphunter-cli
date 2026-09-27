---
id: REQ-R-009
uuid: 1654645e-bfd7-431d-a7e3-6585ae2ea778
title: CRAN pinning rule
scope: r
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** pin a package that a lock records from a repository
(at the locked version, the `DESCRIPTION` requirement kept as requested) or
from GitHub by a commit, and a `DESCRIPTION` requirement `(== x)`; a
requirement `(>= x)` floats, a declaration without a version floats, and a
`Remotes` entry pins only when its ref is a commit; a locally installed
package carries its path as its origin and is not pinned.

## Rationale

R's `==` is the only requirement naming one version; install.packages always
takes the newest otherwise.

## Acceptance criteria

1. `R6 (== 2.5.1)` is pinned at 2.5.1, `dplyr (>= 1.1.0)` is not, `rlang`
   floats, glue from `Remotes: tidyverse/glue@<sha>` is pinned by the commit.

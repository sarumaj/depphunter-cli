---
id: REQ-R-006
uuid: f72b8aba-2347-48bf-befd-14ec879c0e2a
title: Package names resolved to the repository and to R
scope: r
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** resolve a package name to: nothing, from a file of the
importing package's own `R/` directory; that package's `R/` directory, from its
tests, vignettes and other files; another package of the repository (a
`DESCRIPTION` with that `Package:`), its `R/` directory from code and its
`DESCRIPTION` from a `DESCRIPTION`; the hidden `r-std` island for R's base
packages (base, compiler, datasets, grDevices, graphics, grid, methods,
parallel, splines, stats, stats4, tcltk, tools, utils) and for a recommended
package (MASS, lattice, Matrix, nlme, survival, boot, cluster, codetools,
foreign, KernSmooth, rpart, class, nnet, spatial, mgcv) that the nearest
`DESCRIPTION` does not declare and the nearest lock does not hold; and
otherwise to CRAN or Bioconductor (REQ-R-007, REQ-R-009), unresolved when
nothing declares or locks it.

## Rationale

Recommended packages ship with R but are versioned and updated on CRAN: a
project that declares or locks one depends on that version.

## Acceptance criteria

1. `library(survival)` goes to `r-std`, `library(MASS)` with MASS in
   renv.lock to CRAN pinned; `library(shopr)` in shopr's tests goes to
   `pkgs/shopr/R`, `shopr:::` inside `pkgs/shopr/R` is dropped.
2. `requireNamespace("jsonlite")` with no manifest naming it is unresolved.

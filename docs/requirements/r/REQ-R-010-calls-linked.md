---
id: REQ-R-010
title: Calls linked within a package or project
scope: r
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** link a file to the file of its package (or, outside
packages, of its project: the directory of an `.Rproj`, `renv.lock`,
`_targets.R` or `.here`, else the repository) that defines a function or class
it calls - directly, through `Class$new()`, or handed to `do.call()`, the apply
family or purrr's map functions - preferring a definition in the caller's own
directory, then in the package's `R/`, and linking nowhere when the name is
defined in several other places; calls to common base functions are not
recorded.

## Rationale

Files of an R package have no imports of each other: every function of
`R/` is visible in every file, so calls are the only thing that ties them.

## Acceptance criteria

1. `total()` in shopr's tests and vignette resolves to `R/cart.R`,
   `vapply(..., price_of, ...)` in `R/cart.R` to `R/utils.R`,
   `load_data()` in `analysis/run.R` to `analysis/R/load.R`.

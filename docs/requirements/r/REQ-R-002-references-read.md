---
id: REQ-R-002
uuid: 9a382554-6018-439a-b43b-900212752739
title: Package and file references read
scope: r
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read as imports: `library()` and `require()` of a name
or string (not of a variable with `character.only = TRUE`),
`requireNamespace()`, `loadNamespace()` and `attachNamespace()` of a string,
`pacman::p_load()`, every `pkg::` and `pkg:::` qualifier, roxygen `@import`
and `@importFrom` tags, `box::use()` packages and local modules (`./mod`,
`app/logic/mod`), `source()`, `sys.source()` and `Rcpp::sourceCpp()` of a path
spelled as a string, `file.path()`, `paste0()` or `here::here()`,
`targets::tar_source()` and roxygen `@include` - and nothing inside comments,
strings or raw strings.

## Rationale

R has no import statement: packages are attached, loaded or qualified in
several ways, and scripts are stitched together with `source()`.

## Acceptance criteria

1. In the fixture's `analysis/run.R`, every form above is captured and
   `library(pkg, character.only = TRUE)` is not.
2. `# library(x)`, `"library(x)"` and `r"-(library(x))-"` import nothing.

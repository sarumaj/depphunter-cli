---
id: REQ-R-001
title: R files, documents and package manifests claimed
scope: r
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The R plugin **shall** claim R sources (`.R`, `.r`, `.Rprofile`), R Markdown
and Quarto documents (`.Rmd`, `.qmd`) and the `DESCRIPTION` and `NAMESPACE`
files of packages, and **shall not** claim what renv and packrat install into a
project (`renv/library`, `renv/staging`, `renv/local`, `renv/sandbox`,
`renv/python`, `packrat/lib`, `packrat/lib-R`, `packrat/lib-ext`,
`packrat/src`). `DESCRIPTION` and `NAMESPACE`, which have no extension, are
told apart by name in the extraction cache key.

## Rationale

R code lives in scripts, packages and documents alike; a package's
dependencies are declared in its `DESCRIPTION` and `NAMESPACE`. An installed
library is somebody else's code.

## Acceptance criteria

1. `R/cart.R`, `script.r`, `doc.Rmd`, `report.qmd`, `.Rprofile`, `DESCRIPTION`
   and `NAMESPACE` are claimed; `renv/library/.../R/zzz.R` and
   `packrat/lib/.../R/a.R` are not.
2. `DESCRIPTION` and `NAMESPACE` have different cache classes.

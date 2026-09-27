---
id: REQ-R-011
title: R read without running R
scope: r
type: limitation
priority: must
status: implemented
verification:
  - manual
---

## Statement

The plugin **shall not** run R, renv or knitr: package names computed at run
time (`library(pkg, character.only = TRUE)`, `lapply(pkgs, library, ...)`),
paths built from variables, functions defined by assignment inside blocks
(`local()`, `if`), S3 dispatch and calls through variables are not seen;
calls are linked by name only; a Bioconductor package that no lock or
`Remotes` entry marks and the curated table does not list is taken for a CRAN
package; and a CRAN mirror other than cloud.r-project.org, `*.r-project.org`,
cran.rstudio.com and Posit Package Manager is taken for a repository of the
project's own.

## Rationale

depphunter reads repositories statically and never executes their code.
The vendored tree-sitter R grammar was measured first on dplyr, usethis,
shiny-examples, renv and rhino-showcase (1047 files): it parsed well (2 files
with ERROR nodes, both roxygen example files) at 0.5 to 3 ms per file on
average (435 ms for dplyr's 195 files, 595 ms for renv's 358). Everything the
plugin needs is token-level, and its own lexer reads the same files about
nine times faster (49 ms for dplyr, 58 ms for renv) without vendoring the
grammar.

## Acceptance criteria

1. `library(pkg, character.only = TRUE)` with `pkg` a variable is not an
   import.

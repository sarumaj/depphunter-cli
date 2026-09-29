---
id: REQ-SUP-048
title: R package dependencies from crandb, CRAN-like repositories and Bioconductor
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The index client **shall** read an R package's dependencies (`Depends`,
`Imports`, `LinkingTo`, without R and its base packages) from crandb
(`https://crandb.r-pkg.org/<name>/<version>` for a pinned version, else
`/<name>`) when the package's index is CRAN (`https://cloud.r-project.org`
unless configured otherwise), a `*.r-project.org` mirror, cran.rstudio.com or
Posit Package Manager's CRAN, and from `<repository>/src/contrib/PACKAGES`,
read once per repository, for any other CRAN-like repository; a package that
`PACKAGES` does not list is not there. The repositories of `renv.lock` (those
other than CRAN also scoped to the packages recorded from it) and those of
`options(repos = ...)` in a project `.Rprofile` are the repository's; those of
`~/.Rprofile`, `R_PROFILE_USER` and `RENV_CONFIG_REPOS_OVERRIDE` are this
machine's. Each such list (`repos = "url"`, or `c(...)` of string literals,
named or not; `renv.lock`'s `R.Repositories`; the override's entries) **shall**
be asked in order, as R asks every repository of its `repos` option, and CRAN
**shall** be asked only when the list names it (a known mirror or the
`@CRAN@` placeholder, both standing for CRAN); a `repos` value with parts that
are not literals (`c(getOption("repos"), internal = "url")`) **shall** add its
literal repositories beside CRAN instead. Bioconductor's repositories and
local ones (`file:`) are not CRAN indexes. CRAN and its known mirrors are
never recorded as a repository's own index. A package of the `bioconductor`
island **shall** be answered from its Bioconductor release's software repository,
`https://bioconductor.org/packages/<release>/bioc`, through the same
`PACKAGES` reader: the release `renv.lock` records
(`Bioconductor.Version`, the first `renv.lock` in path order that records
one), else `release`. A dependency that `PACKAGES` lists is a Bioconductor
package; any other is CRAN's. Every release's repository on
bioconductor.org is Bioconductor's public index.

## Rationale

CRAN publishes only its current releases' metadata; crandb serves every
release's `DESCRIPTION` as JSON. Every CRAN-like repository (drat, r-universe,
an internal one) serves a `PACKAGES` index. An internal repository is not
guessed to be a CRAN mirror by its name, so its packages are never named to
crandb.

## Acceptance criteria

1. Against a stub crandb, dplyr 1.1.4 depends on R6, cli `>= 3.4.0`,
   generics and vctrs `>= 0.6.4`; a requirement naming no release, or a
   release crandb does not have, gets the current release's dependencies.
2. Against a stub repository, `PACKAGES` is fetched once and answers for two
   packages.
3. renv.lock's internal repository serves the package recorded from it alone
   and is asked after CRAN for the others; Posit Package Manager is taken for
   CRAN. A `repos` list without CRAN, in `~/.Rprofile` or
   `RENV_CONFIG_REPOS_OVERRIDE`, switches CRAN off; one extending
   `getOption("repos")` keeps it.
4. With `Bioconductor.Version` `3.18` in renv.lock, DESeq2 is attributed to
   and asked of the 3.18 repository, whose `PACKAGES` is fetched once: it
   depends on S4Vectors and BiocGenerics (Bioconductor) and on Rcpp,
   RcppArmadillo and ggplot2 (CRAN). Without a valid version the `release`
   repository is asked.
5. With `repos = c(CRAN = "@CRAN@", corp = "<corp>")`, a package CRAN's
   `PACKAGES` lacks is answered by corp, and one CRAN has is never named to
   corp.

## Notes

R keeps the highest version any repository offers (the first repository on a
tie, `available.packages`' `duplicates` filter); depphunter takes the first
repository that has the package. `options(repos = r)` with `r` built in
earlier statements (`r["CRAN"] <- "url"`) is not followed, and only the last
profile R reads would count: every one found is read here.

The R version renv.lock records is not mapped to a Bioconductor release:
each R version is served by two releases. A package of Bioconductor's
annotation or experiment-data repositories is not asked there; one such
dependency is taken for CRAN's.

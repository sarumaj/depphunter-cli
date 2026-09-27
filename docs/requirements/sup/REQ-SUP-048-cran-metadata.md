---
id: REQ-SUP-048
title: R package dependencies from crandb and CRAN-like repositories
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
read once per repository, for any other CRAN-like repository. The repositories
of `renv.lock` (each scoped to the packages recorded from it) and the literal
URLs of `options(repos = ...)` in a project `.Rprofile` are the repository's;
those of `~/.Rprofile`, `R_PROFILE_USER` and `RENV_CONFIG_REPOS_OVERRIDE` are
this machine's. CRAN and its known mirrors are never recorded as a
repository's own index.

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
3. renv.lock's internal repository serves only the package recorded from it;
   Posit Package Manager is taken for CRAN.

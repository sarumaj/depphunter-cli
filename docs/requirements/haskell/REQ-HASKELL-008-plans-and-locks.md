---
id: REQ-HASKELL-008
uuid: bc4547a9-0658-40c6-ae88-91eb89a50333
title: Build plans, freeze files and stack locks read
scope: haskell
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read `cabal.project.freeze` (`any.pkg ==x`, not
`installed`, flag or setup constraints), cabal's build plan
`dist-newstyle/cache/plan.json` (every package's version, a
source-repository's location and tag, and what each unit depends on) and
`stack.yaml.lock` (completed Hackage and repository entries), from the file
list or from disk when git-ignored; and answer `--resolve-depth` from the build
plan: a package's dependencies at the versions the plan chose, without GHC's
own packages and the project's.

## Rationale

Freeze files and locks are how a Haskell project pins; the plan is the only
file that records edges between packages.

## Acceptance criteria

1. aeson 2.2.3.0 depends on conduit (at its commit), containers 0.6.7 and
   text 2.0.2 by the fixture's plan; another version of aeson gets no answer.
2. `mtl` pinned by the freeze file shows `2.3.1`, requested `>=2.2 && <2.4`.

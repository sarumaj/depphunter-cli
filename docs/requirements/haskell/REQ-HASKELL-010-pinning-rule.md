---
id: REQ-HASKELL-010
title: Hackage pinning rule
scope: haskell
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A Hackage package **shall** be pinned at the version the build plan, the
freeze file or an exact `cabal.project` constraint, `stack.yaml.lock` or an
`extra-deps` entry fixes (the declared range kept as requested), and at a
repository's commit (a tag or branch floats); else as `build-depends` declares
it: `==1.2.3` pins (shown bare), a range or `^>=` does not, no version floats
unless a stack snapshot fixes it, when its version, unknown offline, is shown
as the snapshot's name and it neither pins nor floats. A package nothing
declares or pins is unresolved. OSV is asked about pinned packages in its
`Hackage` ecosystem.

## Rationale

A cabal range or caret is resolved anew by every build without a freeze file;
a Stackage snapshot is immutable, but which version it holds is only known by
fetching it.

## Acceptance criteria

1. `aeson ^>=2.2` with the plan's 2.2.3.0 is pinned at 2.2.3.0, requested
   `^>=2.2`; `deepseq ==1.4.8.1` is pinned at 1.4.8.1.
2. In the stack project, `text` with no version is `lts-22.43`, neither pinned
   nor floating; `http-client` in the cabal project floats.

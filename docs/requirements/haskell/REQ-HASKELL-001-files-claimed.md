---
id: REQ-HASKELL-001
uuid: 6d222d38-7510-475b-bd9d-426491a227f4
title: Haskell sources, package descriptions and projects claimed
scope: haskell
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Haskell plugin **shall** claim Haskell modules (`.hs`), literate modules
(`.lhs`), boot files (`.hs-boot`) and hsc2hs sources (`.hsc`), package
descriptions (`*.cabal` and hpack's `package.yaml`) and project files
(`cabal.project`, `stack.yaml`), and **shall not** claim what cabal and stack
build into `dist-newstyle` and `.stack-work`. `package.yaml`, `stack.yaml` and
`cabal.project` are told apart from other files of their extension by name in
the extraction cache key.

## Rationale

A Haskell package declares its dependencies in its package description and
a project pins them; generated build output is not the project's code.

## Acceptance criteria

1. `src/A.hs`, `doc/B.lhs`, `src/A.hs-boot`, `cbits/C.hsc`, `shop.cabal`,
   `cabal.project`, `package.yaml` and `stack.yaml` are claimed;
   `dist-newstyle/build/x/A.hs` and `.stack-work/dist/x/Paths_a.hs` are not.
2. `package.yaml` does not share a cache class with `config.yaml` or
   `stack.yaml`.

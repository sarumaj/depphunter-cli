---
id: REQ-HASKELL-011
title: Haskell read without GHC
scope: haskell
type: limitation
priority: must
status: implemented
verification:
  - manual
---

## Statement

The plugin **shall not** run GHC, cabal, stack, hpack or the C preprocessor:
imports and declarations of every CPP branch are read, Template Haskell and
quasi-quoted code is not expanded, the layout rule is not applied (top-level
declarations are those starting a line in the leftmost column), a module no
project file declares is attributed to a package by a curated table and name
heuristics, cabal globs in `packages:` are not expanded, conditionals of
package descriptions are all taken, a Stackage snapshot is not fetched (its
packages' versions are unknown offline), and Trivy has no Haskell package type.

## Rationale

depphunter reads repositories statically and never executes their code. The
vendored tree-sitter Haskell grammar was measured first on shallow clones of
haskell/aeson, jgm/pandoc, commercialhaskell/stack and PostgREST/postgrest (1391
`.hs` files): 6 to 45 ms per file on average (16.5 s for pandoc's 365 files) and
253 files (18%) with ERROR nodes, mostly CPP and extensions. Everything the
plugin needs is token-level; its own lexer (nested comments, pragmas, string
gaps, character literals versus primes and promotion ticks, CPP lines) is
fast enough that the whole analysis of each of those repositories, every other
plugin included, takes 0.08 to 0.4 s.

## Acceptance criteria

1. `import` lines in both branches of `#if`/`#else` are both read (once when
   identical).

---
id: REQ-RACKET-011
title: Read without Racket or raco
scope: racket
type: limitation
priority: should
status: implemented
verification:
  - unit
---

## Statement

Racket **shall** be read without running Racket or raco: macros are not
expanded (a require a macro generates, or one inside a non-standard form
wrapping module-level code, is not seen), `info.rkt` is not evaluated, no
lock file exists (raco has none), the catalog (pkgs.racket-lang.org) is
not asked (no `--online`), installed packages are not read (no
`--resolve-depth` beyond the declared packages), and a collection no file
of the repository has is attributed by the base list, a curated table and
the declared packages' names.

## Rationale

Which modules a macro requires and which package installed a collection
are known only to Racket's expander and raco's installation.

## Acceptance criteria

1. A require inside `@racketblock[...]` or a quoted list is not read.

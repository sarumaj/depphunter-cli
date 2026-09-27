---
id: REQ-RACKET-001
uuid: d56e36aa-ea35-480a-8aaf-e93db72eee1a
title: Files claimed
scope: racket
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Racket plugin **shall** claim Racket modules (`.rkt`), load files
(`.rktl`), data files (`.rktd`, claimed with nothing to link), Scribble
documents (`.scrbl`) and packages' `info.rkt` (told apart from modules by
its name), and a Scheme file (`.scm`, `.ss`) or a script without an
extension only when the scan labels it Racket: a `#lang` line (or
`#!lang`) after an optional `#!` line, blank lines and `;` comments, or a
`#!` line that runs `racket`. Nothing in a `compiled/` directory (what
`raco make` writes) **shall** be claimed, and a walk of the file system
**shall** not enter one.

## Rationale

Racket reads `.scm` and `.ss` files only through a `#lang` line; without
one they are Chez Scheme, Guile or R6RS programs, which Racket's resolution
does not describe. `compiled/` holds bytecode (`.zo`) and dependency files
(`.dep`), never source.

## Acceptance criteria

1. The fixture's modules, load files, data file, Scribble documents, both
   `info.rkt` files, `scheme/legacy.scm` (with `#lang racket`) and
   `bin/shop-cli` are claimed; `scheme/chez.ss`, `scheme/r6rs.scm` and
   `compiled/stale.rkt` are not.
2. A binary `.rkt`, a `.ss` labelled Scheme and `src/compiled/errortrace/a.rkt`
   are not claimed; `compiledx/a.rkt` is.

---
id: REQ-RACKET-004
uuid: 035ae872-2275-409d-a902-b7a3d9c4b934
title: Relative paths resolved to files
scope: racket
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A relative path string, a relative `(file ...)`, `(submod "file.rkt" ...)`,
an `include` or `load` path **shall** resolve to the repository's file
relative to the importing file (a `.ss` path to the `.rkt` file beside it
when that exists, a path without a suffix to its `.rkt`), and `(path-up
"x.rkt")` to the nearest such file in the importing file's directory or
above. A missing file, an absolute `(file ...)` path and a submodule of the
file itself (`(submod "." x)`, `'x`) **shall** be dropped.

## Rationale

Racket resolves a string module path relative to the enclosing module's
file.

## Acceptance criteria

1. `require "util.rkt"` in `main.rkt` resolves to `util.rkt`,
   `require "../main.rkt"` in `bin/shop-cli` to `main.rkt`,
   `(path-up "util.rkt")` in `sub/helpers.rkt` to `util.rkt`.
2. `(file "/opt/racket/shared.rkt")` and `(submod "." inner)` are dropped.

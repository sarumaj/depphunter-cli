---
id: REQ-RACKET-009
title: Islands
scope: racket
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** declare two islands: `raco` ("Racket packages";
`raco:` is a private-pattern prefix) and `racket-std` ("Racket base
collections", hidden like other standard libraries), and emit no other.

## Rationale

Catalog packages are what a Racket project depends on; the base
collections ship with every Racket.

## Acceptance criteria

1. Every fixture import resolves to a file, `raco`, `racket-std` or
   nothing.

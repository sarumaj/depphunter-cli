---
id: REQ-RACKET-012
title: Package catalog entries
scope: racket
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read a package catalog's entry for a package (a `read`-able
`#hash`, `#hasheq` or `#hasheqv` of `(key . value)` pairs) and return its
`dependencies` - or, without them, those of its `versions` table's `default`
entry - as it resolves an info.rkt's `deps` entries (REQ-RACKET-006). An answer
that is not a hash table **shall** report that it is not an entry.

## Rationale

The index client (REQ-SUP-070) asks catalogs; the plugin already knows how raco
reads a dependency.

## Acceptance criteria

1. See REQ-SUP-070.

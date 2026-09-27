---
id: REQ-RACKET-008
title: PLaneT packages
scope: racket
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

A `(planet owner/pkg:major:minor/file)` or `(planet "file.rkt" ("owner"
"pkg.plt" major minor))` module path **shall** resolve to the `raco`
package `planet/owner/pkg`, with the version `major.minor`, floating.

## Rationale

PLaneT, Racket's first package system, installs packages on demand; a
minor version is a minimum.

## Acceptance criteria

1. `(planet jaymccarthy/sqlite:5:1)` resolves to `planet/jaymccarthy/sqlite`
   version 5.1, floating.

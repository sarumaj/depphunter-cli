---
id: REQ-DHALL-001
title: Files claimed
scope: dhall
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Dhall plugin **shall** claim `.dhall` files, except the Dhall files of
spago that the PureScript plugin reads (`packages.dhall`, `spago.dhall`,
other files whose name contains `spago`, and `test.dhall`), told apart by
one predicate both plugins share.

## Rationale

A spago configuration is a PureScript manifest written in Dhall; two
plugins claiming it would put its imports on the map twice.

## Acceptance criteria

1. `package.dhall`, `config/app.dhall` and `types/Deployment.dhall` are
   claimed; `spago.dhall`, `packages.dhall`, `test/test.dhall` and
   `spago-test.dhall` are not, and are exactly the files the PureScript
   plugin classifies as spago's.

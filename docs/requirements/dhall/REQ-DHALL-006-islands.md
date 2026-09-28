---
id: REQ-DHALL-006
title: Islands
scope: dhall
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

Remote Dhall imports **shall** form the "Dhall packages" island, with
`dhall:` as a private-pattern prefix. No vulnerability database and no
registry **shall** be asked about them.

## Rationale

OSV has no Dhall ecosystem, and Dhall has no package registry: a package
is a URL.

## Acceptance criteria

1. The plugin declares the one island, and OSV's ecosystem map has no entry
   for it.

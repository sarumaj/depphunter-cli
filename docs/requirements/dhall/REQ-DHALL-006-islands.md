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
`dhall:` as a private-pattern prefix. No registry **shall** be asked about
them, and the vulnerability database only about the commit an import's URL
names, when it names a full one of a repository on a public forge
(REQ-FND-026).

## Rationale

OSV has no Dhall ecosystem, and Dhall has no package registry: a package
is a URL.

## Acceptance criteria

1. The plugin declares the one island, and OSV's ecosystem map has no entry
   for it.

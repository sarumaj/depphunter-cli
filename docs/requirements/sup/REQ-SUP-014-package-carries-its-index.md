---
id: REQ-SUP-014
uuid: 857bebda-a922-4330-9fe2-475ebd492410
title: Every package carries its index
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
  - manual
---

## Statement

The system **shall** record on every external package the URL of the package
index it resolves from, except one installed from outside any index
([REQ-PY-015](../py/REQ-PY-015-installed-packages-no-index-has.md)), and the
side panel **shall** show that index's host.

## Rationale

Which index a package resolves from decides whether naming it discloses anything
and whether it may be fetched at all.

## Acceptance criteria

1. A package of an ecosystem with a configured index carries that index;
   otherwise it carries the ecosystem's public default.
2. A scoped source (an npm scope, a Maven group prefix) serves only the packages
   it covers, ahead of an unscoped one.
3. This holds with `--resolve-depth 0`.

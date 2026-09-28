---
id: REQ-SUP-014
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

1. A package of an ecosystem with a configured index that replaces the public
   default carries that index; otherwise it carries the ecosystem's public
   default, also when a source is only asked beside it
   ([REQ-SUP-063](REQ-SUP-063-additive-sources-fall-back-to-the-public-index.md)).
2. A scoped source (an npm scope, a Maven group prefix) serves only the packages
   it covers, ahead of an unscoped one; a Cargo alternative registry only the
   crates that declare it.
3. A package matching a private pattern carries the first index that would
   serve it and is not a public one.
4. With `--online`, a package carries the index that answered for it, or the
   repository's index when none of the indexes asked had it.
5. This holds with `--resolve-depth 0` (criteria 1 to 3).

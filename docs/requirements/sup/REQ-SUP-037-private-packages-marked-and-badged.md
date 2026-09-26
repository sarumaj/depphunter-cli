---
id: REQ-SUP-037
uuid: 2b4dc5dd-36ac-41af-ad94-6bc681eb0c84
title: Private packages are marked
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
  - manual
---

## Statement

A package matched as private **shall** carry `private` on the graph, and the
side panel **shall** show a "private" badge for it.

## Rationale

Whoever reads the map should see what depphunter treats as internal.

## Acceptance criteria

1. A package matching a declared pattern is marked `private`; a package not
   matching is not.
2. Nothing is marked private when nothing is declared, except a package
   installed from outside any index
   ([REQ-PY-015](../py/REQ-PY-015-installed-packages-no-index-has.md)).

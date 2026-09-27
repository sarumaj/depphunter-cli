---
id: REQ-EXP-013
title: Floating flag in JSON and GraphML exports
scope: exp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The JSON and GraphML exports **shall** carry the `floating` flag of an external
package that is not pinned to one version.

## Rationale

Downstream tools can then report unpinned dependencies without re-deriving the
ecosystem rules.

## Acceptance criteria

1. A floating package appears with `"floating":true` in JSON and with `<data
   key="floating">true</data>` in GraphML.

## Notes

What counts as floating is specified in scope `sup` (REQ-SUP).

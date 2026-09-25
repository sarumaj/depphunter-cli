---
id: REQ-SUP-006
uuid: 8234dd2e-d12d-477f-a881-21673a332c13
title: Floating flag in JSON and GraphML
scope: sup
type: interface
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M12
verification:
  - unit
---

## Statement

The JSON and GraphML exports **shall** carry `floating` for every floating
package, and `requested` where a lock file resolved a range.

## Rationale

An export is how the supply-chain picture reaches other tools; the pin status
must survive it.

## Acceptance criteria

1. The JSON export of a floating package has `"floating": true`.
2. The GraphML export declares the `floating` and `requested` keys and sets
   `floating` to `true` on a floating package node.

## Notes

The JSON field is defined by the graph model (scope [`mod`](../mod/)). No
automated test checks the flag in either export.

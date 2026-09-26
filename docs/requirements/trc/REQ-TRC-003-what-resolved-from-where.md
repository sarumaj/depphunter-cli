---
id: REQ-TRC-003
uuid: 4f8aeaca-46c0-4864-b9d4-5dd3996dc2a8
title: What resolved from which index
scope: trc
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The report **shall** state, per ecosystem and index, how many packages on the
map resolve from it and how many of them are private, counting packages
installed from outside any index as a row of their own, and the totals of
packages, transitive, private and untrusted-index packages.

## Rationale

The index a package resolves from is invisible in the drawing.

## Acceptance criteria

1. A run with no `--resolve-depth` still reports which index every package
   resolves from.

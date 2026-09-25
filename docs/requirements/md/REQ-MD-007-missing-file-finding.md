---
id: REQ-MD-007
uuid: 74da1e51-daeb-42a8-b65c-a6a0e1244ab4
title: Missing link target finding
scope: md
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** report a link to a repository path that does not exist on
disk as a `link/missing-file` finding of medium severity.

## Rationale

A file moved or deleted without updating the documents that point at it is the
most common kind of link rot.

## Acceptance criteria

1. A README linking to a moved file reports it.
2. `[one that is not](docs/GONE.md)` on line 3 yields `link/missing-file` on
   line 3.

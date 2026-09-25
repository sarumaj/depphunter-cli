---
id: REQ-SUP-018
uuid: 5f10bdde-22a9-4efa-b2b7-d15b7600a337
title: A repository-only index is marked
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
  - manual
---

## Statement

A package whose index only the repository names, and which nothing on this
machine vouches for, **shall** carry `indexUnknown` on the graph, and the side
panel **shall** show an "⚠ index" warning badge for it.

## Rationale

A repository pointing at an index nobody on this machine configured is the shape
a dependency-confusion attack takes.

## Acceptance criteria

1. A repository whose `.npmrc` names an index this machine does not know has
   every npm package marked.
2. The same package resolved from the public default or from a
   machine-configured index is not marked.

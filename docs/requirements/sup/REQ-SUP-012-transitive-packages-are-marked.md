---
id: REQ-SUP-012
uuid: 53f4773f-822f-426d-86d6-d2ce2d7ad779
title: Transitive packages are marked
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
  - manual
---

## Statement

A package that the transitive walk adds, and that no file in the project
imports, **shall** carry `transitive` on the graph.

## Rationale

A package on the map because another package needs it answers a different
question from one the code imports.

## Acceptance criteria

1. After `--resolve-depth 1` a package only a dependency needs is marked
   `transitive`.
2. A package the project imports directly is never marked `transitive`, whatever
   the depth.

## Notes

The side panel shows a "transitive" badge for such a package.

---
id: REQ-MD-005
uuid: 0941af27-14b7-415c-a8a9-64e634b58626
title: No island for external hosts
scope: md
type: constraint
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M19
verification:
  - unit
---

## Statement

The Markdown plugin **shall** declare no ecosystem, and an external URL
**shall not** become a node or an edge on the map.

## Rationale

A URL is not a dependency the map can characterize, and a legend of third-party
domain names would add nothing.

## Acceptance criteria

1. The Markdown plugin declares no ecosystem.
2. A link to `https://…` resolves to nothing.

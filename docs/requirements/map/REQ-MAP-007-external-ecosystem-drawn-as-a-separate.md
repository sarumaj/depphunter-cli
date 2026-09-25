---
id: REQ-MAP-007
uuid: fcead37f-922a-405c-942b-16b68309661b
title: External ecosystem drawn as a separate island
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §2
verification:
  - ui
  - manual
---

## Statement

The system **shall** draw each visible external ecosystem (Go modules, npm, PyPI
and the others) as a separate island of its own, off the mainland, carrying that
ecosystem's packages.

## Rationale

What the project draws in from outside is kept visibly apart from the project
itself, one island per ecosystem.

## Acceptance criteria

1. A repository importing npm and PyPI packages shows two islands, one per
   ecosystem, none overlapping the mainland.
2. Every package building stands on the island of its own ecosystem.

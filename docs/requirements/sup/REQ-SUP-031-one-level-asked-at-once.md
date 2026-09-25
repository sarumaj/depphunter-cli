---
id: REQ-SUP-031
uuid: a7310290-581a-4054-acbb-c8ef0259d207
title: One level asked at once
scope: sup
type: non-functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M12
verification:
  - unit
---

## Statement

The walker **shall** ask about all packages of one level concurrently, with at
most 12 questions in flight, and **shall** apply the answers in the level's own
order.

## Rationale

Asking one package at a time made `--online` walks take as long as the requests
laid end to end; a bound spares somebody else's registry, and a fixed order
keeps the graph independent of response order.

## Acceptance criteria

1. Two runs over the same repository produce the same graph.
2. Cancelling the analysis stops the walk without asking the rest of the level.

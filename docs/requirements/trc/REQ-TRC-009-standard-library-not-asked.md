---
id: REQ-TRC-009
uuid: 27ca5a62-5dea-4619-9e35-1612975244c0
title: A standard library is not asked about
scope: trc
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M18
verification:
  - unit
---

## Statement

The walker **shall not** ask about a package of a standard-library ecosystem.

## Rationale

Nothing publishes what `fs` or `os` depends on; asking produces a round of
questions nobody can answer and a report full of them.

## Acceptance criteria

1. A run over a standard-library import records no question about it.

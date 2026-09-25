---
id: REQ-SUP-010
uuid: cf573347-f10b-48e3-9fe3-cafba0413b11
title: The offline walk fetches nothing
scope: sup
type: constraint
priority: must
status: implemented
verification:
  - unit
  - inspection
---

## Statement

Without `--online` the walker **shall not** make any network request; it
**shall** answer only from the files the repository carries.

## Rationale

depphunter operates offline by default; asking an index discloses what the
project uses.

## Acceptance criteria

1. A run with `--resolve-depth -1` and without `--online` builds no index client
   and records every question the lock files cannot answer as unanswered for
   being offline.

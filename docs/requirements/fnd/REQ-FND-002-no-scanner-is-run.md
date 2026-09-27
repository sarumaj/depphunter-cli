---
id: REQ-FND-002
title: No scanner is run
scope: fnd
type: constraint
priority: must
status: implemented
verification:
  - inspection
---

## Statement

The system **shall not** run any scanner; it **shall** only read reports that
other tools have written.

## Rationale

Scanners are slow, need their own installation and are already run in CI; the
map puts their results in place.

## Acceptance criteria

1. No code path of `internal/findings` starts an external process.

---
id: REQ-JS-007
uuid: 53f1fa49-08bc-439e-aa19-37184e97a988
title: package-lock.json versions
scope: js
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** replace a declared range by the version the nearest
enclosing `package-lock.json` (formats v1 to v3) records for the package's
top-level installation, keeping the range as the requested specifier, and
**shall** treat that version as pinned.

## Rationale

The manifest states what was asked for, the lock file what is installed; both
are shown.

## Acceptance criteria

1. `react` declared `^18.2.0` and locked at `18.3.1` resolves to version
   `18.3.1`, requested `^18.2.0`, pinned.

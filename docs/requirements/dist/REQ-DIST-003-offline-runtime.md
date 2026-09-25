---
id: REQ-DIST-003
uuid: c96c358e-3f39-49e3-a142-0c7f527b8a04
title: No Node.js or network at runtime
scope: dist
type: non-functional
priority: must
status: implemented
verification:
  - inspection
  - e2e
---

## Statement

The system **shall** run without Node.js and **shall not** access the network at
runtime unless the user opts in (for example with `--online`).

## Rationale

The tool must work offline, and a repository's contents must not leave the
machine unless the user asks for it.

## Acceptance criteria

1. Building the binary needs only Go.
2. A run without `--online` on a machine without network access analyzes and
   serves the map.

## Notes

Opt-in network access (package indexes, the vulnerability database, http link
checks) belongs to scopes `sup`, `fnd` and `md`.

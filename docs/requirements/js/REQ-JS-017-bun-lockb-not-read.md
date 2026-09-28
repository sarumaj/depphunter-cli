---
id: REQ-JS-017
title: Binary bun.lockb not read
scope: js
type: limitation
priority: should
status: implemented
verification:
  - inspection
---

## Statement

The plugin **shall not** read `bun.lockb`, the binary lock file of Bun before
1.2: a project that has only it gets the ranges its `package.json` files
declare, unless a `yarn.lock` beside it (written by `bun install --yarn` or
bunfig's `print = "yarn"`) is read as any `yarn.lock`. A `bun.lockb` with
neither a `bun.lock` nor a `yarn.lock` beside it **shall** be noted in the
resolution report ([REQ-TRC-017](../trc/REQ-TRC-017-resolver-notes.md)).

## Rationale

The binary layout is Bun's in-memory one and undocumented; `bun install
--save-text-lockfile` converts it to `bun.lock`.

---
id: REQ-ELM-011
title: Read without running elm
scope: elm
type: limitation
priority: should
status: implemented
verification:
  - unit
---

## Statement

Elm **shall** be read without running elm or downloading packages: a module of
a package that is not installed in `ELM_HOME`, not in the curated table and
not spelled by a listed package's name is dropped, since an Elm module name
does not say which author published it.

## Rationale

Which package exposes a module is recorded only in that package's
`elm.json`; asking the package site per module would need the network.

## Acceptance criteria

1. `Mystery.Thing` is dropped.

---
id: REQ-GLEAM-008
title: Hex pinning rule
scope: gleam
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

Gleam packages **shall** be pinned by the BEAM plugin's Hex rule:
`manifest.toml` pins a Hex package at its version (a range requirement kept
as the requested version) and a git package at its commit (the ref asked
for kept as requested); without a manifest `== 1.2.3` and a bare `1.2.3`
pin, `~>`, `>=` and `and`/`or` ranges are kept as written, a git ref that is
a full commit pins, a branch or tag floats, and a path dependency is an
edge.

## Rationale

One package on one island must follow one rule, whichever language reached
it.

## Acceptance criteria

1. `gleam_stdlib = "== 0.40.0"` and `birl = "1.7.1"` without a manifest
   are pinned; `argv = "~> 1.0"` is not.
2. A git dependency with a 40-hex `ref` is pinned; one with `ref = "main"`
   floats.

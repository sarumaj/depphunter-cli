---
id: REQ-BEAM-011
uuid: 7c0f48a7-9c70-4b56-995e-15ff99123aad
title: Hex pinning rule
scope: beam
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** treat a Hex package as pinned when a lock pins it (a
git package by its commit), when its requirement is `== 1.2.3` or a bare
`1.2.3` (shown bare), or when it is a git dependency whose `ref` is a commit;
`~>`, `>=`, `or`/`and` combinations, branches and tags float.

## Rationale

In Hex, a bare version requirement matches that version only.

## Acceptance criteria

1. `{:jason, "== 1.4.1"}` is pinned at 1.4.1; `{:recon, {git, Url, {branch,
   "master"}}}` floats.

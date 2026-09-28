---
id: REQ-CUE-001
title: Files claimed
scope: cue
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The CUE plugin **shall** claim CUE files (`.cue`), `cue.mod/module.cue`
told apart by its path. Nothing in `cue.mod/pkg`, `cue.mod/gen` or
`cue.mod/usr` (vendored modules, what `cue get go` generated and what
augments it) **shall** be claimed, and a walk of the file system **shall**
not enter a `pkg`, `gen` or `usr` directory beside a `module.cue`.

## Rationale

Those trees hold dependencies' definitions; they are read only to
resolve imports (REQ-CUE-006).

## Acceptance criteria

1. The fixture's CUE files and module file are claimed; its files under
   `cue.mod/gen`, `cue.mod/pkg` and `cue.mod/usr` are not.

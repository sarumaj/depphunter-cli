---
id: REQ-CUE-008
title: Islands
scope: cue
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

CUE modules **shall** form the "CUE modules" island (`cue:` as a
private-pattern prefix) and the builtin packages the hidden "CUE standard
library"; the Go islands **shall** be the Go plugin's. No vulnerability
database **shall** be asked about CUE modules; with `--online` a CUE registry
is asked for a module's dependencies (REQ-SUP-073).

## Rationale

OSV has no CUE ecosystem.

## Acceptance criteria

1. The plugin declares `cue`, `cue-std` (standard), `go` and `go-std`
   (standard).

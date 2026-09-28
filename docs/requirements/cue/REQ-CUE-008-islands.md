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
database and no registry **shall** be asked about CUE modules.

## Rationale

OSV has no CUE ecosystem; the central registry (registry.cue.works) is
an OCI registry this tool does not read.

## Acceptance criteria

1. The plugin declares `cue`, `cue-std` (standard), `go` and `go-std`
   (standard).

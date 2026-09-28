---
id: REQ-CUE-007
title: Standard library and unresolved imports
scope: cue
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

An import of one of CUE's builtin packages (`strings`, `list`,
`encoding/json`, `tool/exec`, ...) **shall** resolve to the hidden
`cue-std` island before anything else. Any other import **shall** be an
unresolved `cue` package: a path without a dot in its first element by
its path, else by its repository (host/owner/repo on GitHub, GitLab,
Bitbucket, Codeberg, sourcehut and cue.dev, else host/first element).

## Rationale

Builtin packages take precedence in CUE.

## Acceptance criteria

1. `strings`, `encoding/json`, `list` and `tool/exec` are `cue-std`;
   `github.com/nobody/thing/pkg` and `unknownstd` are unresolved.

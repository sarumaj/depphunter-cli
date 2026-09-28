---
id: REQ-CUE-005
title: Module dependencies
scope: cue
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

`cue.mod/module.cue` **shall** be read (`module`, and `deps` in struct
or label-chain form); each dependency **shall** be an import of the module
file and an import under its path **shall** resolve to it: a package of
the `cue` island named by its module path without the major version,
pinned by its `v` (the modules system selects exact versions, as Go's
does); a dependency without `v` floats.

## Rationale

The CUE modules system records exact versions in module.cue.

## Acceptance criteria

1. `github.com/acme/schemas/k8s` is `github.com/acme/schemas` pinned at
   v0.3.1, a label-chain dependency is read, and one without `v`
   floats.

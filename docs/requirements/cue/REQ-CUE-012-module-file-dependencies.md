---
id: REQ-CUE-012
title: A module file's dependencies
scope: cue
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read the `deps` of a module.cue served by a registry as it
reads a repository's (REQ-CUE-005): each module path without its major version,
pinned by the version it names.

## Rationale

The index client (REQ-SUP-073) reads module files from CUE registries.

## Acceptance criteria

1. See REQ-SUP-073.

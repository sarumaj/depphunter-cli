---
id: REQ-CUE-010
title: Read without the cue command
scope: cue
type: limitation
priority: should
status: implemented
verification:
  - unit
---

## Statement

CUE projects **shall** be read without running `cue`: a package's files
in parent directories (which CUE adds to a package instance) are not
linked, attributes and `@if` build tags are not evaluated (every file
counts), dependencies' own dependencies are not read (no module cache),
and nothing is asked of the central registry (no `--online`).

## Rationale

The module cache lives outside the repository, and the registry speaks
OCI.

## Acceptance criteria

1. A package's files are linked from its own directory only, and module
   dependencies have no dependencies of their own.

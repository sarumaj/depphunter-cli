---
id: REQ-PY-005
uuid: 7c69900d-b442-4774-8f70-49d1bac10b03
title: Python standard library
scope: py
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M2
verification:
  - unit
---

## Statement

The plugin **shall** resolve an import whose top-level module is in the built-in
list of standard-library modules to a package of that top-level name in the
ecosystem `python-std` ("Python standard library"), declared as a standard
library.

## Rationale

Standard-library modules are not distributions and belong on their own island.

## Acceptance criteria

1. `os`, `os.path` and `json` resolve to `python-std` packages `os`, `os` and
   `json`.

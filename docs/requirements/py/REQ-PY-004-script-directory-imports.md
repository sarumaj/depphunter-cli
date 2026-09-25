---
id: REQ-PY-004
uuid: 382cad61-6d22-4110-a839-6ff35e472a67
title: Script directory imports
scope: py
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** resolve an absolute import that matches no project root and
no standard-library module against the importing file's own directory.

## Rationale

A script's own directory is on `sys.path` when it runs.

## Acceptance criteria

1. `import sibling` in `scripts/tool.py` resolves to `scripts/sibling.py`.

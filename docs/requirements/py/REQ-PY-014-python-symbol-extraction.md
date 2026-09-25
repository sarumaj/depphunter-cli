---
id: REQ-PY-014
uuid: 8e8d2fba-e0ce-48b9-b611-ae90ff3e150b
title: Python symbol extraction
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

The plugin **shall** extract module-level functions (`func`), classes (`class`)
and assignments to a name (`var`), including decorated definitions, and the
methods of a class (`method`, named `<Class>.<method>`).

## Rationale

These are the definitions a reader navigates by in Python.

## Acceptance criteria

1. `src/app/main.py` yields `run` and `dec` (func), `Service` (class),
   `Service.start` and `Service.name` (method) and `CONSTANT` (var).

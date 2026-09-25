---
id: REQ-PY-012
uuid: 1eb5ca2e-f311-4cda-92e1-c6f810102565
title: setup.py literal lists
scope: py
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M6
verification:
  - unit
---

## Statement

The plugin **shall** read the string literals of a literal `install_requires =
[...]` list and of the lists in a literal `extras_require = {...}` dict in
`setup.py`, without executing the file.

## Rationale

Executing `setup.py` would run project code; literal lists cover the common
case.

## Acceptance criteria

1. `rich==13.7.0` in `install_requires` is declared with version `13.7.0`,
   pinned.
2. `sphinx>=7` in an `extras_require` list is declared; the dict's keys are not.

## Notes

Requirements computed at run time are not seen.

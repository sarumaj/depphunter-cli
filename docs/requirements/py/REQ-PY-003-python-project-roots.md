---
id: REQ-PY-003
uuid: 8c62b367-259d-435b-a516-bfa6e928c81e
title: Python project roots
scope: py
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** resolve an absolute import against the project's import
roots - the repository root, every directory containing a `pyproject.toml`,
`setup.py` or `setup.cfg`, and the `src/` directory of each of these where it
holds Python files - trying the deepest root first and the longest matching
module prefix within a root.

## Rationale

`src/` layouts and nested sub-projects are common; the deepest root shadows
same-named top-level modules as it does at run time.

## Acceptance criteria

1. `from app.helpers import thing` with the package under `src/app` resolves to
   `src/app/helpers.py`.

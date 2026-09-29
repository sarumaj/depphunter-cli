---
id: REQ-PY-003
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
holds Python files, plus the roots `PYTHONPATH` and the tools' settings add
([REQ-PY-016](REQ-PY-016-configured-import-roots.md)) - trying a project's own
roots deepest first, then the configured roots, then the repository root, and
the longest matching module prefix within a root.

## Rationale

`src/` layouts and nested sub-projects are common; the deepest root shadows
same-named top-level modules as it does at run time.

## Acceptance criteria

1. `from app.helpers import thing` with the package under `src/app` resolves to
   `src/app/helpers.py`.
2. A module in a sub-project's directory shadows one of the same name under a
   configured root, which shadows one in the repository root.

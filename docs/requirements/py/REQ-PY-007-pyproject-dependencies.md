---
id: REQ-PY-007
uuid: 48bfb92c-a6eb-41d7-a723-e4662ab6a612
title: pyproject.toml dependencies
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

The plugin **shall** read distributions from `pyproject.toml`: PEP 621
`project.dependencies` and `project.optional-dependencies`, PEP 735
`dependency-groups` (string entries), and Poetry's `tool.poetry.dependencies`,
`tool.poetry.dev-dependencies` and `tool.poetry.group.*.dependencies`.

## Rationale

These are the manifests modern Python projects declare their dependencies in.

## Acceptance criteria

1. A PEP 621 dependency `requests>=2.31` is declared with specifier `>=2.31`.
2. A Poetry dependency `beautifulsoup4 = "^4.12"` is declared with version
   `^4.12`.

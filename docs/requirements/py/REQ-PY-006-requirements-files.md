---
id: REQ-PY-006
uuid: 460c407c-bb46-4bac-9c76-643c1266d74a
title: Requirements files
scope: py
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read distributions and their specifiers from requirements
files - files named `requirements*.txt` and `.txt` files in a directory named
`requirements` - as PEP 508 lines, ignoring comments, options lines starting
with `-`, extras and environment markers.

## Rationale

Requirements files remain the most common way to declare Python dependencies.

## Acceptance criteria

1. A `requirements-dev.txt` declaring `pytest>=8` makes `import pytest` resolve
   to `pytest` version `>=8`.

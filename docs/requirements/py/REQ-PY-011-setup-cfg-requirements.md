---
id: REQ-PY-011
uuid: 0bab7c97-0202-41d6-ae97-e687bfa594b5
title: setup.cfg requirements
scope: py
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read `install_requires` in section `[options]` and every
key of section `[options.extras_require]` of a `setup.cfg`, including indented
continuation lines, and ignoring comments.

## Rationale

Setuptools projects commonly declare dependencies declaratively in `setup.cfg`.

## Acceptance criteria

1. `requests>=2.31` with a trailing comment is declared with specifier `>=2.31`.
2. An extras entry `pytest>=8` is declared.

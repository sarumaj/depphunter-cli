---
id: REQ-PY-008
uuid: 8bcff2ff-6e34-46e1-8840-34597d5cb867
title: Pipfile dependencies
scope: py
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read distributions from the `packages` and `dev-packages`
tables of a `Pipfile`, as a version string or a table with a `version` key.

## Rationale

Pipenv projects declare dependencies only in the Pipfile.

## Acceptance criteria

1. A `Pipfile` entry `requests = "*"` makes `import requests` resolve to the
   declared distribution `requests`.

## Notes

`TestPipfile` covers both sections and the table form.

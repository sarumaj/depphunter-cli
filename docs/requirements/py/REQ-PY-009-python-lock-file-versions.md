---
id: REQ-PY-009
uuid: 6afa1755-87a4-466d-998d-925af491b9ce
title: Python lock file versions
scope: py
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read locked versions from `poetry.lock`, `uv.lock`,
`pdm.lock` and `Pipfile.lock` (`default` and `develop`) after all manifests,
**shall** treat a locked version as pinned, and **shall** keep the manifest's
differing specifier as the requested one. Distributions found only in a lock
file **shall** be resolvable as well.

## Rationale

Lock files state what is installed; transitive distributions are installed too,
so importing them resolves.

## Acceptance criteria

1. `requests>=2.31` locked by `uv.lock` at `2.31.0` resolves to version
   `2.31.0`, requested `>=2.31`, pinned.
2. `certifi`, present only in the lock file, resolves to its locked version,
   pinned.

## Notes

Only `uv.lock` is covered by an automated test.

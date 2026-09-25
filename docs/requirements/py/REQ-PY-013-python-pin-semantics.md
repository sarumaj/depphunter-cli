---
id: REQ-PY-013
uuid: 588faa79-1a73-4480-a5fc-bffc287074c0
title: Python pin semantics
scope: py
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M12
verification:
  - unit
---

## Statement

The plugin **shall** treat a Python dependency as pinned when a lock file
records its version or its specifier is `==<version>`, and as floating when its
specifier is a range; a range read from a manifest **shall not** loosen a
version a lock file already fixed.

## Rationale

Only an exact version or a lock file fixes what `pip` installs.

## Acceptance criteria

1. `black==24.1.0` is pinned; `pytest>=8` is floating.
2. A distribution locked by `uv.lock` stays pinned whatever range a manifest
   gives.

## Notes

The general floating rule is scope `sup`.

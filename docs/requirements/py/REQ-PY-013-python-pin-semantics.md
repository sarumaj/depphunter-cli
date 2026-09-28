---
id: REQ-PY-013
title: Python pin semantics
scope: py
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** treat a Python dependency as pinned when a lock file
records its version, its specifier is `==<version>` or it is a direct
reference to a full git commit (`name @ git+<url>@<sha>`, which also records
that checkout for REQ-FND-026), and as floating when its specifier is a range
or a direct reference to a branch or tag; a range read from a manifest
**shall not** loosen a version a lock file already fixed.

## Rationale

Only an exact version or a lock file fixes what `pip` installs.

## Acceptance criteria

1. `black==24.1.0` is pinned; `pytest>=8` is floating.
2. A distribution locked by `uv.lock` stays pinned whatever range a manifest
   gives.
3. `foo @ git+https://github.com/o/foo@<40 hex digits>` is pinned and carries
   `https://github.com/o/foo#<sha>`; `bar @ git+https://github.com/o/bar@v1.0`
   floats.

## Notes

The general floating rule is scope `sup`.

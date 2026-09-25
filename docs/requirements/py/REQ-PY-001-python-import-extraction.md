---
id: REQ-PY-001
uuid: 7fc69519-d07b-4f11-88fb-451605efaad8
title: Python import extraction
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

The Python plugin **shall** claim non-binary `.py` and `.pyi` files and extract
`import a.b` statements (one import per module, also when aliased), `from m
import n` statements (one import per imported name), wildcard imports and `from
__future__` imports.

## Rationale

A `from m import n` may import a sub-module `n`, so each name is resolved on its
own.

## Acceptance criteria

1. `from app.models import *` is one import of `app.models`.
2. `from . import utils` and `from .models import User` are separate imports.
3. `from __future__ import annotations` resolves to the standard-library package
   `__future__`.

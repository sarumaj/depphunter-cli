---
id: REQ-PY-002
uuid: 1e9b1ae3-ef9a-4868-9290-c670eeaeea4f
title: Relative import resolution
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

The plugin **shall** resolve a relative import against the importing file's
package, climbing one directory per leading dot beyond the first, to a module
file (`.py` or `.pyi`) or a package directory, trying the imported name as a
sub-module first; an import climbing out of the project **shall** be dropped.

## Rationale

Relative imports are how packages refer to their own modules.

## Acceptance criteria

1. `from . import utils` in `src/app/main.py` resolves to `src/app/utils.py`.
2. `from .models import User` resolves to `src/app/models`; `from .models.user
   import User` to `src/app/models/user.py`.
3. `from ..outside import x` leaving the project yields no target.

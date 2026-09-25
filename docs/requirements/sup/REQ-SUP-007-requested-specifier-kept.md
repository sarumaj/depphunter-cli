---
id: REQ-SUP-007
uuid: a56b312b-b239-426a-bdf7-2ca39d1fdc09
title: Requested specifier kept beside the version
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
  - manual
---

## Statement

Where a lock file resolved a range, the system **shall** keep both the resolved
version and the specifier the manifest requested, and the side panel **shall**
show both.

## Rationale

The requested range says what the project accepts; the locked version says what
it gets. Either alone hides half of the answer.

## Acceptance criteria

1. A package declared as `^4.2.0` and locked to 4.3.1 carries version `4.3.1`
   and requested `^4.2.0`.
2. The side panel of that package shows both values.

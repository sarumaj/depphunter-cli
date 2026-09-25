---
id: REQ-SUP-034
uuid: 64b07075-3f3f-489c-8f4a-4669d03b4ae9
title: Private package patterns
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** accept `--private` (repeatable), `private:` in the
configuration and `DEPPHUNTER_PRIVATE` as comma-separated glob patterns with
GOPRIVATE's glob-prefix meaning, and **shall** treat a package they match as the
organization's own.

## Rationale

Whether a package is internal cannot be inferred; anyone setting this has met
the GOPRIVATE rule before.

## Acceptance criteria

1. `corp.example/*` matches `corp.example/team/lib` and not
   `corp.example.org/lib`.
2. One entry may hold several comma-separated patterns.
3. With nothing declared nothing matches.

---
id: REQ-MD-010
title: Offline link checks run on every analysis
scope: md
type: functional
priority: must
status: implemented
verification:
  - unit
  - integration
---

## Statement

The system **shall** run the missing-file, missing-fragment and
undefined-reference checks on every analysis without being asked for them, and
without network access.

## Rationale

The three checks need nothing but the repository and are exact, so there is no
reason to make them opt-in.

## Acceptance criteria

1. A plain run with no flags enables findings and runs the link check.
2. A document that cannot be read marks the set of findings incomplete, and the
   others are still checked.

---
id: REQ-EXT-006
uuid: 5ff574df-8891-4b13-9c37-a57180689ae4
title: Tree marks unpinned and unvouched packages
scope: ext
type: functional
priority: should
status: implemented
source:
  - README.md The panel beside the code
verification:
  - extension
  - manual
---

## Statement

The Dependencies view **should** mark, in the row itself rather than only in its
tooltip, a package that is not pinned to one version and a package that resolves
from an index this machine does not configure.

## Rationale

Those are the packages worth picking out of a list of a hundred.

## Acceptance criteria

1. A floating package row carries `floating` in its description and a
   warning-colored icon.
2. A package whose index is unknown to this machine, or which no manifest
   declares, carries a warning icon.

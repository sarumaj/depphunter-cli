---
id: REQ-EXP-015
uuid: 6b6a716d-67ac-4e23-a478-9f2c2f7fd48b
title: Backpack export offered by the page
scope: exp
type: functional
priority: must
status: not-implemented
source:
  - docs/REQUIREMENTS.md M15
verification:
  - manual
---

## Statement

The page **shall** offer the backpack export in Markdown, CSV and JSON.

## Rationale

M15 requires the catch to be written out from the page as well as from the
editor.

## Acceptance criteria

1. The backpack panel of the page offers downloads in the three formats.

## Notes

Only the editor extension offers it (scope `ext`); the page backpack panel has
no export action, although the server endpoint of REQ-EXP-014 exists.

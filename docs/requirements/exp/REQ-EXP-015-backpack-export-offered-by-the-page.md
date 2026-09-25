---
id: REQ-EXP-015
uuid: 6b6a716d-67ac-4e23-a478-9f2c2f7fd48b
title: Backpack export offered by the page
scope: exp
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

The page **shall** offer the backpack export in Markdown, CSV and JSON.

## Rationale

The catch can be written out from the page as well as from the editor
(REQ-EXT-013).

## Acceptance criteria

1. The backpack panel of the page offers downloads in the three formats.

## Notes

Fixed: the backpack panel has an Export footer with Markdown, CSV and JSON links
to `GET /api/backpack?format=md|csv|json` (REQ-EXP-014). The page hands up the
backpack on every change, so the server's copy is the page's. The footer is
hidden while the backpack is empty and in the static HTML export, which has no
server (`data-server`, as for the export menu's server links).

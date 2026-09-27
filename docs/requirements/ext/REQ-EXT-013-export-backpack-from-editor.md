---
id: REQ-EXT-013
title: Backpack export from the editor
scope: ext
type: functional
priority: must
status: implemented
source:
  - README.md Use
verification:
  - extension
---

## Statement

The command **depphunter: Export the Backpack** **shall** write the collected
findings of the attached server to a file chosen by the user, as Markdown, CSV
or JSON.

## Rationale

The catch can be written out from the editor as well as from the page.

## Acceptance criteria

1. The command offers exactly Markdown, CSV and JSON and a save dialog
   defaulting to `<folder>-backpack.<ext>` in the folder.
2. The saved file equals `GET /api/backpack?format=<md|csv|json>`.
3. Without an attached server the command says there is nothing to export.

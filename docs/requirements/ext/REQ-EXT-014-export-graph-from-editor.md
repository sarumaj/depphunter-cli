---
id: REQ-EXT-014
title: Graph export from the editor
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

The command **depphunter: Export the Graph** **shall** write the graph of the
attached server to a file chosen by the user, as JSON, GraphML, DOT or a
self-contained HTML map.

## Rationale

The graph can be written out in its own formats from the editor as well as from
the page.

## Acceptance criteria

1. The command offers exactly JSON, GraphML, DOT and HTML and a save dialog
   defaulting to `<folder>.<ext>` in the folder.
2. The saved file equals `GET /api/export?format=<format>`.

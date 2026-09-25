---
id: REQ-EXP-005
uuid: a371d2e0-0845-4097-a8f9-844b86cd2238
title: Export menu in the UI
scope: exp
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M3
  - docs/REQUIREMENTS.md M4
  - docs/REQUIREMENTS.md M15
verification:
  - integration
  - manual
---

## Statement

The UI **shall** offer an Export menu with JSON, GraphML, DOT and HTML
downloads, served by `GET /api/export?format=<format>` as attachments named
after the project.

## Rationale

The graph can be written out in its own formats from the page without restarting
the tool.

## Acceptance criteria

1. `/api/export?format=json|graphml|dot|html` returns 200 with the respective
   content.
2. `/api/export?format=svg` returns 400.

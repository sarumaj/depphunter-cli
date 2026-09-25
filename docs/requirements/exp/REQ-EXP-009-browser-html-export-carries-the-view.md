---
id: REQ-EXP-009
uuid: 39164a09-5db0-430d-a34a-04e303de4f81
title: Browser HTML export carries the view on screen
scope: exp
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M10
verification:
  - integration
  - manual
---

## Statement

An HTML export requested from the browser **shall** carry the view settings on
screen in a `ui` query parameter, which the server **shall** validate and use;
an invalid value **shall** be answered 400.

## Rationale

The exported page should open looking like the map on screen, not like the
configuration file.

## Acceptance criteria

1. Exporting HTML after switching the theme produces a page that opens in that
   theme.
2. `/api/export?format=html&ui=<invalid>` returns 400 `invalid ui settings`.

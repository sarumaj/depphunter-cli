---
id: REQ-UI-003
uuid: fa32fd16-d10b-4b26-9212-c89e2b4198cf
title: Clicking the stopped indicator retries
scope: ui
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M10
verification:
  - manual
  - e2e
---

## Statement

When the UI has stopped reconnecting, clicking the status indicator **shall**
reset the attempt count and connect again.

## Rationale

A restarted server can be picked up without reloading the page.

## Acceptance criteria

1. Restarting depphunter on the same address and clicking "depphunter stopped"
   reconnects the page.

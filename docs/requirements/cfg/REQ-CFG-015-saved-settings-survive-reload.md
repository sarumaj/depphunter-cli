---
id: REQ-CFG-015
uuid: 7000f135-cbec-4f61-93a4-14d35d9b2d53
title: Saved settings survive a reload
scope: cfg
type: functional
priority: must
status: implemented
verification:
  - integration
  - e2e
---

## Statement

After a successful save, the server **shall** serve the saved view settings from
`/api/config`, so that a reload of the page, and the next run of the command,
start from them.

## Rationale

Save is only worth having if what it saved is what the next load shows.

## Acceptance criteria

1. After a save with height scale `log`, `GET /api/config` answers
   `"heightScale":"log"`.
2. A reload of the page opens with the saved settings.
3. A new run in the same directory opens with the saved settings unless the
   environment or a flag overrides them.

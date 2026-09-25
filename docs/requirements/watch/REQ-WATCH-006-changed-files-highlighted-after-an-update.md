---
id: REQ-WATCH-006
uuid: c3d0980b-b8f4-4749-8e3d-f27a0c1e03db
title: Changed files highlighted after an update
scope: watch
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M3
verification:
  - manual
---

## Statement

After a live update the UI **shall** highlight the changed files for 3 s and
report in the status bar the time of the update and the number of changed files.

## Rationale

The user should see at a glance what the last change touched.

## Acceptance criteria

1. After editing a file its building is highlighted and the status bar reads
   `updated <time> · 1 changed (highlighted)`.

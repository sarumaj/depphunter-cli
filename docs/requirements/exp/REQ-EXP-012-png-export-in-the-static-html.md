---
id: REQ-EXP-012
uuid: ad4eee94-a5fd-4f47-aa25-fe915fdeb646
title: PNG export in the static HTML export
scope: exp
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M7
verification:
  - manual
---

## Statement

The PNG export **shall** be available in the static HTML export.

## Rationale

The image is produced in the browser and needs no server.

## Acceptance criteria

1. In a static export opened from `file://`, Export → PNG image and `P` download
   a PNG.

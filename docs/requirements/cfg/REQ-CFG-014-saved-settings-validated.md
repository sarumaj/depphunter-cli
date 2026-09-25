---
id: REQ-CFG-014
uuid: 62586e1a-7e66-4643-a8d5-165fe5a2c057
title: Saved settings validated
scope: cfg
type: functional
priority: must
status: implemented
verification:
  - unit
  - integration
---

## Statement

The server **shall** refuse `POST /api/settings` with status 400 and leave the
file unchanged when a value lies outside its allowed values.

## Rationale

A file that no longer loads would make the next run fail (REQ-CFG-007).

## Acceptance criteria

1. A save with theme `neon` answers 400.
2. A save with style `swamp` answers 400.
3. `SaveUI` with an invalid value returns an error and does not create the file.

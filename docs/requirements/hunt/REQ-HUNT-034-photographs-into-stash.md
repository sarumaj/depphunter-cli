---
id: REQ-HUNT-034
uuid: b243661b-aa92-4c4e-badb-42ddbb0534a0
title: Photographs kept in a captioned stash
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - ui
  - e2e
---

## Statement

A photograph **shall** be kept in a stash rather than written to a file,
captioned with what was in the frame (the bug under the reticle, or the
building).

## Rationale

Writing a file for every press put a download in the browser's bar and gave no
way to look at the pictures.

## Acceptance criteria

1. A photograph can be looked at before it is saved.
2. A photograph of a bug is captioned with its severity and title; one of a
   building with the building's name.
3. The newest photograph is listed first.

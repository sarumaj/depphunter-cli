---
id: REQ-UI-010
uuid: 9bbc18cd-923c-452a-9d06-68e06352ba00
title: Help leads back to the introduction
scope: ui
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M22
verification:
  - manual
  - e2e
---

## Statement

The help **shall** carry a button that closes it and opens the introduction
again - walk mode's while walking, the map's otherwise.

## Rationale

Anyone who skipped the introduction, or wants it again, needs a way back to it.

## Acceptance criteria

1. Show the introduction in the help opens the map's introduction on the map and
   walk mode's in the street.

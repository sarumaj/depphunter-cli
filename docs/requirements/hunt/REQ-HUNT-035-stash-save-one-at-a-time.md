---
id: REQ-HUNT-035
uuid: 7fdcaa91-e1cf-4055-9d61-a041d7231e69
title: Photographs saved one at a time
scope: hunt
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M25
verification:
  - ui
---

## Statement

The stash **shall** let each photograph be saved individually as a file named
after the repository and the photograph's number.

## Rationale

Only the pictures worth having are saved.

## Acceptance criteria

1. Saving photograph 3 of repository `demo` downloads `demo-photo-03.png`.

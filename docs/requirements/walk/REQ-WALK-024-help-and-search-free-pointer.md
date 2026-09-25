---
id: REQ-WALK-024
uuid: 3b8e15f7-20db-45c0-bc06-9160355abec8
title: Help and search free the pointer
scope: walk
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M10
verification:
  - manual
---

## Statement

Opening the help dialog or the search from walk mode **shall** release the
pointer, and closing the help or choosing a search result **shall** capture it
again where the browser allows.

## Rationale

A dialog or an input cannot be used with the pointer locked.

## Acceptance criteria

1. `?` in walk mode shows the help with a visible cursor.
2. Closing the help with its Close button returns to the reticle.
3. Choosing a search result returns to the reticle.

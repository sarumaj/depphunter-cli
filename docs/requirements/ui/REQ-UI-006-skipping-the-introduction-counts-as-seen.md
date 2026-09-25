---
id: REQ-UI-006
uuid: ce1bfb7c-ca97-432e-983a-9abd54543370
title: Skipping the introduction counts as seen
scope: ui
type: functional
priority: must
status: implemented
verification:
  - ui
---

## Statement

The introduction **shall** offer Skip, and closing it by Skip, by `Esc` or by
finishing it **shall** each count as having seen it.

## Rationale

Somebody who skipped it has said they do not want it.

## Acceptance criteria

1. After Skip, a reload does not open the introduction.

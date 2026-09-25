---
id: REQ-TOOL-053
uuid: d961b1bb-1808-4ebd-9730-45f85d69c6f2
title: Tool row laid out by hand
scope: tool
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M29
verification:
  - e2e
  - manual
---

## Statement

The HUD's tool row **shall** show the left hand's tools on the left and the
right hand's tools on the right, each group behind a small hand icon.

## Rationale

A hand is read at a glance; a word between the groups is not.

## Acceptance criteria

1. The row shows a left hand over the carried tools and a right hand over the
   rest (M29 acceptance).

---
id: REQ-TOOL-053
title: Tool row laid out by hand
scope: tool
type: functional
priority: must
status: implemented
verification:
  - e2e
  - manual
---

## Statement

The HUD's tool row **shall** show the left hand's tools on the left and the
right hand's tools on the right, each group behind a small hand icon. The row
**shall** be one line where the window has room for it; where it has not, the
left hand's group **shall** stand over the right hand's, and neither group
**shall** ever be broken over two lines.

## Rationale

A hand is read at a glance; a word between the groups is not.

## Acceptance criteria

1. The row shows a left hand over the carried tools and a right hand over the
   rest.
2. On a wide window the row is one line; narrowed, the carried tools go over the
   hunting ones, each group still in one line.

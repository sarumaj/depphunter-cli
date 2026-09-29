---
id: REQ-TOOL-054
title: Digit keys counted along the row
scope: tool
type: functional
priority: must
status: implemented
verification:
  - ui
---

## Statement

The digit keys **shall** be counted along the tool row from left to right,
skipping a tool with a key of its own: `1` to `3` for the carried tools after
the parachute, `4` upwards for the hunt, and the tenth numbered slot on `0`. The
parachute, the eleventh tool, **shall** have the key left of `1` and the first
slot ([REQ-TOOL-070](REQ-TOOL-070-parachute-thrown-open.md)).

## Rationale

A walker counting along the row to the fourth slot must find a 4.

## Acceptance criteria

1. The tool row reads `` ` `` and then `1` to `0` from left to right, with
   `` ` ``, `1`, `2` and `3` under the left hand.
2. Keys that are not digits pick no tool.

## Notes

Numbering by kind (8, 9 and 0 for the carried tools, drawn at the left-hand
end) made the keys and the layout disagree.

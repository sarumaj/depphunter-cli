---
id: REQ-HIST-010
uuid: 1ab37952-69bc-43bd-991c-23d0e0687bf0
title: History color modes computed in the browser
scope: hist
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M5
  - docs/REQUIREMENTS.md §5
verification:
  - manual
---

## Statement

The UI **shall** offer the color modes Commits, Lines changed, Last change and
Authors, computed in the browser from the raw change lists, and **shall** fall
back to language colors until the history has loaded.

## Rationale

Computing in the browser lets any time range be evaluated without a request.

## Acceptance criteria

1. With history loaded, the four modes appear under "Git history" in the
   color-by control and recolor the map.
2. Selecting a mode issues no request.

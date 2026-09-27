---
id: REQ-HIST-014
title: History figures and top authors in tooltip and panel
scope: hist
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

The tooltip **shall** show a file's or directory's commits since the selected
date, lines changed, last change and number of authors, and the side panel
**shall** show the same figures and the top five authors with their commit
counts.

## Rationale

The colors say where; the figures and the authors say who and how much.

## Acceptance criteria

1. Hovering a committed file in a history mode shows the four figures.
2. Selecting it lists up to five authors, most commits first.

---
id: REQ-HIST-015
uuid: 8276f438-2af3-42f0-bcca-07f676519a6a
title: HTML export embeds the history
scope: hist
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M5
verification:
  - e2e
---

## Statement

The static HTML export **shall** embed the history, when one is available, so
that the exported map keeps the overlay.

## Rationale

Acceptance criterion of M5: the HTML export keeps the overlay.

## Acceptance criteria

1. An HTML export of a git repository, opened from `file://`, offers the four
   history modes and the slider.

---
id: REQ-HIST-015
uuid: 8276f438-2af3-42f0-bcca-07f676519a6a
title: HTML export embeds the history
scope: hist
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

The static HTML export **shall** embed the history, when one is available, so
that the exported map keeps the overlay.

## Rationale

A shared export shows the same overlays as the live map.

## Acceptance criteria

1. An HTML export of a git repository, opened from `file://`, offers the four
   history modes and the slider.

---
id: REQ-WATCH-005
uuid: 174cf7b3-673e-4c40-b754-110e5107fb43
title: View state kept across a live update
scope: watch
type: functional
priority: must
status: implemented
verification:
  - manual
  - e2e
---

## Statement

On a `graph` event the UI **shall** load the new graph while keeping the
expanded directories, the selection, the filters and the language color
assignment of the previous model.

## Rationale

A map that resets on every save would be unusable in watch mode.

## Acceptance criteria

1. After an edit in `--watch` mode, expanded directories stay expanded and the
   selected node stays selected when it still exists.
2. A language keeps its color when another language appears or disappears.

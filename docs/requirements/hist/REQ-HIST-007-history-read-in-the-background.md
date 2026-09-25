---
id: REQ-HIST-007
uuid: d969d8c2-cd39-4f31-9016-09ec418f0dc6
title: History read in the background
scope: hist
type: functional
priority: must
status: implemented
verification:
  - integration
---

## Statement

The server **shall** serve the map before the history is read, read it in the
background, and answer `GET /api/history` with 202 while it is being read, 204
when no history is available (history disabled, not a git work tree, no
commits), and the history document otherwise.

## Rationale

A large history must not delay the map (ripgrep: 2,000+ commits load while the
map is already usable).

## Acceptance criteria

1. Immediately after start `/api/history` answers 202.
2. After the history was set it answers 200 with the document, and 304 to a
   request naming its entity tag.
3. Without history it answers 204.

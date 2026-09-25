---
id: REQ-HIST-008
uuid: f0e90f34-4a8b-49b6-a548-a46de761b63f
title: History availability announced
scope: hist
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M5
verification:
  - integration
---

## Statement

The server **shall** announce a `history` event on the event stream when the
history becomes available or unavailable, and **shall not** announce a re-read
that produced the same history.

## Rationale

The page loads the overlay as soon as it exists without polling.

## Acceptance criteria

1. Setting the history emits one `history` event; the page then loads
   `/api/history`.

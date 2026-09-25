---
id: REQ-WATCH-004
uuid: 73040059-f89d-4906-ba9a-9b2a86a71cdc
title: Updates pushed through Server-Sent Events
scope: watch
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §5
  - docs/REQUIREMENTS.md M3
verification:
  - integration
---

## Statement

The server **shall** push updates to connected browsers as Server-Sent Events on
`GET /api/events`; after a re-analysis that changed the graph fingerprint it
**shall** announce a `graph` event carrying the new version and the sorted list
of changed file paths (added, removed, resized or re-read), and after one that
did not change it **shall** announce nothing.

## Rationale

Decision (M3): SSE instead of WebSocket. Updates flow one way, SSE needs no
dependency and the browser reconnects by itself.

## Acceptance criteria

1. An update adding `b.go` and resizing `a.go` is announced as
   `{"version":2,"changed":["a.go","b.go"]}`.
2. An update with an identical graph returns "unchanged" and announces nothing.
3. The stream sends a comment heartbeat every 25 s and a `retry` of 2000 ms.

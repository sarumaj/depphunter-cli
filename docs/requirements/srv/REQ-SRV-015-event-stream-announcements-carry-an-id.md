---
id: REQ-SRV-015
uuid: 0d785aa6-9631-4615-968d-6370e8d7c0a9
title: Event stream announcements carry an id
scope: srv
type: functional
priority: must
status: implemented
verification:
  - integration
---

## Statement

The server **shall** number every announcement on the event stream with a
sequence that increases by one per announcement, sent as the event `id`.

## Rationale

A reconnecting client hands the last id back (`Last-Event-ID`); without ids a
dropped connection is indistinguishable from a quiet one.

## Acceptance criteria

1. The first announcement after the greeting carries `id: <n>` before its
   `event:` line.
2. The sequence moves with every announcement.

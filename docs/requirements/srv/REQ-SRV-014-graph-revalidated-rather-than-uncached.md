---
id: REQ-SRV-014
uuid: bb96c5ea-c58a-4b7a-a3d9-2145f8230067
title: Graph revalidated rather than uncached
scope: srv
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M17
verification:
  - integration
---

## Statement

The server **shall** send `Cache-Control: no-cache` (not `no-store`) and `Vary:
Accept-Encoding` with `/api/graph`.

## Rationale

A validator is of no use to a client that was told not to keep the document.

## Acceptance criteria

1. The `Cache-Control` header of `/api/graph` is `no-cache`.

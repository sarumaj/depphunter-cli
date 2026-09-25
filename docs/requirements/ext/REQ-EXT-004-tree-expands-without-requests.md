---
id: REQ-EXT-004
uuid: a579b046-cf8d-41b5-9db0-afb0017913bb
title: Tree rows expand without requests
scope: ext
type: non-functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M15
  - README.md The panel beside the code
verification:
  - extension
---

## Statement

Expanding a row of the Dependencies view **shall not** issue a request to the
server; the graph **shall** be fetched once when the panel attaches and again
only when the server announces a changed graph, a non-resumed reconnection, or
the user asks for a refresh.

## Rationale

Every edge arrives with the graph, so a request per row would only add latency
and load.

## Acceptance criteria

1. Expanding any number of rows after the panel has attached makes no further
   HTTP request.
2. A `graph` event on the event stream causes exactly one conditional `GET
   /api/graph`.
3. The Refresh command reads the graph unconditionally.

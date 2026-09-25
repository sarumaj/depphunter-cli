---
id: REQ-SRV-002
uuid: 0e7b075b-02c4-411f-8eda-d63eb4a0df1e
title: Graph document endpoint
scope: srv
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M1
  - docs/REQUIREMENTS.md M17
verification:
  - integration
---

## Statement

The server **shall** answer `GET /api/graph` with the current graph document as
JSON (`root`, `generatedAt`, `nodes`, `edges`), gzip-compressed when the client
accepts it, together with an `X-Graph-Version` header naming the snapshot
version.

## Rationale

The browser draws the whole map from this one document; every other endpoint
adds to it.

## Acceptance criteria

1. `GET /api/graph` returns 200 and a JSON document holding every node and edge
   of the analysis.
2. The served document carries every field of `graph.Graph`
   (TestServedGraphMatchesTheDocument).
3. A request with `Accept-Encoding: gzip` receives `Content-Encoding: gzip`.

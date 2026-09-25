---
id: REQ-SRV-013
uuid: fe27f89a-61e0-4264-a258-e56f223e8c42
title: Graph entity tag from the content fingerprint
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

The server **shall** send an `ETag` with `/api/graph` derived from a fingerprint
of the graph nodes and edges only, and **shall** answer 304 Not Modified to a
request whose `If-None-Match` names the current tag (also as a list, a weak
validator or `*`).

## Rationale

The tag has to be the same for a re-analysis that found the same project;
`generatedAt` would make every snapshot a new entity.

## Acceptance criteria

1. A request with the current tag is answered 304.
2. After an update with an identical graph the old tag still answers 304.
3. After a real change the old tag receives 200.

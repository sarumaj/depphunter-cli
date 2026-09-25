---
id: REQ-FND-022
uuid: e16d0fc6-45d1-4156-8df2-0e449a6064fd
title: Findings served as a background dataset
scope: fnd
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M13
verification:
  - integration
---

## Statement

The server **shall** compute the findings in the background after the map is
served and serve them at `GET /api/findings` with the status codes of the other
background datasets (202 while computing, 204 when there are none), announcing a
`findings` event when they change.

## Rationale

Reading reports and asking OSV must not delay the map.

## Acceptance criteria

1. `/api/findings` answers 202 until the reports are read, then 200 with
   `{findings, sources, partial}`.
2. An unchanged re-read is not announced.

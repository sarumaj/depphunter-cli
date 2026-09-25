---
id: REQ-EXT-030
uuid: d6387299-82ac-420e-af5d-19df25152f94
title: Status bar item while serving
scope: ext
type: functional
priority: should
status: implemented
source:
  - README.md Use
verification:
  - manual
---

## Statement

While at least one server runs, the extension **should** show a status bar item
that opens the map when selected, and **should** hide it otherwise.

## Rationale

It shows that a server is running and is a one-click way back to the map.

## Acceptance criteria

1. The item appears when a server starts and disappears when the last one stops.
2. Its tooltip lists the addresses without the token.

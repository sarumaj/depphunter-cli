---
id: REQ-HUNT-028
uuid: 92111417-1301-4275-86bf-361f6ce15b73
title: Page uploads the backpack as it loads
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

When the map page loads, it **shall** upload its stored backpack to the server,
so that the session starts from what the browser remembers.

## Rationale

The page holds the only store that outlives the server.

## Acceptance criteria

1. After a page load, the server's session backpack equals the browser's stored
   backpack.

---
id: REQ-HUNT-027
uuid: c854c996-439f-4188-937e-788b39b2ac83
title: Backpack kept in the browser per repository
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

The backpack's lasting store **shall** be the browser's local storage, keyed by
repository, so that it outlives the tab and the server.

## Rationale

The server runs only while the map is open; the backpack has to outlive it. Two
maps open side by side do not share one backpack.

## Acceptance criteria

1. A caught finding is still in the backpack after the page is reloaded.
2. Two repositories keep separate backpacks.
3. A blocked or full store leaves the map working, with the backpack kept for
   the session.

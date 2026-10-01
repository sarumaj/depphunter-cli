---
id: REQ-HUNT-028
title: Page uploads the backpack as it loads
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

When the map page loads, it **shall** add to its stored backpack what the
server's session holds besides (what the editor caught while no map page was
open), newest first, and upload the result, so that the session starts from
what the browser remembers and what was caught elsewhere meanwhile.

## Rationale

The page holds the only store that outlives the server. The editor's side
panel catches through the server, and a catch made while no page was open is
the server's alone: uploading the page's store over it would lose it.

## Acceptance criteria

1. After a page load, the server's session backpack equals the browser's stored
   backpack.
2. An entry the server held that the page did not is in both afterwards; an
   entry both held is the page's.

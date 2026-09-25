---
id: REQ-EXP-008
uuid: 85533120-17fb-4db1-af4c-72383f93f54e
title: Static HTML export opens from file URLs
scope: exp
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

The static HTML export **shall** open from a `file://` URL without a server,
reading all data from the embedded payload and hiding the server-only actions
(save settings, server exports, opening an editor).

## Rationale

The export is a file to share, not a page to host.

## Acceptance criteria

1. Opening the exported file from disk shows the map, the side panel with source
   and the history overlay when one was embedded.
2. The Save button and the server export links are hidden.

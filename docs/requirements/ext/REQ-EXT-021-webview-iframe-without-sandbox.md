---
id: REQ-EXT-021
uuid: 1d801d5d-ab22-43db-90d6-304c60f4f98f
title: Map tab frame without a sandbox
scope: ext
type: constraint
priority: must
status: implemented
source:
  - README.md Why the map is in a tab of its own
verification:
  - extension
  - manual
---

## Statement

The dedicated map tab **shall** consist of one webview containing one iframe of
the map, and that iframe **shall not** carry a `sandbox` attribute; the webview
**shall** be retained while its tab is hidden.

## Rationale

A sandbox can only narrow what the webview was granted, and what it would take
away is the pointer lock walk mode uses; retaining the context avoids reloading
and redrawing the map on every tab switch.

## Acceptance criteria

1. The webview HTML contains an `<iframe>` of the map address without `sandbox`.
2. Walk mode captures the mouse inside the tab.

## Notes

One tab per folder; reopening reuses it with the new address.

---
id: REQ-MAP-024
uuid: f4097f24-187b-4b5f-b9bf-06d632cc7791
title: Hover tooltip describes the box
scope: map
type: functional
priority: must
status: implemented
verification:
  - manual
  - e2e
---

## Statement

The UI **shall** show, for the box under the pointer, a tooltip with the node's
path (or name), and for a file its language and its size in lines, for a
directory its file and line counts, for a symbol its kind and line, and for a
package its ecosystem, version and number of importing files.

## Rationale

The tooltip repeats in words what height and color show, so the map can be read
without the legend.

## Acceptance criteria

1. Hovering a file shows its path, language and lines.
2. Hovering a package shows its ecosystem, version and importers.
3. Moving off every box hides the tooltip.

## Notes

The tooltip appears after the pointer has rested for 240 ms, and then follows
the pointer from box to box without delay.

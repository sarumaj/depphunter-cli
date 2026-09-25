---
id: REQ-MAP-039
uuid: bfece4ad-ce6d-410e-b3bc-43335603bc6c
title: Light and dark themes
scope: map
type: functional
priority: must
status: implemented
verification:
  - manual
  - inspection
---

## Statement

The UI **shall** provide a light and a dark theme, follow the operating system's
preference by default (including changes while the page is open), and let the
user override it with a Theme setting.

## Rationale

Both themes are chosen deliberately in the stylesheet, not derived by inversion.

## Acceptance criteria

1. With Theme set to Auto, switching the OS to dark mode switches the map to the
   dark theme.
2. Setting Theme to Light keeps the light theme regardless of the OS.

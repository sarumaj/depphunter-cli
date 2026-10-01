---
id: REQ-DIST-017
title: Established web libraries
scope: dist
type: constraint
priority: must
status: implemented
verification:
  - inspection
---

## Statement

The UI **shall** use the vendored `fzf-for-js` for fuzzy search instead of
local code. Terrace packing is the map's own (REQ-MAP-043): no library packed
nested terraces densely enough.

## Rationale

Same policy as REQ-DIST-016, applied to the browser modules.

## Acceptance criteria

1. `web/static/core/filter.js` imports `vendor/fzf.es.js`.

---
id: REQ-HUNT-001
uuid: 0c2dd0a1-6f33-4666-b500-3cd9ef254f21
title: Tagging a module by hitting it
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - e2e
  - manual
---

## Statement

When a use of the primary tool reaches a building that the tool can act on and
that module is not yet tagged, the system **shall** tag the module and select
it, so that its dependency trails are lit, exactly as a click on it in the
isometric view would.

## Rationale

The walker hunts dependencies: tagging is the walk-mode equivalent of selecting,
so walking the map selects buildings like clicking does.

## Acceptance criteria

1. A dart that lands on an untagged building selects that module and lights its
   dependency trails.
2. A dart aimed at what the reticle is on tags that module.
3. The ground underfoot and the shore are never tagged.

## Notes

A secondary tool never tags (REQ-TOOL-022).

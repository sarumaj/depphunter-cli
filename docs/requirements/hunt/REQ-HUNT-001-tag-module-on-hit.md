---
id: REQ-HUNT-001
uuid: 0c2dd0a1-6f33-4666-b500-3cd9ef254f21
title: Tagging a module by hitting it
scope: hunt
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M10
  - docs/REQUIREMENTS.md M7
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
so walking the map selects buildings like clicking does (M7 acceptance).
Tracking darts replaced the newspapers thrown before M10.

## Acceptance criteria

1. A dart that lands on an untagged building selects that module and lights its
   dependency trails.
2. A dart aimed at what the reticle is on tags that module (M10 acceptance).
3. The ground underfoot and the shore are never tagged.

## Notes

Before M10 the thrown object was a newspaper; this history is kept here only. A
secondary tool never tags (REQ-TOOL-022).

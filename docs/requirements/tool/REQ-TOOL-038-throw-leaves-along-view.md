---
id: REQ-TOOL-038
uuid: 27c23def-3941-44b0-8b90-f2811e4c0109
title: What a tool throws leaves along the view
scope: tool
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M21
verification:
  - e2e
---

## Statement

A projectile **shall** leave from the tool's muzzle and travel along the view
direction or towards the aimed point, not across the view.

## Rationale

A cast that comes from nowhere or leaves sideways does not look thrown.

## Acceptance criteria

1. A bobber starts at the rod's tip.
2. A shot at nothing flies away along the line of sight.

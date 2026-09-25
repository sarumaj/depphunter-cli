---
id: REQ-TOOL-039
uuid: e75af76b-880a-498d-a1e8-397179fe0fbe
title: Projectiles are unlit
scope: tool
type: constraint
priority: must
status: implemented
verification:
  - ui
---

## Statement

Every projectile **shall** be built of unlit materials.

## Rationale

The scene carries no lights, so a lit material out there is drawn black.

## Acceptance criteria

1. A bubble is blue in the air rather than black.
2. Every projectile mesh uses an unlit material.

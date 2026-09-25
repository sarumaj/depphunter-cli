---
id: REQ-CITY-022
uuid: d9e0a537-87f4-4b15-91aa-ff8cd5a3db01
title: Vegetation as geometry
scope: city
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M10
verification:
  - ui
  - manual
---

## Statement

Trees and bushes **shall** be geometry: bushes along shores and in parks, and
trees of three species (broadleaf, conifer, poplar) along shores and in parks,
with per-vertex shading darker towards the base and foliage colors varied per
plant.

## Rationale

Painted-on vegetation looks flat from the street.

## Acceptance criteria

1. Shores and parks show three distinct tree shapes and bushes.
2. neighboring trees of one species differ in color.

## Notes

When the plant models (`models.js`) have not loaded, `makeProps` falls back to
the `TREES` table. Its entries had `trunk`/`crown` fields while `makeProps` reads
`stem`/`head`, as every other style's table names them, so the fallback trees had
empty geometry yet their trunks still blocked the walker.

Fixed: the `TREES` entries are named `stem`/`head`; `web/uitest/props.test.mjs`
checks that every species drawn without the models has geometry.

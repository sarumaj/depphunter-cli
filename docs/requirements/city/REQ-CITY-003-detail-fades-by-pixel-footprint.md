---
id: REQ-CITY-003
uuid: e0a1c722-f689-441c-a3ab-466eb18b2c67
title: Detail fades by pixel footprint
scope: city
type: non-functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M10
verification:
  - manual
---

## Statement

Procedural texture detail **shall** fade to its mean where a pixel covers
several of its features, so that zooming out does not produce flicker or moiré.

## Rationale

The textures are unlit and antialiased by pixel footprint (`fwidth`).

## Acceptance criteria

1. Zooming the isometric map out fully shows no shimmering windows or asphalt.

---
id: REQ-HUNT-038
uuid: bb988c8a-8c44-403e-abac-1e72d321317a
title: Camera on a tagged module reads it
scope: hunt
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M28
verification:
  - e2e
---

## Statement

A use of the camera on a module that is already tagged, with no bug in front of
it, **shall** open that module's details and **shall not** keep a photograph.

## Rationale

A tool that answered a click differently from the rest would be pressed twice to
find out what it does.

## Acceptance criteria

1. A second click on the same building opens its details instead of keeping a
   picture (M28 acceptance).
2. A bug in front of a tagged wall is still photographed.

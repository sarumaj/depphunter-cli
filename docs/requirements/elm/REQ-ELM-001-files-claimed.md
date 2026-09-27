---
id: REQ-ELM-001
uuid: 96b3e1cd-c20d-4f1f-8f3d-807c17f79456
title: Elm files claimed
scope: elm
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Elm plugin **shall** claim Elm modules (`.elm`) and `elm.json`, telling
`elm.json` apart from other JSON files by its name, and **shall not** claim
anything under `elm-stuff/`, where the compiler keeps its build artifacts
and generated code. The scanner **shall** label `.elm` files Elm.

## Rationale

`elm.json` is the only manifest Elm has; `elm-stuff/` is output, usually
git-ignored, and not part of the project.

## Acceptance criteria

1. `src/Main.elm` and `tests/elm.json` are claimed, `elm.json` is classed
   as a manifest, `elm-stuff/generated-code/x/Main.elm` and
   `app/elm-stuff/elm.json` are not claimed.

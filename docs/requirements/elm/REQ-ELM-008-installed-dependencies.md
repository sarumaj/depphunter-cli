---
id: REQ-ELM-008
uuid: c42192eb-5c22-48bd-a909-09187dc07d02
title: Dependencies of installed packages
scope: elm
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The Elm plugin **shall** answer what a package depends on from its installed
`elm.json` in `ELM_HOME`, without its test dependencies: each at the exact
version an application of the repository lists, else its range, floating;
the resolution report **shall** say the answer came from installed packages.
Without an installed `elm.json` it answers nothing (an application's
`elm.json` lists indirect packages flat, without saying who needs them).

## Rationale

The compiler keeps every package it downloaded, with its `elm.json`, so the
graph below the direct dependencies is on disk after one build.

## Acceptance criteria

1. acme/elm-toolkit 2.0.0 depends on elm/core 1.0.5 and elm/html 1.0.0
   (pinned by the application) and acme/elm-colors `1.0.0 <= v < 2.0.0`.
2. elm/html `1.0.0 <= v < 2.0.0` answers from 1.0.1, not 2.0.0.

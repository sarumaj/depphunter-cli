---
id: REQ-ELM-007
title: Modules resolved to packages
scope: elm
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A module no source directory holds **shall** resolve, in order: to the listed
package whose installed `elm.json` in `ELM_HOME` (else `~/.elm`) exposes
it (`<version>/packages/<author>/<name>/<version>/elm.json`, the exact version
an application lists or the newest installed version a package's range
admits); an elm/core module (Array, Basics, Bitwise, Char, Debug, Dict,
List, Maybe, Platform, Platform.Cmd, Platform.Sub, Process, Result, Set,
String, Task, Tuple) to elm/core, a versioned package like any other; to the
listed package that the longer of a curated module table entry (`Html`
elm/html, `Html.Styled` rtfeldman/elm-css, `Json.Decode.Pipeline`
NoRedInk/elm-json-decode-pipeline) and the package's own name (without
`elm-`, folded: `List.Extra` elm-community/list-extra) names; to a listed
package whose folded name starts with the module's first segment (four
characters or more); a kernel module to elm/core or the listed package it
spells; else to the unresolved package the table names. A module of the
package's own `exposed-modules` missing from `src/`, and a module nothing
names, are dropped.

## Rationale

`elm.json` names packages, not modules. An installed package's `elm.json`
is authoritative; the table and the naming conventions cover a checkout
where nothing was installed.

## Acceptance criteria

1. Without `ELM_HOME` packages, `Widget.Button` is dropped; with
   `acme/elm-toolkit` 2.0.0 installed exposing it, it is that package,
   pinned.
2. `Dict` is elm/core 1.0.5, `Glitter.Sparkle` is
   acme/elm-glitter-effects, `Markdown` is the unresolved
   elm-explorations/markdown and `Mystery.Thing` is dropped.

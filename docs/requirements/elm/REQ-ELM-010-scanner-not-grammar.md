---
id: REQ-ELM-010
uuid: 1b37f592-ce63-42cf-8593-a1771201af94
title: Read by a scanner, not the grammar
scope: elm
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

Elm **shall** be read by a lexer of its own, not a tree-sitter grammar:
`--` and nested `{- -}` comments, single-line and `"""` strings with
escapes, characters, `[glsl| |]` blocks, qualified names as one token,
operators and brackets. A declaration starts at a token in column 0 (Elm's
layout rule), so any input **shall** be read in time linear in its size
without a panic. `elm.json` is read with encoding/json.

## Rationale

The vendored grammar parsed 411 of 412 files of elm/core, elm/html,
rtfeldman/elm-spa-example, mdgriffith/elm-ui and NoRedInk/noredink-ui
cleanly but took 4 to 7 ms per file; everything needed is token-level.

## Acceptance criteria

1. Every prefix of every fixture file and 200 KB runs of each construct
   extract without a panic within the time bound.

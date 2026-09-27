---
id: REQ-ELM-002
uuid: 0df51a2c-3fda-4688-96b9-6c56f75a6b68
title: Imports read
scope: elm
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Elm plugin **shall** read every `import A.B` of a module, with or without
`as C` and an `exposing` list, once per module name, on the line it is
written; the module header (`module`, `port module`, `effect module`) is
not an import, and nothing inside comments (`--`, nested `{- -}` and
`{-| -}`), strings (`"..."`, `"""..."""`), characters or `[glsl| |]`
blocks is read. Modules Elm imports without an import (Basics, List, Maybe
and the rest of the default imports) are not edges.

## Rationale

An import is Elm's only way to use another module, so it is the edge; the
default imports would put the same edge on every file.

## Acceptance criteria

1. `import B.C as D exposing (e, F(..))` and `import E` are read, and
   `import` lines inside a nested block comment, a multi-line string, a glsl
   block or a line comment are not.

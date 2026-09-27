---
id: REQ-ELM-003
title: Symbols
scope: elm
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The Elm plugin **shall** record as symbols the top-level functions and values
(with or without a type annotation, one symbol on the first line naming it),
`type` and `type alias` declarations, the constructors of a custom type as
`Type.Constructor`, `port` declarations and the operators an `infix`
declaration defines; an `elm.json` of a package has the package's name as
its symbol.

## Rationale

These are what other modules can expose and import.

## Acceptance criteria

1. `src/Shop/Cart.elm` has `Item`, `Item.Book`, `Item.Toy`, `Item.Gift`,
   `Cart`, `empty`, `total`, `andThen` and `|>>`; `src/Main.elm` has
   the ports `sendMessage` and `messageReceiver`.

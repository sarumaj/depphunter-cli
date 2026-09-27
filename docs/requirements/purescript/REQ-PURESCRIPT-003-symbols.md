---
id: REQ-PURESCRIPT-003
title: Symbols
scope: purescript
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The PureScript plugin **shall** record as symbols a module's top-level values
and functions (a type signature and its definition count once, on the first
line), `data` and `newtype` types with their constructors as `Type.Ctor`,
type synonyms, classes with their members as `Class.member`, named instances
(`instance showFoo :: Show Foo`, derived and chained ones too), `foreign
import` values and `foreign import data` types, and the operators `infix`,
`infixl` and `infixr` declare; and the package name of `spago.yaml` and
`spago.dhall`.

## Rationale

These are what other modules refer to; instances are named so that the
compiler's generated code and error messages can be found.

## Acceptance criteria

1. `Item`, `Item.Book`, `Priced`, `Priced.price`, `pricedItem`, `showCart`,
   `greet` (foreign), `Handle` and `<+>` are symbols of the fixture.

---
id: REQ-HASKELL-003
title: Top-level definitions extracted
scope: haskell
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** extract a module's name (kind `module`); `data`,
`newtype` and `type` declarations (kind `type`, type operators included);
type and data families (`type family`, `data family`); classes (`class`) with
their method signatures as `Class.method` (`method`); instances, standalone
deriving and type or data family instances named after their head without its
context (`instance Show (Tree a)`, `type instance F Int`, kind `instance`);
pattern synonyms (`pattern`); and top-level functions and values from their
signatures (`f, g :: T`), their definitions (`f x = ...`, guards, operators
defined prefix or infix, backquoted) and foreign imports (`function`), each
once, at its first line. A `.cabal` file's components are symbols of kind
`component`.

## Rationale

These are the definitions a Haskell module is made of; instances carry much
of a Haskell program and are named the way Haddock lists them.

## Acceptance criteria

1. The fixture's `Shop/Types.hs` gives `Item`, `Price`, `Label`, `:+:`, the
   family `Elem`, `type instance Elem [e]`, the class `Priced` with
   `Priced.price`, `Priced.discount` and `Priced.surcharge`, three instances,
   the pattern `Free` and the functions `label`, `<+>`, `total` and `foldl'`.
2. A Template Haskell splice at the top level (`makeLenses ''T`) and a bang
   pattern (`f !x = ...`) do not name a function after themselves.

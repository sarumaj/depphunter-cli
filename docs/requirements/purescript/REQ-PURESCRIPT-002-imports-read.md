---
id: REQ-PURESCRIPT-002
title: Imports read
scope: purescript
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The PureScript plugin **shall** read every `import` of a module - plain,
qualified (`as C`), with an import list (`(x, T(..))`) or `hiding` - as an
import of the module it names, once per module, and a module that declares
a value with `foreign import` as needing its JavaScript companion. Comments
and strings **shall not** yield imports.

## Rationale

Imports are the edges between modules; a foreign import is the edge from a
module to the JavaScript that implements it.

## Acceptance criteria

1. `import Data.Maybe (Maybe(..))`, `import Shop.Cart (empty) as Cart` and
   `import Data.Maybe hiding (fromJust)` are imports; `import` inside a
   `{- -}` comment, a `--` comment, a string or a `"""` string is not.

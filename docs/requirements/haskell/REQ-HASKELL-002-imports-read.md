---
id: REQ-HASKELL-002
title: Import declarations read
scope: haskell
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read every `import` declaration of a module, including
`safe` and `qualified` imports (before or after the module name), `as` and
`hiding` clauses and import lists, PackageImports (`import "pkg" M`, the
package kept for resolution) and `{-# SOURCE #-}` imports (resolved to the
module's `.hs-boot` file). Its spec is the declaration without its import
list, so two imports of one module stay apart. Imports inside every branch of
C preprocessor conditionals are read, once per distinct spec.

## Rationale

Imports are the only declared edges between Haskell modules and from a module
to the packages it uses.

## Acceptance criteria

1. `import {-# SOURCE #-} Shop.Types` goes to `Shop/Types.hs-boot`,
   `import Shop.Types` to `Shop/Types.hs`.
2. `import qualified "containers" Data.Set as Set` goes to `containers`,
   `import "this" Shop.Parser` to the importing package's own file.
3. An `import` inside a `{- -}` comment, a string or a literate document's prose
   is not read.

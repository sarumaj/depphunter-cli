---
id: REQ-R-003
uuid: c81a9f46-6087-4e72-853b-5b70fd6d1938
title: R definitions extracted
scope: r
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** extract the top-level definitions of R code: functions
(`f <- function`, `f = \(x)`, `assign("f", function ...)`, backquoted and
string names), R6, reference and S7 classes with their methods as
`Class.method` (R6 `public`, `private`, `active`; `setRefClass` `methods`;
`Class$methods(...)`), S4 classes (`setClass`), generics (`setGeneric`) and
methods (`setMethod` as `Signature.generic`), and variables where they are
first assigned; definitions inside function bodies and blocks are not
symbols.

## Rationale

What a file defines is what other files call; R code defines almost
everything by assignment at the top level.

## Acceptance criteria

1. The fixture's `pkgs/shopr/R/cart.R` and `utils.R` give exactly their
   functions, classes, methods, generic and variables.
2. A function assigned inside another function, or on a line continuing an
   expression, is not a symbol.

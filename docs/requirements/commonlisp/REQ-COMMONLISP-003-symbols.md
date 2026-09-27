---
id: REQ-COMMONLISP-003
title: Symbols
scope: commonlisp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The top-level definitions of a file **shall** be its symbols: `defun`
(`(setf name)` functions too) and `defgeneric` as func, `defmacro` as
macro, `defmethod` as method named by its function, qualifiers and
specializers (`print-object (item t)`, `total :around (cart)`), `defclass`
and `define-condition` as class, `defstruct` as struct, `deftype` as type,
`defvar` and `defparameter` as var, `defconstant` as const, `defpackage`
(and `define-package`) as package and `defsystem` as system. A name
defined twice (on both sides of a reader conditional) **shall** count
once.

## Rationale

Definitions are what the map's buildings show inside a file.

## Acceptance criteria

1. `src/model/item.lisp` yields each kind above; `src/model/cart.lisp`'s
   definitions inside `eval-when` and `let` count, and its `#+sbcl` and
   `#-sbcl` definitions of one function give one symbol.

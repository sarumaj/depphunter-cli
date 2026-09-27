---
id: REQ-COMMONLISP-002
title: Imports read
scope: commonlisp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read as imports, at top level (and inside `progn`,
`eval-when`, `let`, `when`, `unless` and the like): a `defpackage` or
`uiop:define-package` form's `:use`, `:import-from`,
`:shadowing-import-from`, `:local-nicknames`, `:mix` and `:reexport`
options, `in-package`, `ql:quickload`, `asdf:load-system` (and
`load-systems`, `require-system`, `operate`, `oos`, `make`), `require` and
`load` of a literal path; in a `defsystem` form (any file, usually an
`.asd`), `:depends-on`, `:defsystem-depends-on` and `:weakly-depends-on`
entries (a name, `(:version name v)`, `(:feature f entry)`,
`(:require module)`) and every file of `:components` (modules and
`:pathname` followed, a component's type giving its extension); and,
anywhere in a file, the package of each qualified symbol (`pkg:sym`,
`pkg::sym`) once, a local nickname read as its package. Comments,
strings, characters, quoted data and forms behind a reader conditional
that is false everywhere (`#+nil`, `#+(or)`, `#-(and)`) **shall** not be
read.

## Rationale

ASDF systems link files and name dependencies; packages link a file to
the file or library defining the package it uses.

## Acceptance criteria

1. `shop.asd`'s entries and components, `src/package.lisp`'s options,
   `scripts/build.lisp`'s quickloads, requires and loads and
   `src/model/cart.lisp`'s qualified symbols are imports; nothing in
   `src/input-output/reader.lisp` but its `in-package` is.

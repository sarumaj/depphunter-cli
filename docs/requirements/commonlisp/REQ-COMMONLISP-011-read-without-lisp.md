---
id: REQ-COMMONLISP-011
title: Read without a Lisp
scope: commonlisp
type: limitation
priority: should
status: implemented
verification:
  - unit
---

## Statement

Common Lisp **shall** be read without running a Lisp, ASDF, Qlot or
ocicl: macros are not expanded (a package defined by a macro other than
`defpackage` or `define-package`, or a system by one other than
`defsystem`, is not seen), `.asd` code is not evaluated (a component
class's file type is found only by the one file of the component's name,
`#.` values are not computed), reader conditionals keep both branches,
`*features*` are not known, `~/quicklisp` and systems Qlot or ocicl
installed outside the repository are not read, and a package no file of
the repository defines is attributed by the declared systems' names and a
curated table.

## Rationale

Which packages and components exist is known only to a Lisp image that
has loaded the systems.

## Acceptance criteria

1. A `defun` inside a quoted list or behind `#+nil` is not read.

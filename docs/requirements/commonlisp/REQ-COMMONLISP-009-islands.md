---
id: REQ-COMMONLISP-009
title: Islands
scope: commonlisp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** declare two islands: `quicklisp` ("Quicklisp
projects", git sources named by repository) and `cl-std` ("Common Lisp
built-ins", standard library), and emit no other.

## Rationale

External projects and the implementation are what a Lisp project's
dependencies divide into.

## Acceptance criteria

1. Every import of the fixture resolves locally, to `quicklisp` or to
   `cl-std`.

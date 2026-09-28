---
id: REQ-DHALL-003
title: Symbols
scope: dhall
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The bindings of the `let` chain a Dhall file starts with **shall** be its
symbols (kind `type` for a capitalized name, `function` for a lambda,
`let` otherwise), and so **shall** the fields of the record the file
evaluates to at its top (kind `field`). The file **shall** be read by the
Dhall reader the PureScript plugin uses for spago's files, which skips what
it does not understand of a binding up to the next `let` or `in`.

## Rationale

A Dhall package is a record of imports (`package.dhall`), a Dhall module a
chain of lets; both are what other files select from.

## Acceptance criteria

1. `config/app.dhall` has its twelve bindings and the two fields of the
   record it returns; `package.dhall` has its two fields.

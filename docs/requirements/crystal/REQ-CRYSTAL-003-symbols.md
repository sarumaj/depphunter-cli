---
id: REQ-CRYSTAL-003
uuid: 43577167-a33f-4c95-aa14-502ede6d41c6
title: Symbols
scope: crystal
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** record modules, classes, structs, enums, libs (C
bindings), annotations and a lib's unions and structs by their qualified
names (`Shop::Models::User`, nesting and `A::B` names joined), methods as
`Owner.name` for instance and class methods alike (as the Ruby plugin
names them; operators, `[]`, `[]?`, `name=` and `name?` kept), top-level
`def`s and a lib's `fun`s as functions, macros, constants, `alias`es and a
lib's `type`s, `record` structs (with a `do` body's methods owned by the
record), and the attributes `getter`, `setter`, `property` and their
`?`/`!`/`class_` forms declare. Nothing inside a `def` body or a macro
definition's template is a symbol, and an enum's members are not constants.
Of the branches of a macro `{% if %}` / `{% else %}`, each **shall** be read
from the same nesting, the first branch's nesting continuing after
`{% end %}`.

## Rationale

Names match what Crystal's documentation and the Ruby plugin show; a macro
template's `def {{name}}` has no name before the macro runs.

## Acceptance criteria

1. The fixture's `src/shop.cr` yields exactly its listed symbols, among
   them `Shop::LibSSL.ssl_init` (func), `Shop::Status.open?`,
   `Shop::Store.[]`, `Shop::Store.name=`, `Shop::Store.begin`,
   `Shop::Price.to_s` and `Shop::Item.active` (attr), and not the `def`s
   inside `macro delegate_all`.
2. `def f(a : Int32)` and `def f(a)` in the two branches of a `{% if %}`
   leave the next method of the module a method of the module.

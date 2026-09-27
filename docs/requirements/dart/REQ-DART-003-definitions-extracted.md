---
id: REQ-DART-003
title: Dart definitions extracted
scope: dart
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** extract classes (including `mixin class`, `sealed`,
`base`, `final`, `interface` and `abstract` classes), mixins, enums, named
extensions, extension types, typedefs, top-level functions, getters, setters
and variables, and, in the body of each class-like declaration, constructors
(`Owner.new` for the unnamed one, `Owner.name` for a named or factory one),
methods and operators (`Owner.operator==`), getters and setters (one property
per name) and fields as `Owner.name`, and **shall not** read function bodies.

## Rationale

The map sizes and labels buildings by their definitions; a body holds none
the map shows.

## Acceptance criteria

1. A class with a named constructor, a factory, an operator, a getter and
   setter pair and a static const field yields `Product.free`,
   `Product.fromJson`, `Product.operator==`, one `Product.label` property and
   `Product.zero` of kind const.
2. A `class` inside a string or block comment is not a definition.

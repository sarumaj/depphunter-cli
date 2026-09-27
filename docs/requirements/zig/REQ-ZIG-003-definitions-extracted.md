---
id: REQ-ZIG-003
title: Definitions extracted
scope: zig
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

Functions (`fn`, with `pub`, `export`, `extern` or `inline`), tests (`test
"name"`, `test name`), and constants and variables at the top level of a
file **shall** be symbols, and a constant whose value is a `struct`, `enum`,
`union`, `opaque` (with `extern`, `packed` or a tag type) or an error set
**shall** be a type; members of such a container **shall** be read with the
type as owner (`Cart.Item`, `Cart.add` as a method), nested types included.
A constant naming an `@import` or `@cImport`, a `comptime` block, fields and
declarations inside function bodies (the struct a generic function returns)
**shall not** be symbols.

## Rationale

The map shows what a file defines; imports are edges, not definitions.

## Acceptance criteria

1. `src/shop.zig` yields `Cart`, `Cart.Item`, `Cart.Item.price`,
   `Cart.add`, `Color`, `Shape.area`, `Error`, `Pair`, the tests and its
   constants and variables, and not `Cart.nested`'s local struct.

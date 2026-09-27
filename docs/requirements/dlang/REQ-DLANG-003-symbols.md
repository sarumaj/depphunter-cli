---
id: REQ-DLANG-003
title: Declarations extracted
scope: dlang
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The D plugin **shall** extract a module's `module` name (kind `module`), its
classes, structs, interfaces, unions, enums, templates and mixin templates
with their nested ones qualified by their owners (`Cart.Line`), functions
(`func`) and member functions, constructors and destructors
(`Owner.name`, `Owner.this`, `Owner.~this`, kind `method`), aliases (old and
new syntax, not `alias this`) and top-level manifest constants (`enum x =
...`, `enum T a = 1, b = 2`, eponymous `enum isX(T) = ...`), including those
inside `version`, `debug`, `static if` and attribute blocks and after
attribute labels (`private:`). Declarations inside function bodies and
unittests **shall** not be symbols.

## Rationale

Aggregates and functions are what references and the map's buildings
show; nested scopes inside functions are local detail.

## Acceptance criteria

1. `source/shop/cart.d` yields exactly the symbols the test lists, among
   them `Cart.Line.total`, `Cart.~this`, `Logged` (mixin template) and
   `platform` twice (`version` and `else` branches); the struct inside
   the unittest is not a symbol.

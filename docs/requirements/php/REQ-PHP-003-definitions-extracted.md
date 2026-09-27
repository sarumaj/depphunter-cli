---
id: REQ-PHP-003
uuid: 5997912f-9ad9-4369-a908-d28256072314
title: Definitions extracted
scope: php
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** record as symbols each namespace a file declares, each
class, interface, trait and enum, each function not defined inside another
function or closure, each method and class constant as `Owner.name`, each
constant of a `const` statement outside a class, and each `define()` of a
literal name. Methods of anonymous classes **shall not** be recorded.

## Rationale

These are the names other files import and the language servers answer for.

## Acceptance criteria

1. A class with a constant and two methods yields the class, `Class.CONST`
   and both `Class.method` symbols.
2. A function declared inside a closure is not a symbol; one declared inside
   an `if (!function_exists(...))` block at the top level is.
3. An enum, a trait and an interface yield symbols of those kinds with their
   methods.

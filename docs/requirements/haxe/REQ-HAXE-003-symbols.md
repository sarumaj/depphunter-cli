---
id: REQ-HAXE-003
title: Symbols
scope: haxe
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** report a module's types by name - `class` (also
`abstract class`, `final class`, `extern class`), `interface`, `enum`,
`enum abstract` (kind `enum`), `abstract` and `typedef` - their functions as
`Type.name` (kind `method`, constructors as `Type.new`), and module-level
functions (`func`) and variables (`var`). Functions inside function bodies,
fields of structures (`typedef X = { function f():Void; }`) and enum
constructors **shall** not be symbols. A type declared in several `#if`
branches **shall** be one symbol, and braces **shall** follow the first
branch.

## Rationale

A Haxe module holds several types; the map sizes a file by its types and
methods.

## Acceptance criteria

1. `src/shop/Main.hx` has `Main`, `Main.new`, `Main.main`, `Main.helper`,
   `Extra` (declared in both branches of `#if flash`) and `Extra.run`,
   `Service` with `Service.call` and `Service.stop`, `Color`, `Level`
   (`enum abstract`) with `Level.isHigh`, `Meters` with `Meters.new` and
   `Meters.add`, `Options`, `Base` (`abstract class Base<T:{}>`) with
   `Base.run`, `helperAtModuleLevel` and `VERSION`.

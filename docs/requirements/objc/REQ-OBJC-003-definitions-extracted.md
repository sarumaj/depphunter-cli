---
id: REQ-OBJC-003
title: Objective-C definitions extracted
scope: objc
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** record as symbols the classes of `@interface` and
`@implementation` (kind `class`), categories as `Class(Category)` (kind
`extension`; a class extension `Class()` adds none), protocols (kind
`interface`), methods as `Owner.selector` with the full selector
(`Cart.initWithItems:`, `Cart.addItem::`; kind `method`, class and instance
methods alike, a category's methods owned by its class), properties as
`Owner.name` (block properties included), C functions (kind `func`),
`NS_ENUM`/`NS_OPTIONS`-style enumerations (kind `enum`), typedefs (kind
`type`, or `struct`/`enum`/`union` for a typedef of a body), top-level
constants and variables, and `#define` macros with a value or parameters. A
declaration the file also defines (an `@interface` method implemented below, a
prototype) **shall** be one symbol, at its definition.

## Rationale

These are what other files use; the selector is the name the method is called
by, and `Owner.name` is the naming every plugin uses for members.

## Acceptance criteria

1. `Cart.h` gives `Cart`, `Cart.items` (property), `Cart.initWithItems:`,
   `Cart.addItem::`, `Cart.formatted:`, `CartObserver` (interface),
   `CartState` and `CartFlags` (enum), `CartCompletion` (type), `CartTotal`
   (struct), `CartDidChangeNotification` (const), `CartTotalMake` (func) and
   the macros `CART_MAX_ITEMS` and `CART_LOG`, not the header guard.
2. `Cart+Pricing.h` gives `Cart(Pricing)` and `Cart.total`; an Objective-C++
   file gives its C++ class and function too.

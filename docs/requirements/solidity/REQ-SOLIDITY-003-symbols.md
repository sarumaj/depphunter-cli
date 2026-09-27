---
id: REQ-SOLIDITY-003
title: Symbols
scope: solidity
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The symbols of a Solidity file **shall** be its contracts and abstract
contracts and its libraries (kind class), interfaces (interface), and
their members named `Owner.member`: functions, the constructor, `receive`
and `fallback` (method), modifiers, events, errors, structs, enums,
user-defined value types and constants; and the free functions (func),
structs, enums, events, errors, user-defined value types and constants
declared at the file's level. An overloaded function gets a symbol per
declaration. Functions of Yul in `assembly` blocks **shall** not be
symbols.

## Rationale

Contracts and their members are what a reader of a Solidity project looks
for; Yul helpers are local to their block.

## Acceptance criteria

1. The fixture's `Counter.sol`, `Math.sol`, `Local.sol` and `Helper.sol`
   give exactly the listed symbols, with their lines.

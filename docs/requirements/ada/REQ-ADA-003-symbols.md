---
id: REQ-ADA-003
uuid: cdbd1f3a-4c5a-4fe0-8e62-edc0668b85ad
title: Symbols
scope: ada
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** list as symbols a file's library units (package,
procedure, function, generic units and instantiations, a subunit as
`Parent.Name`) and the declarations of the packages, tasks and protected
units in them, named `Unit.Name`: nested packages, procedures and
functions (operators as `Unit."+"`), and types by kind: `class` for a
tagged or derived-with-extension type, `struct` for a record,
`interface`, `enum`, `task` and `protected` types and objects, and
`type` for the rest and for subtypes. A declaration seen twice (a spec
and its completion, overloads) **shall** be one symbol. Declarations local
to a subprogram body and generic formals **shall not** be symbols. A
project file's symbol **shall** be its project and an `alire.toml`'s its
crate.

## Rationale

The map sizes and names nodes by what a unit declares.

## Acceptance criteria

1. `src/shop-cart.ads` has Shop.Cart (package), Item (struct), Cart
   (class), Shape (interface), Count (type), Color (enum), Add, Total and
   `"+"`, Item_Vectors (package), Worker (task), Lock (protected) and
   Lock.Seize; `tools/all_units.ada` has Tools, Tools.Run and Tools_Main.

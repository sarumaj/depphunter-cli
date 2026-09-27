---
id: REQ-ADA-002
uuid: 9824d163-60d1-4f10-bbd7-0cec21d619e5
title: With clauses and what a unit's name implies
scope: ada
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The scanner **shall** read every context clause's with clause (`with A.B,
C;`, `limited with`, `private with`, `limited private with`),
comparing names without case, and **shall** add what a compilation unit's
name implies: a body (`package body P`, a subprogram body) imports its own
spec, a child unit (`package A.B`) its parent's spec, and a subunit
(`separate (P)`) its parent's body. `use`, `use type` and `use all type`
clauses **shall not** be imports. Comments (`--`), strings (with doubled
quotes), character literals (`'x'`, `'''`, `'"'`) and attribute ticks
(`X'First`, `T'('c')`) **shall** neither hide a with clause nor make one
up. Of a gnatprep `#if` only the first branch **shall** be read.

## Rationale

Ada names its dependencies in with clauses only; a body, a child and a
subunit depend on their parents without naming them.

## Acceptance criteria

1. `src/shop-cart.ads` imports Ada.Containers.Vectors,
   Ada.Strings.Unbounded, `limited with Shop.Orders`, `private with
   Shop.Internal`, `WITH gnatcoll.json` and its parent Shop, and not
   `Fake.Comment` or `Fake.String`.
2. `src/shop-cart.adb` imports its spec, `src/shop-cart-total.adb` its
   parent body, and a source with fake with clauses in comments, strings
   and a gnatprep `#else` branch next to character literals and ticks
   imports only the real ones.

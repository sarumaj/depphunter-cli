---
id: REQ-LSP-008
uuid: fcd59876-0f8a-49f7-a8cd-7bd0f0525efd
title: Positions sent in UTF-16 code units
scope: lsp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** send positions to language servers with columns counted in
UTF-16 code units.

## Rationale

Decision: UTF-16 is the LSP default position encoding.

## Acceptance criteria

1. `Area` in `const π = 3; func Area() {}` is at column 18.
2. In `let 😀 = 1; function f() {}` `f` is at column 21 (the emoji counts two
   units).

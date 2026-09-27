---
id: REQ-CPP-007
uuid: 573c8713-be6a-4d88-8c8c-ee97185ffd59
title: #if 0 blocks skipped
scope: cpp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The C/C++ plugin **shall** skip the includes and definitions of an
`#if 0` (or `#if false`) branch and of the branches after an `#if 1`
(or `#if true`); every other branch **shall** be read.

## Rationale

`#if 0` is the usual way to disable code, and its includes are no
dependency.

## Acceptance criteria

1. An include in `#if 0` ... `#endif` is not recorded, while one in its
   `#elif defined(X)` branch is.
2. A function in `#if 0` is no symbol.

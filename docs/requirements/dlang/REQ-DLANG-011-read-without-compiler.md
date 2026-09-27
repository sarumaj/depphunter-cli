---
id: REQ-DLANG-011
title: Read without the compiler or dub
scope: dlang
type: limitation
priority: should
status: implemented
verification:
  - unit
---

## Statement

D **shall** be read without running the compiler or dub: code a `mixin`
generates is not read, every branch of `version`, `debug` and `static if`
counts, an import's module is found by name only (no `-I` flags beyond
dub's recipes), a module of a package dub has not fetched is attributed by
the curated table or the declared name it spells, every platform's
settings count at once, dub's own settings (`registryUrls`) are not read,
and a string import needs a literal path.

## Rationale

What mixins and conditional compilation produce is known only to the
compiler; which package provides a module only to dub.

## Acceptance criteria

1. `enum code = q{ import fake.two; };` yields no import; both `platform`
   functions of a `version`/`else` pair are symbols.

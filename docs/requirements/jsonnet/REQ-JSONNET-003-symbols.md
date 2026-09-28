---
id: REQ-JSONNET-003
title: Symbols
scope: jsonnet
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A Jsonnet file's leading top-level `local` bindings (functions as `func`,
other values as `var`) and the fields of the object literals its body
evaluates to at the top (`a: e`, hidden `a:: e`, `a+: e`, quoted names;
methods `f(x): e` and fields bound to a `function` as `func`) **shall** be
its symbols; computed fields, object locals, assertions, comprehensions
and nested fields **shall** not.

## Rationale

What a library file exports is its top-level object.

## Acceptance criteria

1. The fixture's `environments/prod/main.jsonnet` has its locals and its
   top-level object's fields (including those of `{...} + {...}`) as
   symbols, and `app.libsonnet`'s `function(config) {...}` its field.

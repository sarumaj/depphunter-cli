---
id: REQ-CRYSTAL-011
title: Read without the compiler or shards
scope: crystal
type: limitation
priority: should
status: implemented
verification:
  - unit
---

## Statement

Crystal **shall** be read without running the compiler or shards: methods a
macro defines (`def {{name.id}}`, `getter` of a macro argument) are not
symbols, a require computed by a macro is not read, only the first branch
of a macro `{% if %}` decides the nesting after it, the `CRYSTAL_PATH`
environment variable is not read, `@[Link("ssl")]` library annotations are
not mapped to system libraries, and a shard is named by its `shard.yml` key,
not its repository.

## Rationale

What macros expand to is known only to the compiler.

## Acceptance criteria

1. `require "./shop/windows"` in a `{% if flag?(:win32) %}` is read (and
   dropped as missing); `def {{to.id}}_name` in a macro is no symbol.

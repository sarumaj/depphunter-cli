---
id: REQ-CRYSTAL-004
title: Requires resolved to files
scope: crystal
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

`require "./x"` and `require "../x"` **shall** resolve relative to the
requiring file to `x.cr`, else `x/x.cr`; `require "./dir/*"` **shall**
resolve to each `.cr` file of `dir` and `require "./dir/**"` to each below
it, in path order, the requiring file excepted, each shown by its path. A
require by name **shall** resolve to the project's own files first when its
first segment is the shard's own name (`shop/cart` is `src/shop/cart.cr`,
`lucky/tasks/exec` is `tasks/exec.cr`, by the compiler's rules for a shard:
`y.cr` at the shard's root, `src/y.cr` or `src/x/y.cr`), else to a file of
the project's `src/` (Crystal's own standard library requiring itself), and
`require "c/x"` in Crystal's repository to its `src/lib_c/<target>/c/x.cr`
(x86_64-linux-gnu first). A path dependency's files resolve to the file in
its directory. A require of no file is dropped.

## Rationale

The compiler reads relative requires by path and named ones through
CRYSTAL_PATH (`lib/` and the standard library); a shard's specs and `bin/`
templates require the shard by name.

## Acceptance criteria

1. `./shop/*` from `src/shop.cr` gives `src/shop/cart.cr` and
   `src/shop/version.cr`, `./shop/models/**` also
   `src/shop/models/admin/role.cr`; `./missing` is dropped.
2. `shop/version` and, from `spec/`, `shop/cart` are the shard's own files;
   `widgets/button` is `libs/widgets/src/widgets/button.cr` of a path
   dependency.

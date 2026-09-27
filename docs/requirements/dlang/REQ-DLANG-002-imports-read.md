---
id: REQ-DLANG-002
title: Imports read
scope: dlang
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The D plugin **shall** read every module an `import` declaration names -
lists (`import a.b, c;`), renamed imports (`import io = std.stdio;`),
selective imports (`import std.algorithm : map, filter;`), `static`,
`public` and other attributed imports, and imports scoped inside
functions, unittests and aggregates - as an import of that module, once per
file, and every `import("file")` expression with a literal path
(`mixin(import("x"))` included) as a string import. Nothing inside a
comment (`//`, `/* */`, nested `/+ +/`), a string of any form (`"..."`,
`r"..."`, backtick, `x"..."`, `q"(...)"`, `q"EOS ... EOS"`), a token string
`q{ ... }` or a character literal, and nothing after `__EOF__`, **shall** be
read as an import.

## Rationale

Imports are how one D module uses another; string imports embed files
from the string import directories.

## Acceptance criteria

1. The fixture's `source/shop/app.d` imports `std.format` for
   `import fmt = std.format;` and `std.conv` from inside `main`, and
   `import("banner.txt")`; the `fake.*` imports inside comments, strings
   and a token string are not read.
2. Imports after `__EOF__` are not read.

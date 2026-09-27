---
id: REQ-PHP-002
uuid: 22564daf-011b-4cba-a1f5-d3baa7af72d6
title: Use statements and qualified names read
scope: php
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** record as imports every name a `use` statement imports,
with grouped (`use A\{B, C as D}`), `use function` and `use const` forms
expanded to one import each, and the names code refers to: the class of
`new`, of a static call or class constant (`X::`), of `extends`,
`implements`, a trait `use` and a `catch`, and a fully qualified function
call (`\f()`). A name in code **shall** be qualified as PHP qualifies it: a
leading `\` makes it fully qualified, a first segment that a `use` statement
aliased is replaced by the alias's name, and any other name is relative to the
namespace (braced or not) it is written in; a name that is exactly an alias is
already imported by its `use` statement and is not recorded again, and a
function call that is neither fully qualified nor aliased is not recorded.
Each name in code **shall** be recorded once per file.

## Rationale

Composer autoloads by class name, so the names a file imports and uses are
what ties it to other files and packages; `use` statements alone miss the
fully qualified names and the classes of the file's own namespace, which need
no `use`.

## Acceptance criteria

1. `use A\{B, C\D as E, function f, const G}` yields `A\B`, `A\C\D`,
   the function `A\f` and the constant `A\G`.
2. `extends Base` in `namespace A { ... }` yields `A\Base`, and
   `extends Y\Z` after `use X\Y` yields `X\Y\Z`.
3. `new Client` after `use GuzzleHttp\Client` adds no import beside the
   `use` statement's.
4. `\strlen()` is recorded; `strlen()` is not.

## Notes

A name relative to the file's namespace resolves only to a project file;
nothing else is looked up for it (REQ-PHP-005).

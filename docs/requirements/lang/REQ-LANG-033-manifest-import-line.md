---
id: REQ-LANG-033
title: An import's line is the manifest line that declares it
scope: lang
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

An import a plugin reads from a TOML manifest **shall** carry the line that
declares it: the first line that defines its key in the table it was read
from, as a `key = ...` pair, a dotted key through it (`key.git = ...` or, at
the top level, `table.key = ...`), a table header through it
(`[table.key]`) or a pair of an inline table (`table = { key = ... }`). The
line **shall** not depend on how the file is written: whitespace or tabs
around `=` and inside headers, a comment after a header, bare, `"basic"` or
`'literal'` keys, and CRLF line endings. A key of another table, a key inside
an inline table that is the value of another key, and text inside a string,
multi-line strings included, **shall** not be taken for it.

## Rationale

The line is what the side panel and the open-in-editor action show for a
declared dependency. The TOML decoder keeps no positions, and six plugins
each searched the text their own way; five of them gave line 0 (shown as 1)
or another key's line on valid TOML.

## Acceptance criteria

1. A table test of the scanner covers table headers (`[a.b]`, `[ "a.b" ]`,
   `[[array]]`, with whitespace and comments after them), bare and quoted
   keys, dotted keys at the top level and in tables, tabs around `=`,
   comments, multi-line basic and literal strings, inline tables and CRLF.
2. A wally.toml dependency after a tab before `=`, under
   `[dependencies] # comment` or written as `'Name'` is on its own line.
3. A foundry.toml dependency under `[dependencies] # comment` or written as a
   top-level `dependencies.name = ...` is on its own line.
4. An fpm.toml `dependencies.foo.git` after `dependencies.foobar.git` puts
   foo on its own line, not foobar's.
5. A gleam.toml dependency is not on the line of a key of the same name in
   another table, in a string or in another dependency's inline table.
6. A Project.toml `deps.Foo = ...` or `'Bar' = ...` dependency, and an
   alire.toml dependency written as a dotted key in `[[depends-on]]` or in a
   `case(...)` table, is on its own line.

## Notes

`lang.TOMLKeyLines` holds the rule. Alire's `depends-on` looks in the table
and every table below it (`lang.KeyLines.Within`), so a dependency in a
`case(os)` alternative is found.

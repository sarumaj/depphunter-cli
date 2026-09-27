---
id: REQ-CRYSTAL-002
title: Requires read
scope: crystal
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read each `require "..."` whose argument is a plain
string literal, wherever it is written (inside a macro `{% if %}` too), and
**shall not** read what only looks like one inside a comment, a string (with
`#{...}` interpolations), a heredoc (`<<-EOS`, `<<-'EOS'`), a `%()`,
`%q()`, `%Q[]`, `%w()` or other %-literal, a regex, a char literal or a
macro tag; a require of an interpolated string or of a macro expression
(`require "./{{name}}"`) is not read.

## Rationale

`require` is the only way one Crystal file reaches another file or a shard;
what a macro computes is not known without running the compiler.

## Acceptance criteria

1. The fixture's requires hidden in a heredoc, a `%()` literal, a regex, a
   comment, an interpolation and a `{% for %}` body are not imports, while
   `require "./real"` is.

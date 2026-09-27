---
id: REQ-FORTRAN-002
uuid: 8bba3b47-b35c-4c0a-be38-ab51be145a97
title: Statements and imports read
scope: fortran
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read Fortran statements in free form (`!`
comments, `&` continuation with an optional leading `&` on the next line,
`;` separating statements) and in fixed form (`C`, `c`, `*`, `!`, `D` or
`d` in column 1 comments a line, a character other than blank or zero in
column 6 continues it, labels in columns 1-5, tab-format lines), keywords
case-insensitively, strings in either quote with the quote doubled inside,
and preprocessor lines (`#...`, fypp's `#:`, `$:` and `@:` lines) apart
from statements, reading both branches of `#if`/`#ifdef`. A file with a
fixed-form extension that puts code into columns 1-5 **shall** be read in
free form. OpenMP conditional compilation lines (`!$ use omp_lib`) **shall**
be read as code. It **shall** record as imports `use m`, `use :: m`,
`use, intrinsic :: m`, `use, non_intrinsic :: m` (with `only:` lists and
renames), a submodule's parent (`submodule (ancestor[:parent]) name`),
`include 'file'`, `#include "file"`, `#include <file>` and fypp's
`#:include "file"`, each once per file. Old-style `external` declarations
and `call` statements **shall** not be recorded.

## Rationale

A module is used by name; nothing in a comment or a string is an import.
Calls of external procedures name no file or package and would only add
noise.

## Acceptance criteria

1. `use` in a comment, a string, a string continued across lines and a
   fixed-form comment line are not imports.
2. A `use` statement split by `&` (with a comment line between) and one
   in fixed form continued in column 6 are read; `use = 3` and `used = 4`
   are not imports.
3. `!$ use omp_lib` is an import and `!$omp parallel` is not.

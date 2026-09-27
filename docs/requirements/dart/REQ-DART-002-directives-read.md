---
id: REQ-DART-002
uuid: 693e2159-a51a-460f-9376-8e3b58ae5033
title: Import, export and part directives read
scope: dart
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read a library's `import` and `export` directives with
the URI of every configuration (`if (dart.library.io) 'io.dart'`) as a
separate import, shown with its condition, and its `part` and `part of`
directives whose URI is a plain string literal, and **shall** stop reading
directives at the first declaration.

## Rationale

Every configuration of a conditional import is compiled on some platform, so
each is a dependency; directives may only precede declarations.

## Acceptance criteria

1. An import with two configurations yields three imports: the default URI and
   one per condition.
2. `part 'model.part.dart'` and `part of 'model.dart'` connect the two files.
3. Text in comments, strings and interpolations is never read as a directive.

## Notes

`part of` naming a library (`part of shop.model;`) rather than a URI is not
resolved. The source is read by a scanner of the plugin's own, not the
tree-sitter grammar: measured on flutter/gallery, felangel/bloc and
dart-lang/http, the grammar took 25-45 ms per file and failed on 5-11% of
the files, sometimes losing the whole file.

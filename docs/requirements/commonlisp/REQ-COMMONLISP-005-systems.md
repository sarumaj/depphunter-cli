---
id: REQ-COMMONLISP-005
title: ASDF systems and components
scope: commonlisp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A `defsystem`'s components **shall** be edges from its file to the
component files: paths relative to the file's directory joined with the
system's `:pathname`, each module's name or `:pathname`, the component's
name or `:pathname`, and the type's extension (`.lisp` for source files,
none for static files, `:type` when given); a component of a class the
`.asd` defines itself resolves to the one file with its name. A system
**shall** resolve to the `.asd` defining it (secondary `foo/bar` systems
included); a system `foo/bar/baz` of a package-inferred system `foo`
(`:class :package-inferred-system`) to the file `bar/baz.lisp` under
`foo`'s directory and `:pathname`; a system the file itself defines is
dropped.

## Rationale

ASDF is how Common Lisp builds; its components are the file graph of a
project, and package-inferred systems name their files as systems.

## Acceptance criteria

1. `shop.asd`'s components resolve to `src/package.lisp`,
   `src/model/item.lisp`, `src/input-output/reader.lisp`, `README.md`,
   `src/grovel.lisp` and `src/old/legacy.lisp`; `src/missing.lisp` is
   dropped.
2. `pis/acme.asd`'s `acme/main` resolves to `pis/src/main.lisp`;
   `quickload shop` in a script to `shop.asd`.

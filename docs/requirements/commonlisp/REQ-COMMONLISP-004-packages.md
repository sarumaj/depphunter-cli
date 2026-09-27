---
id: REQ-COMMONLISP-004
title: Packages to files and systems
scope: commonlisp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A package a file names (a `defpackage` option, `in-package`, a qualified
symbol) **shall** resolve to the file of the repository defining it (by
name or `:nicknames`; the nearest of several, none when it is the file
itself), else to the built-ins (REQ-COMMONLISP-008), else to a
package-inferred system's file (`acme/util`), else to the system the
file's systems declare that the package belongs to
(`asdf:register-system-packages`, the curated table, the package's own
name, a name folding alike such as `json` and `cl-json`, a declared name
the package's dotted or slashed prefix spells). The declared systems are
the `:depends-on` of the systems whose components include the file (or of
the package-inferred system whose directory holds it, else of the nearest
`.asd` files above it, else of all), followed through the repository's
own systems, and the projects the governing `qlfile`, lock and
`ocicl.csv` list. A `defpackage` option naming a package nothing declares
**shall** be an unresolved Quicklisp project named by the curated table or
the package; in a package-inferred system, where a package is the system
of its name, it **shall** resolve like a `:depends-on`. `in-package` and
a qualified symbol of a package nothing declares **shall** be dropped.

## Rationale

Common Lisp packages are not tied to files or systems by name; the
repository's `defpackage` forms and the systems' declarations are what
connect them.

## Acceptance criteria

1. `in-package shop` resolves to `src/package.lisp`, `shop.core:` to
   `src/core.lisp`, `use shop` in `t/main.lisp` to `src/package.lisp`.
2. `import-from json` is cl-json, `ppcre:` cl-ppcre, `a:` (a local nickname
   defined in another file) alexandria; `use undeclared-pkg` and
   `import-from 5am` in `src/core.lisp` are unresolved; `mystery:` is
   dropped.

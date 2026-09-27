---
id: REQ-SWIFT-013
uuid: dc217df7-ffc4-4f72-83ac-14a9fcd5bca7
title: Swift read without running SwiftPM
scope: swift
type: limitation
priority: must
status: implemented
verification:
  - manual
---

## Statement

The plugin **shall not** run SwiftPM, Xcode or the Swift compiler: a
manifest's computed values and conditionals are not evaluated, an Xcode
project's targets and their source files are not read (module directories
are guessed by name), `Package.resolved` gives no package-to-package edges
without a checkout, a module no product, table entry or name match
attributes is unresolved, and type references are matched by name only (no
overloads, nested types or generics). The scanner (REQ-SWIFT-014) does not
type-check: a generic type in an expression with a single argument before a
call or closure (`Box<Int>(x)`, `Stream<T> { }`) and a type after an
arithmetic or prefix operator (`x + Int(y)`, `try! Foo()`) are not recorded
as type uses, as the grammar it replaced read them, macros are not expanded,
and a `/` that could divide is not read as a regex literal. No package index
is asked with `--online`.

## Rationale

depphunter reads repositories statically and never executes their code;
Swift packages have no common index (a registry is optional and rare).

## Acceptance criteria

1. A module imported only as a product another package re-exports (Vapor's
   `import UnixSignals`) is an unresolved package.

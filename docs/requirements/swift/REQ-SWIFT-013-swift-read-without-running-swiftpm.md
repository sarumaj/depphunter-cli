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
attributes is unresolved, type references are matched by name only (no
overloads, nested types or generics), and files the grammar cannot fully
parse (about one in ten in real projects: new syntax, complex generic
operator declarations) yield what the partial tree holds, at up to the
3-second parse bound. No package index is asked with `--online`.

## Rationale

depphunter reads repositories statically and never executes their code;
Swift packages have no common index (a registry is optional and rare).

## Acceptance criteria

1. A module imported only as a product another package re-exports (Vapor's
   `import UnixSignals`) is an unresolved package.

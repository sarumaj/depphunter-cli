---
id: REQ-SWIFT-011
title: Types used across a module's files
scope: swift
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** record the capitalized type names a file uses (type
annotations, inheritance, extensions, constructor calls, static member
access) that it does not declare itself, and **shall** resolve each to a file
of the file's own module that declares a non-private type of that name at
the start of a line, else to such a file in a project module the file
imports; any other name is dropped. A file's module is the target directory
around it, else the top-level directory under its project (an Xcode
project's targets are not read).

## Rationale

Files of one Swift module see each other's declarations without imports;
without these edges a module's files would stand unconnected on the map.

## Acceptance criteria

1. `Server.swift` has edges to `Model.swift` for Service, Job and Status
   and to `Sources/Util/Helper.swift` for Helper through `import Util`; a
   test file reaches `Server.swift` through `@testable import Demo`.

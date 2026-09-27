---
id: REQ-SWIFT-003
uuid: c4460808-ef1d-4b53-b19a-f6bb975de643
title: Swift definitions extracted
scope: swift
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** extract classes and actors (kind `class`), structs,
enums and typealiases (kind `type`), protocols (kind `interface`) and
extensions (kind `extension`, named after the extended type) named with the
types around them (`Job.Step`), functions and methods (`Owner.name`, protocol
requirements included), initializers as `Owner.init`, and properties
(`Owner.name` of kind `property`, top-level ones of kind `var`). A
declaration inside a function, closure or accessor body is not extracted.

## Rationale

The symbols are what the map draws inside a file and what `--lsp` asks
references for.

## Acceptance criteria

1. `Sources/Demo/Server.swift` yields `Server` (class), `Server.init`,
   `Server.start` (methods), `Server.logger` (property), the extension
   `Server@33`, `makeServer` (func) and `defaultPort` (var), and nothing for
   the local function in `start`.

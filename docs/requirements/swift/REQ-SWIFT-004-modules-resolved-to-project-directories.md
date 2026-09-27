---
id: REQ-SWIFT-004
uuid: f6779530-a41a-4e1b-9541-ef041ed8b596
title: Modules resolved to project directories
scope: swift
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** resolve an imported module built by the project to
the directory of its sources: a target of any `Package.swift` in the
repository (`.target`, `.executableTarget`, `.testTarget`, `.macro`,
`.plugin`, `.systemLibrary`, `.binaryTarget`) by its C99 module name (target
`demo-cli` is module `demo_cli`), at its `path:` or SwiftPM's default
directory (`Sources/<name>`, `Source`, `src` or `srcs`; `Tests/<name>` for a
test target; `Plugins/<name>` for a plugin), nearest to the importing file
first; else, after the toolchain's and the declared packages' modules, a
directory named after the module that holds Swift sources (an Xcode
project's framework), outside `Pods`, `Carthage` and checkouts. An import of
the file's own module is dropped.

## Rationale

A Swift module is a directory of sources; the map already shows directories,
so the edge goes to the directory as a Go package import does.

## Acceptance criteria

1. `@testable import Demo` in `Tests/DemoTests` resolves to `Sources/Demo`;
   `import CoreKit` in the Xcode app resolves to `App/CoreKit`.

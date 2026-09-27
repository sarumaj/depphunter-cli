---
id: REQ-SWIFT-002
uuid: 8da78e86-6553-4361-b6cc-77267436ead0
title: Import declarations read
scope: swift
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read every `import` declaration with the vendored
tree-sitter Swift grammar, with its attributes (`@testable`, `@_exported`,
`@_implementationOnly`, `@preconcurrency`, `@_spi(...)`) and access modifiers
(`public import`) shown but not changing the module, a declaration kind
(`import struct Collections.Deque`) singling out a declaration, and the first
component of the path as the module (`import os.log` imports `os`). Before
parsing, `#if`/`#elseif`/`#else`/`#endif` lines are blanked so that every
branch is read, a freestanding macro starting a line (`#expect(...)`) is read
as a call, and Swift 6.2's `unsafe` expression marker is dropped; lines and
columns are kept.

## Rationale

The grammar does not know conditional-compilation blocks where declarations
are expected, and its error recovery dropped the imports they guard. Each
branch is code the project builds on some platform, so each import is a
dependency. Parse errors also cost the parser a slow retry ladder.

## Acceptance criteria

1. `@testable import Demo`, `import struct Collections.Deque` and both
   `import UIKit` and `import AppKit` of one `#if canImport(UIKit)` block are
   recorded with the modules Demo, Collections, UIKit and AppKit.

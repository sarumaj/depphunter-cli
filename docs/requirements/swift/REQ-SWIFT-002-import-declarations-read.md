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

The plugin **shall** read every `import` declaration with its scanner
(REQ-SWIFT-014), with its attributes (`@testable`, `@_exported`,
`@_implementationOnly`, `@preconcurrency`, `@_spi(...)`) and access modifiers
(`public import`) shown but not changing the module, a declaration kind
(`import struct Collections.Deque`) singling out a declaration, and the first
component of the path as the module (`import os.log` imports `os`). Every
branch of an `#if`/`#elseif`/`#else` block is read; when the branches open or
close braces differently (a declaration whose header differs by platform),
only the first branch is.

## Rationale

Each branch of a conditional-compilation block is code the project builds on
some platform, so each import is a dependency. Reading one branch of a block
whose branches leave the braces unbalanced keeps the declarations after it at
the right nesting.

## Acceptance criteria

1. `@testable import Demo`, `import struct Collections.Deque` and both
   `import UIKit` and `import AppKit` of one `#if canImport(UIKit)` block are
   recorded with the modules Demo, Collections, UIKit and AppKit.
2. Of an `#if os(iOS)` block whose branches open `extension Box: UIView {`
   and `extension Box: NSView {`, only the first branch is read, and the
   declarations after the block stay in the extension.

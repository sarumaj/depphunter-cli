---
id: REQ-SWIFT-001
uuid: a32f1b14-3a3d-4f54-9c3a-3f660e3a8321
title: Swift files claimed
scope: swift
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** claim every `.swift` file that is not binary,
`Package.swift` and its versioned variants (`Package@swift-5.9.swift`)
included, except files under a `.build` directory (SwiftPM's build output and
its checkouts of dependencies).

## Rationale

Swift sources and package manifests share one extension; a checkout under
`.build` is another package's code, not the project's.

## Acceptance criteria

1. `Sources/App/App.swift`, `Package.swift` and `Package@swift-5.9.swift` are
   claimed; `.build/checkouts/swift-nio/Package.swift` is not.

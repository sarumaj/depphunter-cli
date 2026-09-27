---
id: REQ-SWIFT-007
title: Modules mapped to packages
scope: swift
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** map an imported module to the package it comes from:
the package a target of the file's projects takes it from as a product
(`.product(name:package:)`, an Xcode product dependency), else the package a
curated table names for it (Logging is swift-log, NIOCore swift-nio,
ArgumentParser swift-argument-parser) when the project declares or pins that
package, else the declared or pinned package whose identity (a leading
`swift-` or trailing `.swift` ignored, a registry id's name) equals the
module's name or is the longest one starting it (NIOHTTP2 is
swift-nio-http2), else the pod or Carthage dependency a Podfile, podspec,
Cartfile or their locks over the file declare (REQ-OBJC-012). A module that
matches nothing is an unresolved package named after the table's URL, else
after the module.

## Rationale

Swift imports modules, but manifests declare packages; the product lists
are exact, the table and the name match cover modules no product names.

## Acceptance criteria

1. `import NIOCore` resolves to `github.com/apple/swift-nio` by product,
   `import Collections` to swift-collections by name, and an undeclared
   `import SnapKit` to an unresolved `github.com/SnapKit/SnapKit`.

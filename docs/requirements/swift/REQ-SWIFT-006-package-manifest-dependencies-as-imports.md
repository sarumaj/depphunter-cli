---
id: REQ-SWIFT-006
uuid: 2721c5d9-6e36-4f1c-9da4-ef3065dd4582
title: Package manifest dependencies as imports
scope: swift
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read a manifest's `.package(url:)`, `.package(path:)`
and `.package(id:)` lines (comments ignored) as imports of the packages they
declare, and **shall** name a package at a URL by the URL without scheme,
user, port and `.git`, host in lower case (`git@github.com:apple/swift-nio.git`
is `github.com/apple/swift-nio`), a registry package by its id, and resolve a
path dependency to its directory. The ecosystem id is `swiftpm`; OSV is asked
in its `SwiftURL` ecosystem, Trivy's `swift` type maps to it, and `swiftpm:`
is a private-pattern prefix.

## Rationale

The URL is a Swift package's only global name: OSV and Trivy name packages
by it, and two manifests spelling the URL differently still name one
package.

## Acceptance criteria

1. `.package(url: "https://github.com/apple/swift-nio.git", from: "2.60.0")`
   is an edge to `github.com/apple/swift-nio`; `.package(path:
   "Local/LocalKit")` is an edge to that directory.

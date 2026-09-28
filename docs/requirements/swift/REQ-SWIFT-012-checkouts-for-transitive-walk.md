---
id: REQ-SWIFT-012
title: Checkouts for the transitive walk
scope: swift
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

For `--resolve-depth`, the plugin **shall** read the dependencies of a
package from the `Package.swift` SwiftPM checked out under
`.build/checkouts/<identity>/` beside the project's manifest, each pinned by
the project's `Package.resolved` where it has one; without a checkout a
package has no known dependencies, and a `Package.resolved` that pins
packages beside no `.build/checkouts` **shall** be noted as flat in the
resolution report ([REQ-TRC-017](../trc/REQ-TRC-017-resolver-notes.md)).

## Rationale

Package.resolved is flat: it says which packages are installed but not
which needs which.

## Acceptance criteria

1. swift-nio's checkout yields swift-atomics 1.2.0 and swift-collections
   1.1.0 (pinned) and swift-docc-plugin (floating).

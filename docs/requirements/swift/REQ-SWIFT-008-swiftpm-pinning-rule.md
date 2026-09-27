---
id: REQ-SWIFT-008
uuid: bc1068f9-09d8-4f7b-8a0e-66f16a13edc8
title: SwiftPM pinning rule
scope: swift
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** treat a package pinned by `Package.resolved` as pinned
at the version it records (or the revision, for a branch or revision pin),
with the manifest's requirement as the requested one, and without a pin
**shall** treat `exact:`, a bare version, `.exact(...)` and `revision:` as
pinned, `from:`, `.upToNextMajor`, `.upToNextMinor` and ranges as floating
(shown as `2.60.0..<3.0.0`), and `branch:` as floating without a version.

## Rationale

SwiftPM resolves every requirement but an exact version or a revision anew
unless Package.resolved says otherwise.

## Acceptance criteria

1. swift-nio `from: "2.60.0"` pinned to 2.64.0 is pinned at 2.64.0 with
   requested `2.60.0..<3.0.0`; `exact: "1.3.0"` pins without a lock; the
   registry package `from: "1.0.0"` floats as `1.0.0..<2.0.0`.

---
id: REQ-SWIFT-009
title: Package.resolved read
scope: swift
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read `Package.resolved` in format versions 1
(`object.pins` with `package` and `repositoryURL`), 2 and 3 (`pins` with
`identity`, `kind` and `location`), beside a `Package.swift` (read from disk
when the scan left it out) and in an Xcode project's or workspace's
`xcshareddata/swiftpm/`, and **shall** count the packages it pins as
declared. One read from disk because the scan left it out **shall** be noted
in the resolution report
([REQ-TRC-017](../trc/REQ-TRC-017-resolver-notes.md)).

## Rationale

Package.resolved names every package installed, direct or not, at its exact
version; libraries often do not commit it, apps do.

## Acceptance criteria

1. The v3 file pins swift-nio at 2.64.0, the Xcode workspace's v1 file pins
   Alamofire at 5.8.1, and a registry pin keeps its id.

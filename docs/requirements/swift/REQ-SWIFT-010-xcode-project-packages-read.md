---
id: REQ-SWIFT-010
title: Xcode project packages read
scope: swift
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read an Xcode project's `project.pbxproj` for its
Swift package references (`XCRemoteSwiftPackageReference` with its
repository URL and requirement kind, `XCLocalSwiftPackageReference` with its
relative path) and the products its targets take from them
(`XCSwiftPackageProductDependency`), as the declarations of the project in
the directory holding the `.xcodeproj`.

## Rationale

An app built with Xcode has no Package.swift: its packages are only in the
project file.

## Acceptance criteria

1. `import Alamofire` in the app resolves to `github.com/Alamofire/Alamofire`
   through its product dependency, requested `5.8.0..<6.0.0`, and
   Kingfisher's `exactVersion` pins 7.10.0.

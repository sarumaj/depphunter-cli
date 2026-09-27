---
id: REQ-SWIFT-005
title: Swift toolchain and Apple SDK islands
scope: swift
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** resolve the modules every Swift toolchain ships (the
standard library and its satellites, Foundation and its parts, Dispatch,
XCTest, swift-testing's `Testing`, the manifest APIs, and the C libraries of
non-Apple platforms such as Glibc, Musl and WinSDK) to the hidden `swift-std`
island, and the frameworks of Apple's SDKs and the Darwin modules (UIKit,
AppKit, SwiftUI, Combine, CoreData, Darwin, os and a curated list of others)
to the hidden `apple-sdk` island.

## Rationale

Both come with the compiler or with Xcode, not from a package; they are split
so that a Linux server's map shows no Apple SDK and an app's map shows which
frameworks it uses.

## Acceptance criteria

1. `import Foundation` and `import XCTest` go to `swift-std`; `import
   SwiftUI` and `import Combine` go to `apple-sdk`.

## Notes

The objc plugin shares the framework list and the island (REQ-OBJC-005);
older frameworks Objective-C still imports (AddressBook, AssetsLibrary,
Twitter...) and Darwin modules (`notify`, `zlib`, `Compression`, `XPC`) were
added to it.

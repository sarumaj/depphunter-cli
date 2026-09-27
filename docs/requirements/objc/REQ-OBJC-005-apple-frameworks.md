---
id: REQ-OBJC-005
uuid: 9aff778f-2964-4967-820c-980986ad7391
title: Apple frameworks in the apple-sdk island
scope: objc
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

An include whose first directory is an Apple SDK framework (`<UIKit/UIKit.h>`,
`<Foundation/Foundation.h>`, `<CoreData/...>`, case ignored) and an `@import`
of one **shall** resolve to the hidden `apple-sdk` island the Swift plugin
declares, by the same framework list (REQ-SWIFT-005) plus Foundation, XCTest
and Dispatch; `<objc/...>` **shall** be the `ObjectiveC` runtime, and Darwin
headers the C/C++ plugin does not list (`<pthread/...>`, `<AssertMacros.h>`,
`<compression.h>`) the Darwin framework they belong to.

## Rationale

A Swift and an Objective-C file of one app using UIKit must reach one node, and
Xcode's case-insensitive file system accepts `<Appkit/Appkit.h>`.

## Acceptance criteria

1. `<UIKit/UIKit.h>` and `<Appkit/Appkit.h>` go to `apple-sdk` `UIKit` and
   `AppKit`, `<objc/runtime.h>` to `ObjectiveC`, `@import
   CoreData.NSManagedObject;` to `CoreData`.

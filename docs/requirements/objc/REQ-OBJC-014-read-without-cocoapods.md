---
id: REQ-OBJC-014
title: Objective-C read without Xcode or CocoaPods
scope: objc
type: limitation
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin reads sources and manifests as text, without Xcode, CocoaPods or
Carthage: header search paths from an Xcode project or `.xcconfig` files are
not read, a pod is matched to a header or module by name (a pod whose module
is named unlike it and is not in the table is unresolved), a Swift module no
manifest declares stays an unresolved SwiftPM package, private spec
repositories are never fetched, and no vulnerability database covers
CocoaPods or Carthage.

## Rationale

Running `pod install` or `xcodebuild` needs macOS and the network; the files
say most of what they would.

## Acceptance criteria

1. A framework header no manifest names, `<Unknown/Unknown.h>`, is an
   unresolved pod under a Podfile; a Swift `import Kingfisher` no manifest
   declares is an unresolved SwiftPM package.

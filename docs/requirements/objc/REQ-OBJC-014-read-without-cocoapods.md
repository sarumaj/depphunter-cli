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
Carthage: an Xcode project's header search paths (REQ-OBJC-015) apply to every
file below the project, whatever its target, build configuration or SDK, and a
setting Xcode defines itself other than the project directory (such as
`$(BUILT_PRODUCTS_DIR)`) is not expanded; a pod is matched to a header or
module by name (a pod whose module
is named unlike it and is not in the table is unresolved), a Swift module no
manifest declares stays an unresolved SwiftPM package, a private spec
repository is read only from its clone on this machine (REQ-SUP-074), never
fetched, and no vulnerability database covers
CocoaPods or Carthage by name and version: only a pod or a Carthage
dependency checked out at a full git commit on a public forge is asked about,
by that commit (REQ-FND-026).

## Rationale

Running `pod install` or `xcodebuild` needs macOS and the network; the files
say most of what they would.

## Acceptance criteria

1. A framework header no manifest names, `<Unknown/Unknown.h>`, is an
   unresolved pod under a Podfile; a Swift `import Kingfisher` no manifest
   declares is an unresolved SwiftPM package.

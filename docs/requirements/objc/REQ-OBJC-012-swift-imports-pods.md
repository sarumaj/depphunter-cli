---
id: REQ-OBJC-012
title: Swift imports of pods
scope: objc
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Swift plugin **shall** resolve an imported module that is neither the
project's, the toolchain's, Apple's nor a declared SwiftPM package's to the pod
or Carthage dependency the manifests over the file declare, by the rules of
REQ-OBJC-006.

## Rationale

Apps built with CocoaPods import pods from Swift as modules (`import
Alamofire`); without this their imports were unresolved SwiftPM packages.

## Acceptance criteria

1. In the fixture, `import Alamofire`, `import GRDB`, `import Lottie` and
   `import FirebaseCore` resolve to their pods; `import Kingfisher`, which no
   manifest declares, stays an unresolved SwiftPM package.

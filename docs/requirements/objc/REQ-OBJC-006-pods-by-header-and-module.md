---
id: REQ-OBJC-006
uuid: 44992036-bca7-47bd-9b08-42eb3e9c489d
title: Framework headers and modules attributed to pods
scope: objc
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A framework header `<Name/Header.h>`, an `@import Name;` and a bare header
`"Name.h"` that is not a project file **shall** resolve to the pod or Carthage
dependency named `Name` among those the manifests over the file declare or
lock (nearest directory first; a file under none sees all, shallowest first):
by the pod's own name, its spelling folded to letters and digits, without a
`lib` prefix or a platform suffix (`swift`, `ios`, `objc`), by a curated table
of modules named otherwise (GRDB is `GRDB.swift`'s, `FirebaseCore` may be the
`Firebase` pod's), or by a Carthage repository's name. A pod the repository
builds (a path pod, a podspec in the repository, also by `module_name`)
**shall** resolve to the header of that name in its directory, else to its
podspec. A project file found under `Pods/<Name>/`, `Pods/Headers/*/<Name>/`,
`Carthage/Checkouts/<repo>/` or `Carthage/Build/.../<Name>.framework/`
**shall** be attributed to that dependency. A framework header matching none
of these resolves to a project directory of that name, else, under a
CocoaPods manifest, to an unresolved pod, else to `c-external`.

## Rationale

CocoaPods puts a pod's headers on the header search path under the pod's
module name, and apps often commit `Pods/`: an edge into it is a dependency,
not the project's own code.

## Acceptance criteria

1. `<AFNetworking/AFNetworking.h>`, `<SDWebImage/UIImageView+WebCache.h>`,
   `"Masonry.h"` and `@import Firebase;` resolve to the pods; `<GRDB/GRDB.h>`
   to `GRDB.swift`; `"AFURLSessionManager.h"` found in `Pods/AFNetworking/` to
   `AFNetworking`; `<LocalKit/LKThing.h>` to `LocalKit/Sources/LKThing.h`;
   `<Unknown/Unknown.h>` to an unresolved pod.
2. In a Carthage project, `<Mantle/Mantle.h>` and `@import Analytics;`
   resolve to the Carthage dependencies named so, and a header of a built
   framework under `Carthage/Build` to its dependency.

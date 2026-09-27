---
id: REQ-OBJC-008
title: Podfile.lock versions and dependency graph
scope: objc
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The resolver **shall** read the `Podfile.lock` beside each Podfile (from disk
when the scan left it out): each pod's version and the pods its specs depend on
(`PODS`, subspecs folded into their pod), the git or path source of a pod
(`EXTERNAL SOURCES`) and what was checked out (`CHECKOUT OPTIONS`), and
**shall** answer the transitive walk (REQ-SUP-011) from that graph, pinned by
the same lock.

## Rationale

Podfile.lock is complete: it lists every installed pod with its dependencies,
so an app's whole pod graph is known offline.

## Acceptance criteria

1. `Firebase` depends on `FirebaseAnalytics` and `FirebaseCore` (10.0.0,
   pinned), `FirebaseAnalytics` on `FirebaseCore` and `GoogleUtilities`, and
   `LocalKit` on `AFNetworking`.

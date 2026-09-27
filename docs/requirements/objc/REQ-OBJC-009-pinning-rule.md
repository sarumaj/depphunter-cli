---
id: REQ-OBJC-009
title: CocoaPods pinning rule
scope: objc
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A pod **shall** be pinned to the version its nearest `Podfile.lock` records
(with the Podfile's requirement as `requested` when that differs); without a
lock, one exact version (`'1.2.3'`, `'= 1.2.3'`) **shall** pin and be shown
bare, a range (`~>`, `>=`, several requirements) **shall** float, and no
requirement **shall** float. A git pod **shall** be pinned by the commit
recorded (`:commit`, or the lock's checkout commit), be shown with a `:tag`
but neither pinned nor floating (a tag can be moved), and float on a
`:branch` or no reference; its origin is the git URL, so no index is asked.

## Rationale

The same rule as Terraform's and CMake's git sources (REQ-CI-011): only a
commit is immutable.

## Acceptance criteria

1. `AFNetworking` `~> 4.0` locked at 4.0.1 is pinned with `requested` `~>
   4.0`; `Masonry` `= 1.1.0` is pinned at 1.1.0 without `requested`; the
   branch pod `AcmeKit` is pinned by the lock's checkout commit; `Alamofire` by
   tag shows 5.8.1, neither pinned nor floating.
2. Without a lock, `PromiseKit` `>= 6.0, < 7.0` floats and `CocoaLumberjack`
   `= 3.8.0` pins at 3.8.0.

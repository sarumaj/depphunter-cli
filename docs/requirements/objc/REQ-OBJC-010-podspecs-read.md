---
id: REQ-OBJC-010
uuid: a0f50f3d-6d15-4446-90d4-7190d8d08a57
title: Podspecs read as library manifests
scope: objc
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read a Ruby `*.podspec` (its `name`, `module_name` and
every `dependency` call, subspecs' and test specs' included) and a
`*.podspec.json` (`name`, `module_name`, `dependencies` of the spec, its
subspecs and test specs), each dependency an import of its pod; a dependency on
the podspec's own subspec is dropped.

## Rationale

A library published as a pod declares its dependencies only in its podspec.

## Acceptance criteria

1. `ShopKit.podspec` imports `AFNetworking`, `Mantle` and `PromiseKit`, not
   `ShopKit/Core`; `@import Shop;` (its `module_name`) resolves to the
   podspec; `Toolkit.podspec.json` imports `SDWebImage` and
   `CocoaLumberjack`.

---
id: REQ-DART-006
title: pubspec dependencies read and shown as imports
scope: dart
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read `pubspec.yaml`'s `dependencies`,
`dev_dependencies` and `dependency_overrides` - a bare constraint, `hosted`
(with a server), `git` (with `ref`), `path` and `sdk` - and **shall** record
each as an import of the package it declares from the pubspec, and each pub
workspace member as an import of the member's pubspec.

## Rationale

A Flutter plugin used only by native code, or a dependency nothing imports
yet, has no Dart import to show it otherwise.

## Acceptance criteria

1. A path dependency's import resolves to the dependency's `pubspec.yaml`;
   `flutter: {sdk: flutter}` to the Flutter SDK island.
2. `workspace: [packages/shop_core]` connects the root pubspec to
   `packages/shop_core/pubspec.yaml`.

---
id: REQ-DART-005
uuid: 96746f6b-ed95-4587-a45f-27c80def9d3b
title: Dart SDK libraries and Flutter SDK islands
scope: dart
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** resolve `dart:` URIs to a hidden standard-library island
`dart-std` named by the URI (`dart:async`), and the packages Flutter's SDK
ships (`flutter`, `flutter_test`, `flutter_localizations`, `flutter_driver`,
`flutter_web_plugins`, `integration_test`, `sky_engine`) or that a pubspec or
lock takes from an SDK (`sdk: flutter`) to a hidden island `flutter-sdk`.

## Rationale

SDK libraries come with the toolchain and no index serves them.

## Acceptance criteria

1. `package:flutter/material.dart` resolves to `flutter` in `flutter-sdk`;
   `import "dart:io"` to `dart:io` in `dart-std`.

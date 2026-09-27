---
id: REQ-DART-001
title: Dart files and pubspecs claimed
scope: dart
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Dart plugin **shall** analyze `.dart` files and files named
`pubspec.yaml`, and **shall not** analyze anything under a `.dart_tool`
directory.

## Rationale

Dart and Flutter projects are Dart sources and pub packages; `.dart_tool`
holds what pub and the analyzer generate.

## Acceptance criteria

1. `lib/main.dart` and `app/pubspec.yaml` are claimed; `pubspec.lock` and
   `.dart_tool/flutter_gen/gen_l10n/l.dart` are not.

---
id: REQ-DART-004
title: package: URIs resolved to project packages
scope: dart
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** resolve a relative URI against the importing file, and
a `package:<name>/<path>` URI to `<package>/lib/<path>` (else that `lib`
directory, else the package) when `<name>` is the package of the nearest
`pubspec.yaml` above the file, a path dependency or override of it (including
`pubspec_overrides.yaml`), a path package of its `pubspec.lock`, a member of
the same pub workspace (`workspace:` in the root pubspec, `resolution:
workspace` in the member) or a package of the same melos repository
(`melos.yaml` `packages:` globs less its `ignore:`); any other package is
external, even when the repository holds a package of that name.

## Rationale

pub resolves a package from the path a pubspec or lock names, and a
workspace or `melos bootstrap` links its packages to each other whatever
version they ask for; anything else is downloaded.

## Acceptance criteria

1. In a pub workspace, a member's `shop_utils: ^1.0.0` import resolves to the
   sibling member's `lib/shop_utils.dart`.
2. A melos package's hosted constraint on another melos package resolves
   locally; a package matched by `ignore:` takes it from pub.

## Notes

A relative URI to a file that is not in the repository (a generated
`main.g.dart`) and Flutter's synthetic `package:flutter_gen` are dropped.

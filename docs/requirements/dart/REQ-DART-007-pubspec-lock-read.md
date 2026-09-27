---
id: REQ-DART-007
title: pubspec.lock read
scope: dart
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read `pubspec.lock` beside a pubspec (from disk when the
scan left it out), or the workspace root's for a workspace member, and resolve
a package it records: hosted at its version, git by its URL and resolved
commit, path to the directory, sdk to the Flutter SDK island, with the
pubspec's constraint as the requested version when it differs.

## Rationale

The lock is what pub installed; a library usually does not commit it.

## Acceptance criteria

1. `http: ^1.2.0` locked at 1.2.1 resolves to http 1.2.1, pinned, requested as
   `^1.2.0`; a package only the lock records is pinned at its version.

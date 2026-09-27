---
id: REQ-SUP-046
title: pub dependencies from the package API
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The index client **shall** read a pub package's dependencies from its
server's package API (`<index>/api/packages/<name>`, `https://pub.dev` unless
configured otherwise, `Accept: application/vnd.pub.v2+json`): the pubspec of
the version asked for, or else the latest, its `dependencies` with their
constraints, without `dev_dependencies` and SDK packages.

## Rationale

pubspec.lock is flat and a library commits none, so what a package depends
on is only on its server; every pub server implements this API.

## Acceptance criteria

1. Against a stub server, http 1.1.0 depends on `async` `^2.5.0` and
   `http_parser` 4.0.2 (pinned).
2. A constraint names no version, so the latest version's dependencies are
   returned, without `flutter` (an SDK package).

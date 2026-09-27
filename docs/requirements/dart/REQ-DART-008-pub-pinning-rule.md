---
id: REQ-DART-008
title: pub pinning rule
scope: dart
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** treat a package pinned by `pubspec.lock` (a git package
only when its resolved reference is a commit) as pinned, and without the lock
**shall** treat a bare version (`1.2.3`, `1.2.3+4`) and a git `ref` that is a
full commit as pinned, and a caret, a range, `any`, no constraint and a git
branch or tag as floating, `any` and no constraint without a version.

## Rationale

In pub a bare version allows exactly that version; every other constraint
lets the next `pub get` choose.

## Acceptance criteria

1. `provider: 6.1.2` pins without a lock; `^1.2.0`, `>=1.0.0 <2.0.0` and `any`
   float; `ref: main` floats.

## Notes

OSV is asked about pinned pub packages in its `Pub` ecosystem.

---
id: REQ-DART-009
uuid: 2fe8c4c1-a007-4715-bd26-6469f87002a1
title: Dart read without running pub
scope: dart
type: limitation
priority: must
status: implemented
verification:
  - manual
---

## Statement

The plugin **shall not** run pub, Flutter or the Dart analyzer: files that
are generated but not committed are not seen, `part of` by library name is not
resolved, a variable declaration list is named by its first variable,
unnamed extensions and their members are not symbols, and `pubspec.lock`,
which records no edges between packages, answers no `--resolve-depth` walk; the
pub.dev API does, with `--online`.

## Rationale

depphunter reads repositories statically and never executes their code.

## Acceptance criteria

1. With `--resolve-depth 1` and without `--online`, the resolution report says
   that the repository records no dependency graph for the Dart plugin.

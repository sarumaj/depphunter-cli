---
id: REQ-DART-009
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
pub.dev API does, with `--online`, and so does a private pub server this
machine holds a `dart pub token add` token for
([REQ-AUTH-027](../auth/REQ-AUTH-027-pub-tokens.md)).

## Rationale

depphunter reads repositories statically and never executes their code.

## Acceptance criteria

1. With `--resolve-depth 1` and without `--online`, the resolution report says
   that the repository records no dependency graph for the Dart plugin.

---
id: REQ-SUP-003
uuid: 6950e8f8-d871-4f07-928c-105dc1ab1dae
title: A lock file pins what it resolves
scope: sup
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M12
verification:
  - unit
  - integration
---

## Statement

The system **shall** treat an external package whose version a lock file the
repository carries resolves as pinned to that version, whatever range the
manifest declares.

## Rationale

A lock file is what fixes an installation; without it a range resolves to
whatever the index serves next.

## Acceptance criteria

1. A repository whose dependencies are all locked shows no floating packages.
2. Removing the lock file from that repository makes every package that the
   manifest declares with a range floating.

## Notes

The lock files read per ecosystem are specified by the plugin scopes; see the
lock file tests `TestLockfiles` (JavaScript) and the Python and Rust resolver
tests.

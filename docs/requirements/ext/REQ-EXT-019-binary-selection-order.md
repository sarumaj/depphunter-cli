---
id: REQ-EXT-019
uuid: 93e53f32-785d-4d7a-80d6-73082acd7bff
title: Which binary is launched
scope: ext
type: functional
priority: must
status: implemented
source:
  - README.md Install the extension
  - README.md Settings
verification:
  - extension
---

## Statement

The extension **shall** launch the binary named by `depphunter.path` when it is
set; otherwise the binary bundled in the extension's `bin` directory when
present; otherwise `depphunter` on `PATH`.

## Rationale

A released VSIX carries the binary of the same release, so extension and server
cannot diverge; an explicit setting is the only way to run another build and is
never second-guessed.

## Acceptance criteria

1. With a bundled binary and no setting, the bundled binary is chosen.
2. Without a bundled binary, `depphunter` is chosen.
3. A non-blank `depphunter.path` wins over both.

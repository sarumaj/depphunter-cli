---
id: REQ-EXT-025
uuid: 357117a5-2c7a-4a41-9fd2-fc395e2659a2
title: Open in editor uses this editor
scope: ext
type: functional
priority: should
status: implemented
source:
  - README.md Settings
verification:
  - extension
  - manual
---

## Statement

The extension **should** pass `--editor` with the command-line launcher of the
editor it runs in, and **shall** pass `depphunter.editorCommand` instead when it
is set.

## Rationale

What depphunter would detect is whatever is on the extension host's `PATH`,
which need not be this editor.

## Acceptance criteria

1. With `depphunter.editorCommand` set, `--editor` carries it verbatim.
2. Otherwise `--editor '<appRoot>/bin/<applicationName>' -g {file}:{line}` is
   passed when that launcher exists, and nothing when it does not.

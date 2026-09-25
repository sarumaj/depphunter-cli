---
id: REQ-EXT-017
uuid: ab5d5947-67f2-4e69-b02b-3fdbca1fa3a9
title: Server output in the output channel
scope: ext
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M18
  - README.md Use
  - README.md Settings
verification:
  - extension
---

## Statement

The extension **shall** write the command line it runs and the server's standard
output and standard error verbatim to the `depphunter` output channel, and
**shall** pass `--explain` when `depphunter.explain` is set, so that the
resolution digest is read there.

## Rationale

`--explain` writes its digest to the log, which is where the editor's output
channel reads it; **depphunter: Show the Server Log** shows it.

## Acceptance criteria

1. The channel shows `> <binary> <args>` followed by the server's output.
2. With `depphunter.explain` true the command line contains `--explain`.

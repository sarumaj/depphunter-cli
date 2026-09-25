---
id: REQ-CLI-009
uuid: 976fa25f-cb85-4e7e-804f-6fb748dae1be
title: Log written to stdout
scope: cli
type: interface
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §5
verification:
  - unit
---

## Statement

The command **shall** write its log to standard output, except as stated in
REQ-CLI-010.

## Rationale

What depphunter says can then be piped and read like any other output of a
command; the VS Code extension reads it from there.

## Acceptance criteria

1. While serving, the log (for example `analyzed ...` and `serving at ...`)
   appears on stdout.
2. With `--export <format> -o <file>`, the log appears on stdout.

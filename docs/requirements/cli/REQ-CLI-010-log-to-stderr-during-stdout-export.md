---
id: REQ-CLI-010
uuid: f0b77203-b55f-4c26-8d7d-6b8b252939c6
title: Log moves aside for an export on stdout
scope: cli
type: interface
priority: must
status: implemented
verification:
  - unit
  - integration
---

## Statement

When an export is written to standard output (`--export` without `--output`),
the command **shall** write its log to standard error, so that standard output
carries the export alone.

## Rationale

A log line in the middle of a JSON graph makes the document useless to whoever
redirected it.

## Acceptance criteria

1. `depphunter --explain --export json <dir>` writes to stdout a document that
   parses as JSON and contains nodes.
2. The log and the resolution report of that run appear on stderr.

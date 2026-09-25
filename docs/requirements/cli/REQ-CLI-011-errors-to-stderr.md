---
id: REQ-CLI-011
uuid: 886edaa4-b13e-461d-bcf6-f82a2c003259
title: Errors written to stderr
scope: cli
type: interface
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §5
verification:
  - integration
---

## Statement

The command **shall** write a fatal error to standard error, whatever the
destination of the log.

## Rationale

A failure is not part of the output anybody asked for.

## Acceptance criteria

1. A failing run with its stdout redirected to a file leaves the error message
   on the terminal and not in the file.

## Notes

No automated test checks the stream of the error.

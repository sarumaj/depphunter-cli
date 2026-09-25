---
id: REQ-CLI-008
uuid: ab22bcc0-c242-47ae-8a14-3707bbd52b89
title: Errors printed once without usage
scope: cli
type: interface
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M9
verification:
  - integration
---

## Statement

When the command fails, it **shall** print the error exactly once, prefixed with
the program name and a colon (`depphunter:`), **shall not** print the usage
text, and **shall** exit with status 1.

## Rationale

Errors concern the input, not the syntax; repeating them or burying them under
the usage text makes them harder to read.

## Acceptance criteria

1. `depphunter --theme neon .` prints one line `depphunter: invalid theme "neon"
   (want one of auto, light, dark)` and exits with status 1.
2. The output of a failed run contains no `Usage:` section.

## Notes

No automated test checks the single printing or the exit status.

---
id: REQ-CLI-005
uuid: 91ed1b8b-1107-4eea-95e8-7ed46252b674
title: Single-dash long flags rejected
scope: cli
type: interface
priority: must
status: implemented
verification:
  - integration
---

## Statement

The command **shall not** accept a long flag written with a single dash (for
example `-addr`); such an argument **shall** be parsed as short flags and fail
with an error when they are unknown.

## Rationale

POSIX flags reserve a single dash for short flags, so `-addr` is not a long
flag.

## Acceptance criteria

1. `depphunter -addr 127.0.0.1:0` exits with an error naming an unknown
   shorthand flag.

## Notes

It follows from pflag's parsing; `TestSingleDashLongFlagIsRejected` runs the
built binary with `-addr`.

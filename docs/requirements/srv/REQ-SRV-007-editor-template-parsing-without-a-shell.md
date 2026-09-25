---
id: REQ-SRV-007
uuid: 061366c3-5406-433a-bb4e-327253a7f38a
title: Editor template parsing without a shell
scope: srv
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** split an editor command template into arguments with POSIX
shell quoting rules (go-shellquote), replace `{file}` with the absolute file
path and `{line}` with the line number (at least 1) in each argument, and start
the program directly without a shell; a template that names no program or lacks
`{file}` **shall** be rejected.

## Rationale

A file name is substituted into an argument and never re-parsed, so it cannot
inject commands. The splitting is the established go-shellquote library's.

## Acceptance criteria

1. `"/opt/My Editor/bin/ed" --goto '{file}:{line}'` for `/src/a b.go` line 12
   yields the arguments `/opt/My Editor/bin/ed`, `--goto`, `/src/a b.go:12`.
2. A file name `/x; rm -rf ~` stays one argument.
3. A line of 0 is passed as 1.
4. A template with an unterminated quote or without `{file}` returns an error.

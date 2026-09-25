---
id: REQ-SEC-009
uuid: aa668897-de92-423b-b9c0-a67115cf1574
title: Editor run without a shell
scope: sec
type: non-functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M3
verification:
  - unit
---

## Statement

The system **shall** split the editor command template with POSIX shell quoting
rules and execute the resulting program directly, never through a shell,
substituting the file and line into the arguments so that a file name always
stays one argument.

## Rationale

File names come from the repository; passing them through a shell would let a
crafted name inject commands.

## Acceptance criteria

1. The template `code -g {file}` with file `/x; rm -rf ~` yields exactly three
   arguments, the last being `/x; rm -rf ~`.
2. A quoted program path with spaces stays one argument.
3. A template that is empty, lacks `{file}` or has unbalanced quotes is
   rejected.

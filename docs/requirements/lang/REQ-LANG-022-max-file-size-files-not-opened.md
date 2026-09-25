---
id: REQ-LANG-022
uuid: 59fd0ae8-70eb-4c75-b787-67fe33007f36
title: Files over the size limit not opened
scope: lang
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** list a file larger than `--max-file-size` without opening
it: its lines **shall not** be counted and no plugin **shall** read it.

## Rationale

The limit promises that such files are not read, which bounds the work spent on
large generated or data files.

## Acceptance criteria

1. A file over the limit appears as a file node with `bytes` and no `loc`.
2. No plugin receives the file.

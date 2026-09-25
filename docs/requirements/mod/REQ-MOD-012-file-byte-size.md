---
id: REQ-MOD-012
uuid: e225996f-c4df-4068-ab55-1f47b9cfa736
title: File size in bytes
scope: mod
type: interface
priority: must
status: implemented
verification:
  - unit
---

## Statement

A file node **shall** carry `bytes`, the size of the file in bytes as measured
when it was scanned, including for files whose lines were not counted.

## Rationale

A binary file or a file over `--max-file-size` has no counted lines; without its
size it cannot be told apart from an empty file.

## Acceptance criteria

1. A non-empty text file's node carries `bytes` equal to its size on disk.
2. A binary file's node carries `bytes` equal to its size on disk and no `loc`.
3. A file larger than `--max-file-size` carries `bytes` and no `loc`.

## Notes

How the UI draws and labels such files is scope `map`.

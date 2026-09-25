---
id: REQ-HIST-012
uuid: e8943437-7457-4f73-882d-0ae4841f2794
title: Districts colored by per-file means
scope: hist
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

In the Commits and Lines changed modes a collapsed directory (district)
**shall** be colored by the mean per file of its files, as in size mode, so
districts share the scale of files.

## Rationale

A sum would make every large directory the hottest thing on the map.

## Acceptance criteria

1. A district of ten files with one commit each is colored like a file with one
   commit.

## Notes

The Commits count of a directory is the number of distinct commits across its
files.

---
id: REQ-MAP-022
uuid: e859e08a-35d8-4aee-a7ae-6d2fd870d590
title: Expand and collapse one node
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §5
  - docs/REQUIREMENTS.md M1
verification:
  - ui
  - manual
---

## Statement

The UI **shall** expand or collapse a single directory or file on a double-click
on its box and on `Enter` for the selection, so that a directory opens into its
files and a file into its symbols; a symbol **shall** toggle its file.

## Rationale

What is shown, and in how much detail, is decided interactively without
re-running the tool.

## Acceptance criteria

1. Double-clicking a district expands it into a terrace with its children;
   double-clicking the terrace collapses it again.
2. Double-clicking a file with symbols shows its symbol blocks.
3. A file without symbols reports that it has nothing to open.

---
id: REQ-ADA-010
uuid: cc6e1c2f-fab3-40cd-adec-2f211a43fea2
title: Read by a scanner, not the grammar
scope: ada
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

Ada and GNAT project files **shall** be read by a lexer and block scanner
of their own, not a tree-sitter grammar, and any input **shall** be read
in time linear in its size without a panic.

## Rationale

The vendored Ada grammar parses well (9 of 479 files of alire, none of
gnatcoll-core's and AWS's sources, 107 of 1739 of the Ada Language
Server's, mostly deliberately broken test inputs) but takes 4.6 to 7.6 ms
per file (492 ms for one generated file); the scanner takes 0.09 to 0.19 ms
per file, and the resolver reads every source's unit header besides.

## Acceptance criteria

1. Every prefix of every fixture file and 200 KB runs of each construct
   are read without a panic within the time bound.

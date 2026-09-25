---
id: REQ-LANG-014
uuid: 0e6ee921-9c7c-45e6-986d-c31da7bc97a1
title: Unclaimed files listed
scope: lang
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §4
  - docs/REQUIREMENTS.md M1
verification:
  - unit
---

## Statement

Every scanned file, including a file no plugin claims, **shall** appear in the
graph as a file node with its language and line count, and a file no plugin
claims **shall** have no outgoing edges.

## Rationale

The map answers what a repository contains; documentation, configuration and
languages without a plugin are part of that answer.

## Acceptance criteria

1. A `README.md`, a `Makefile` and a `.css` file of the project appear as file
   nodes.
2. A file of a language without a plugin has no edges.

## Notes

Some files are claimed by plugins outside this scope (for example Markdown by
scope `md`).

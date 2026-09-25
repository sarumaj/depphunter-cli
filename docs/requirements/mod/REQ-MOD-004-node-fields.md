---
id: REQ-MOD-004
uuid: c58324a6-af74-4740-8832-f590503e94c5
title: Node fields
scope: mod
type: interface
priority: must
status: implemented
verification:
  - unit
  - inspection
---

## Statement

A node **shall** carry `id`, `kind` and `name`, and **shall** carry, where they
apply, `path` (directories and files), `parent` (the id of the containing node:
a directory for directories and files, a file for symbols, an ecosystem for
packages), `lang` and `loc` (files), and `symbolKind` and `line` (symbols).
Members that do not apply **shall** be omitted.

## Rationale

The hierarchy expressed through `parent` is what positions every element on the
map; the remaining members are what the tooltip, the side panel and the size
encodings show.

## Acceptance criteria

1. Every node except the root directory and ecosystem nodes has a `parent`
   naming a node of the same document.
2. A file node carries `path`, `lang` (when the language is known) and `loc`.
3. A symbol node carries `symbolKind` and a 1-based `line`.
4. A member that is empty or zero does not appear in the JSON.

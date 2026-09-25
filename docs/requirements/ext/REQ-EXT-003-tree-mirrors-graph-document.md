---
id: REQ-EXT-003
uuid: 5394e8ef-db5b-4c16-805f-7e2c5d3b8e61
title: Dependency tree mirrors the graph document
scope: ext
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M15
  - README.md The panel beside the code
verification:
  - extension
---

## Statement

The Dependencies view **shall** derive its rows from the graph document alone: a
directory **shall** expand into what it contains, a file into what it imports,
an island into its packages and a package into the packages it depends on.
Symbols **shall not** appear as rows.

## Rationale

The map answers what the code base looks like; a tree answers what is under a
node, which is the question asked with a keyboard while reading code. It is the
same graph either way, so the panel draws the one document the server serves and
nothing of its own. The editor has an outline of its own for symbols.

## Acceptance criteria

1. The roots include the repository's root directory, and it expands into its
   directories followed by its files.
2. A file that imports something expands into the imported nodes and never into
   a symbol.
3. An island expands into its packages and a package into its `depends` targets.
4. An import made from a symbol is shown under the file that holds the symbol.

---
id: REQ-LSP-006
uuid: 665ddf49-ff3c-4499-82f1-14856ad7ecbd
title: References cached by content and servers
scope: lsp
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M6
verification:
  - inspection
---

## Statement

The system **shall** cache complete reference results keyed by the graph nodes
and edges and by the commands of the installed language servers, reuse a cached
result for the same key, and **shall not** cache a partial result.

## Rationale

A repeated run on unchanged code must not wait for the servers again; a newly
installed server invalidates the result.

## Acceptance criteria

1. A second run on an unchanged project reads the references from the cache
   without starting a server.
2. A result whose time budget ran out is not cached.

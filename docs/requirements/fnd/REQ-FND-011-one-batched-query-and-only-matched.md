---
id: REQ-FND-011
uuid: a5a40805-d279-472d-990c-1a5c159f7cfb
title: One batched query and only matched advisories
scope: fnd
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** ask OSV about the whole dependency tree in batched
`querybatch` requests (up to 500 packages per request) and **shall** fetch only
the advisories the batch matched, each once however many packages it affects.

## Rationale

A request per package would be slow and would disclose the dependency list
piecemeal.

## Acceptance criteria

1. For a tree of fewer than 500 packages exactly one batch request is made.
2. An advisory affecting two packages is fetched once.

---
id: REQ-AUTH-013
uuid: 464c87bf-30ae-45db-bafa-3bcda69b7254
title: URL credentials are never recorded
scope: auth
type: constraint
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M16
verification:
  - unit
---

## Statement

The system **shall** remove any credential from an index URL before the URL is
recorded, so that it never reaches the graph, the side panel, the resolution
report or an export.

## Rationale

The index a package resolves from is written into every export, and the HTML
export is a file the documentation recommends sharing.

## Acceptance criteria

1. An index configured as `https://user:secret@mirror.corp/simple` is recorded
   as `https://mirror.corp/simple`, whether the machine or the repository names
   it.

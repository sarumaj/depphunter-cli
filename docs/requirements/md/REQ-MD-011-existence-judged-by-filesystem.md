---
id: REQ-MD-011
uuid: 735ff7e2-6956-49bc-93cc-6c65696c8969
title: Link targets judged by the filesystem
scope: md
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** decide whether a link target exists by the filesystem, not
by the map: a target that exists on disk but is ignored, excluded or otherwise
not on the map **shall not** be reported broken.

## Rationale

Repositories link to files they generate and files they ignore; neither is a
defect in the document.

## Acceptance criteria

1. A README linking to a generated file that exists on disk does not report it,
   while a link to a file absent from disk does.

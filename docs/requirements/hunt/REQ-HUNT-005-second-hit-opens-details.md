---
id: REQ-HUNT-005
uuid: 940618d0-fe80-4ea9-9dcf-4962c6c81fd0
title: Second hit on a tagged module shows details
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

When a use of the primary tool reaches a module that is already tagged, the
system **shall** open that module's details (dependencies, source) in the side
panel instead of tagging it again.

## Rationale

The hunt takes two shots: the first tags, the second reads. The details are
reached where a shooter's hands already are, without a separate key.

## Acceptance criteria

1. A second dart into a tagged building opens its details panel.
2. The modules-tagged counter does not change.

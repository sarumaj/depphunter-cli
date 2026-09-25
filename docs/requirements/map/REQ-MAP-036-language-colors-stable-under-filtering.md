---
id: REQ-MAP-036
uuid: 480a2ed8-7a9f-4e48-b9f4-3d7137db249a
title: Language colors stable under filtering
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M2
  - docs/REQUIREMENTS.md M3
verification:
  - ui
  - manual
---

## Statement

The system **shall** assign the categorical color slots to languages by lines of
code once per repository, and **shall not** change the color of any remaining
language when languages are hidden or the graph is updated while that language
still exists.

## Rationale

Color follows the language, not its rank; recoloring on a filter would break
everything the reader had learned.

## Acceptance criteria

1. Hiding the largest language leaves every other language's color unchanged.
2. A live update that adds a language keeps the existing languages' colors.

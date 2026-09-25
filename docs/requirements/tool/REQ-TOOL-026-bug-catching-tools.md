---
id: REQ-TOOL-026
uuid: 37fb720a-9844-43d1-8da0-e3a4ef0839db
title: Tools that catch bugs
scope: tool
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M20
verification:
  - ui
---

## Statement

The butterfly net, the bubble wand and the fire extinguisher **shall** catch
bugs by design, and the fishing rod (a hook) and the nail gun **shall** catch
them incidentally; the tracking dart **shall not** catch bugs.

## Rationale

Three deliberate ways to catch, each with its own trade, and two that follow
from what the tool physically does.

## Acceptance criteria

1. A nail catches a bug (M20 acceptance).
2. The net, the bubble wand and the extinguisher catch bugs.
3. The tracking dart passes through bugs.

## Notes

The camera also catches bugs (the photographed bug blinks out, REQ-HUNT-030),
which the design log's count of three plus two does not mention.

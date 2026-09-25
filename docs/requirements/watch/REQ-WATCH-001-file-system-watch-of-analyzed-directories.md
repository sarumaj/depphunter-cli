---
id: REQ-WATCH-001
uuid: 08336ab3-59bb-4d6b-b95b-fceee511d8ce
title: File-system watch of analyzed directories
scope: watch
type: functional
priority: must
status: implemented
verification:
  - unit
  - integration
---

## Statement

With `--watch`, the system **shall** watch, through fsnotify, only the
directories that hold analyzed files, and **shall** re-synchronize the watched
set with the graph after every analysis.

## Rationale

Ignored trees such as `node_modules`, build output and `.git` then cost nothing;
a new directory is picked up because its creation is an event in its watched
parent.

## Acceptance criteria

1. A change to a file in an analyzed directory triggers a re-analysis.
2. A change inside an ignored directory triggers none.
3. A directory created after start-up is watched after the re-analysis that
   lists its files.

## Notes

The git directory and the scanner reports are watched as well; see REQ-HIST-009
and REQ-FND-023.

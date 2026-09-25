---
id: REQ-HIST-001
uuid: 4a606e89-a831-44f0-b275-4ab9c148735d
title: Single git log pass over the analyzed directory
scope: hist
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M5
  - docs/REQUIREMENTS.md M6
verification:
  - integration
---

## Statement

The history reader (`internal/history`) **shall** read the git history in one
`git log --no-merges -M --relative --numstat -- .` pass over the commits
reachable from HEAD, restricted by the pathspec `-- .` to commits touching the
analyzed directory, with paths relative to it.

## Rationale

One pass is fast (ripgrep: 2,223 commits in 0.6 s). `--relative` alone still
lists commits outside the analyzed directory, so the pathspec restricts them.

## Acceptance criteria

1. Analyzing a subdirectory of a repository yields only commits that touched it,
   with paths relative to it.
2. Merge commits are not counted.

## Notes

M5 specified `--no-renames`; M6 replaced it with `-M` (see REQ-HIST-006).

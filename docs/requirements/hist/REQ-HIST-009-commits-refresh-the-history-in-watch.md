---
id: REQ-HIST-009
uuid: 15dcc2d5-97fc-452e-a5cf-4a938d1d1587
title: Commits refresh the history in watch mode
scope: hist
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M5
verification:
  - e2e
---

## Statement

In `--watch` mode the system **shall** watch the git directory and `refs/heads`,
and after a change that moved HEAD **shall** read the history again and announce
it.

## Rationale

A commit changes nothing in the work tree the map watches, but changes the
history overlay.

## Acceptance criteria

1. With the map open in `--watch` mode, committing a change updates the Commits
   overlay without a restart.

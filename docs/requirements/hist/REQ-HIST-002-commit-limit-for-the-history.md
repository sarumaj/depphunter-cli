---
id: REQ-HIST-002
uuid: 6147cf30-c060-4cd8-b3cc-e4bb05b9bdae
title: Commit limit for the history
scope: hist
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M5
verification:
  - integration
---

## Statement

The history reader **shall** read at most the newest `history_commits` commits
(default 10,000) and mark the history as truncated when more exist.

## Rationale

A bound keeps the read time and the payload predictable on very old
repositories.

## Acceptance criteria

1. With a limit of 2 in a repository of 3 commits, 2 commits are read and
   `truncated` is true.
2. The default of `history_commits` is 10000.

## Notes

The `--history-commits` flag and its validation belong to scope `cli`/`cfg`.

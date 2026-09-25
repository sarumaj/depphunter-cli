---
id: REQ-MAP-031
uuid: fdc61e25-e5d8-49c6-b9f6-31ddded6d635
title: Fuzzy search over files, symbols and packages
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §5
  - docs/REQUIREMENTS.md M2
  - docs/REQUIREMENTS.md M7
verification:
  - ui
  - manual
---

## Statement

The UI **shall** offer a fuzzy search, focused with `/`, over files,
directories, symbols and packages (fzf's algorithm through fzf-for-js), listing
the best 12 matches among visible nodes; choosing a match **shall** reveal and
select it.

## Rationale

Search is the fastest way to a known name on an unfamiliar map.

## Acceptance criteria

1. Typing part of a symbol's name lists that symbol with its file.
2. Arrow keys move through the results and `Enter` reveals the chosen node,
   expanding its ancestors.
3. Nodes hidden by filters are not listed.

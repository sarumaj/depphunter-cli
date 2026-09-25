---
id: REQ-EXT-029
uuid: c3a568ee-d6c9-4952-b135-05725685255f
title: Stopping a server closes its tab
scope: ext
type: functional
priority: must
status: implemented
source:
  - README.md Use
verification:
  - extension
---

## Statement

Stopping or restarting a server **shall** terminate the process and close the
map tab of that folder.

## Rationale

A tab holding a page on a port that no longer answers is worse than no tab; the
next invocation analyzes afresh.

## Acceptance criteria

1. After Stop, the tab is disposed and the address no longer answers.

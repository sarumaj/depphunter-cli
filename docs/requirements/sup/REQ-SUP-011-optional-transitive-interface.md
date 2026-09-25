---
id: REQ-SUP-011
uuid: 5b099658-d5c3-48e3-8d9f-83d56c110bc7
title: Optional transitive interface
scope: sup
type: interface
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M12
verification:
  - unit
---

## Statement

A plugin resolver **may** implement the optional `lang.Transitive` interface to
answer what an external package depends on; the walker **shall** use it where
present and **shall** treat a resolver without it as unable to answer offline.

## Rationale

An ecosystem that cannot answer offline simply does not implement it, and the
language plugin contract stays small.

## Acceptance criteria

1. A plugin whose resolver implements `Dependencies(Target) []Target` is walked
   from its lock files.
2. A plugin whose resolver does not is not walked offline and is recorded as not
   walked.

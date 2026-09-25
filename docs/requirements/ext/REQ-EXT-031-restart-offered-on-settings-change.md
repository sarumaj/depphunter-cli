---
id: REQ-EXT-031
uuid: c5780565-a0a0-4b21-9ee4-3c96fc84e8d4
title: Restart offered on settings change
scope: ext
type: functional
priority: should
status: implemented
source:
  - README.md Settings
verification:
  - manual
---

## Statement

When a `depphunter.*` setting that affects a running server changes, the
extension **should** offer to restart the affected servers, at most one offer at
a time.

## Rationale

A server reads its settings at start-up, so a change takes effect only on
restart; the user is told rather than left to wonder.

## Acceptance criteria

1. Changing `depphunter.theme` with a server running shows one offer to restart;
   accepting restarts that server.
2. Changing only `depphunter.openIn` shows no offer.

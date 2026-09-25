---
id: REQ-EXT-026
uuid: 8006fc50-a7a7-400c-9ea8-ae9e5269181d
title: One server per folder
scope: ext
type: functional
priority: must
status: implemented
source:
  - README.md Use
  - README.md VS Code extension
verification:
  - extension
---

## Statement

The extension **shall** keep at most one server per folder, reuse it for every
later open of that folder, and keep it until the window closes or it is stopped
explicitly.

## Rationale

Analyzing a large repository takes time, and under `--watch` it needs to happen
only once.

## Acceptance criteria

1. Opening the map twice uses the same address.
2. Two opens made while the server is starting share one server.
3. Deactivating the extension stops every server.

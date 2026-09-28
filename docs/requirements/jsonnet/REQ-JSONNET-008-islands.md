---
id: REQ-JSONNET-008
title: Islands
scope: jsonnet
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

jsonnet-bundler packages **shall** form the "jsonnet-bundler packages"
island, with `jsonnet-bundler:` as a private-pattern prefix. No
vulnerability database and no registry **shall** be asked about them.

## Rationale

OSV has no Jsonnet ecosystem and jb has no registry.

## Acceptance criteria

1. The plugin declares the one non-standard island.

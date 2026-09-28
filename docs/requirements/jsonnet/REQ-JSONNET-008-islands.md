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
island, with `jsonnet-bundler:` as a private-pattern prefix. No registry
**shall** be asked about them, and the vulnerability database only about the
commit a lock pins one to, when its repository is on a public forge
(REQ-FND-026).

## Rationale

OSV has no Jsonnet ecosystem and jb has no registry.

## Acceptance criteria

1. The plugin declares the one non-standard island.

---
id: REQ-SUP-020
uuid: ff3c1f6e-d8e3-461c-9fe5-e2b0fbf985b2
title: Online mode asks trusted indexes
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
  - integration
---

## Statement

With `--online` (or `online: true` in the user's configuration) the system
**shall** ask the trusted package indexes what an external package depends on
where the repository does not record it; without it the system **shall not** ask
any index.

## Rationale

Some ecosystems keep their dependency graph outside the repository; `--online`
is what lets `--resolve-depth` reach them.

## Acceptance criteria

1. `--online` together with `--resolve-depth 1` adds the dependencies a Go
   module proxy reports for a directly imported module.
2. A project configuration file setting `online` has no effect.

---
id: REQ-SUP-016
uuid: 5ac379e0-de78-43d9-96e7-58d33815da26
title: Machine configuration is preferred
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

Among sources of the same kind for a package - two scoped sources that both
cover it, or two unscoped ones - an index this machine's configuration names
**shall** be preferred over one the repository names. A scoped source that covers
the package **shall** take precedence over an unscoped one whoever named either
([REQ-SUP-014](REQ-SUP-014-package-carries-its-index.md)); a repository's scoped
source chosen this way is still marked and never fetched from
([REQ-SUP-018](REQ-SUP-018-repository-only-index-marked.md)).

## Rationale

The machine's configuration is what the package manager on this machine would
actually use.

## Acceptance criteria

1. With the same ecosystem configured on the machine and in the repository,
   packages resolve from the machine's index.
2. A repository's `@acme:registry` wins over the machine's unscoped registry for
   `@acme/*` packages, which are then marked as coming from an index nothing
   here vouches for.

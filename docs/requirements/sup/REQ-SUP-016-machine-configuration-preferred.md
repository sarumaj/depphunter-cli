---
id: REQ-SUP-016
uuid: 5ac379e0-de78-43d9-96e7-58d33815da26
title: Machine configuration is preferred
scope: sup
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M12
verification:
  - unit
---

## Statement

An index this machine's configuration names **shall** be preferred over one the
repository names for the same packages.

## Rationale

The machine's configuration is what the package manager on this machine would
actually use.

## Acceptance criteria

1. With the same ecosystem configured on the machine and in the repository,
   packages resolve from the machine's index.

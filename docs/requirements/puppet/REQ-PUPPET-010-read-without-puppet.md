---
id: REQ-PUPPET-010
title: Read without Puppet
scope: puppet
type: limitation
priority: should
status: implemented
verification:
  - unit
---

## Statement

Manifests **shall** be read without compiling a catalog: class names built
from variables, Hiera data (`lookup('classes').include`), tasks called by
plans, and modules whose name Puppet derives differently from their
directory are not followed; only r10k's default and `moduledir` install
directories are read; and the Forge is not asked (no `--online`).

## Rationale

Puppet decides much at catalog compilation; forgeapi.puppet.com was not
reachable when this was written, so no index client was added.

## Acceptance criteria

1. `lookup('classes', ...).include` yields nothing, and no Puppet module is
   named to an index.

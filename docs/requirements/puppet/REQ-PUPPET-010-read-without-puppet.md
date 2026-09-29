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
directory are not followed; and only r10k's default and `moduledir` install
directories are read (the Forge is asked with `--online`, REQ-SUP-069).

## Rationale

Puppet decides much at catalog compilation.

## Acceptance criteria

1. `lookup('classes', ...).include` yields nothing, and without `--online` no
   Puppet module is named to an index.

---
id: REQ-PUPPET-008
title: Islands
scope: puppet
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

Puppet modules **shall** form the "Puppet modules" island, with
`puppet-forge:` as a private-pattern prefix. No vulnerability database
**shall** be asked about them.

## Rationale

OSV has no Puppet Forge ecosystem.

## Acceptance criteria

1. The plugin declares the one island, and OSV's ecosystem map has no entry
   for it.

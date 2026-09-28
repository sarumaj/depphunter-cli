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
**shall** be asked about them by name and version; a module fetched from a
git repository on a public forge at a full commit (`.fixtures.yml`,
Puppetfile `:commit`) is asked about by that commit (REQ-FND-026).

## Rationale

OSV has no Puppet Forge ecosystem.

## Acceptance criteria

1. The plugin declares the one island, and OSV's ecosystem map has no entry
   for it.

---
id: REQ-PUPPET-005
title: Puppetfile, metadata.json and .fixtures.yml
scope: puppet
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

Every module a `Puppetfile` (`mod`), a module's `metadata.json`
(`dependencies`) or a `.fixtures.yml` (`forge_modules`, `repositories`)
names **shall** be an import of its package in the "Puppet modules"
island: a Forge module by its slug (`puppetlabs-stdlib`, from
`puppetlabs/stdlib` too), a git module by its repository (with the URL as
origin). A Puppetfile's exact version and a git `:commit` (or a `:ref`
that is a hash) **shall** pin; `:latest`, no version, a `:branch` and a
`:ref` naming a branch **shall** float; a `:tag` **shall** be shown,
neither pinned nor floating. A `metadata.json` range and a fixture without
`ref` **shall** float. A `.fixtures.yml` repository for a module the
module's `metadata.json` declares **shall** be that Forge module.
`:local` modules **shall** be skipped.

## Rationale

r10k installs exactly what the Puppetfile says; metadata ranges are what
the Forge resolves at install time.

## Acceptance criteria

1. The fixture's Puppetfile entries, the widget module's dependencies and
   its fixtures are pinned, floating and named as stated.

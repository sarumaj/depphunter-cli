---
id: REQ-PUPPET-001
title: Files claimed
scope: puppet
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Puppet plugin **shall** claim manifests (`.pp`), `Puppetfile`,
`.fixtures.yml` and a `metadata.json` in a directory laid out as a Puppet
module (with `manifests/`, `functions/`, `types/`, `plans/`, `tasks/`,
`templates/` or `lib/puppet/`). Nothing in a `modules` directory beside a
`Puppetfile` (what r10k installed) or in `spec/fixtures/modules` (what the
spec helper installed) **shall** be claimed, and a plain walk **shall**
skip both.

## Rationale

Installed modules are the packages', not the repository's; r10k's modules
are read only to resolve names (REQ-PUPPET-006, REQ-PUPPET-007).

## Acceptance criteria

1. The fixture's manifests, Puppetfile, module metadata and fixtures file
   are claimed; `modules/`, `dist/widget/spec/fixtures/modules/` and a
   `metadata.json` outside a module are not.

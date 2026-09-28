---
id: REQ-PUPPET-007
title: Installed dependencies
scope: puppet
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

With `--resolve-depth`, a module r10k installed beside a Puppetfile
**shall** depend on the modules its installed `metadata.json` lists, and
**shall** be reported as installed.

## Rationale

The installed metadata is the only dependency information available
offline.

## Acceptance criteria

1. The installed puppetlabs-apt depends on puppetlabs-stdlib's floating
   range; the installed stdlib depends on nothing; apache, installed
   without a metadata.json, is not reported as installed.

---
id: REQ-PUPPET-006
title: Declared modules
scope: puppet
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A reference to a module the repository does not hold **shall** resolve to
the module the manifests governing the file declare: the `metadata.json`
and `.fixtures.yml` of the module the file is in, the Puppetfiles above
it (nearest first), then any Puppetfile or `metadata.json` of the
repository. Otherwise a module r10k installed beside a Puppetfile
**shall** be named by its `metadata.json` (with its installed version); a
well-known module (`stdlib`, `concat`, `apt`, ...) **shall** be its Forge
slug, unresolved, and any other the bare module name, unresolved.

## Rationale

A control repository declares modules in its Puppetfile; a module in its
metadata.json.

## Acceptance criteria

1. `Stdlib::Port` in the control repository is the Puppetfile's pinned
   stdlib; in the widget module, the metadata's floating range; `include
   apt` is the installed puppetlabs-apt; `docker` and `unknownmod` are
   unresolved.

---
id: REQ-PUPPET-003
title: Symbols
scope: puppet
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

Classes, defined types, nodes (each name or regular expression, and
`default`), functions, type aliases and Bolt plans a manifest declares
**shall** be its symbols; a module's `metadata.json` **shall** have the
module's name.

## Rationale

These are what Puppet's autoloader and other manifests name.

## Acceptance criteria

1. The fixture's nodes, classes, function, type alias, plan and defined
   type are symbols of their files.

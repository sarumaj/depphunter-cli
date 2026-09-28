---
id: REQ-REGO-005
title: No package island
scope: rego
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Rego plugin **shall** declare no island: every Rego dependency is a
file of the repository, and nothing **shall** be asked online.

## Rationale

OPA, Conftest and Regal have no package registry; bundles are not
packages with versions.

## Acceptance criteria

1. The plugin declares no ecosystem.

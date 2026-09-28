---
id: REQ-REGO-001
title: Files claimed
scope: rego
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Rego plugin **shall** claim `.rego` files. An OPA bundle's `.manifest`
and data files (`data.json`, `data.yaml`) **shall** not be claimed.

## Rationale

A bundle manifest names roots, not dependencies; data files hold no
references.

## Acceptance criteria

1. `.rego` files are claimed; `.manifest` and `data.json` are not.

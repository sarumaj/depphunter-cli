---
id: REQ-SUP-035
uuid: 4e05c56f-451a-4e91-b9fa-4bf391b9fe43
title: Ecosystem prefix on a pattern
scope: sup
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M16
verification:
  - unit
---

## Statement

A private pattern **may** be limited to one ecosystem by a prefix naming it
(`npm:@acme/*`); such a pattern **shall** match only that ecosystem's packages.

## Rationale

An npm scope and a Maven group are strings that may occur elsewhere.

## Acceptance criteria

1. `npm:@acme/*` matches the npm package `@acme/ui` and not a package of the
   same name in another ecosystem.
2. `localhost:5000/*` is a host and port, not an ecosystem prefix.

---
id: REQ-JS-008
uuid: 19e9505d-872d-4376-9db6-a731e4005c4e
title: yarn.lock versions
scope: js
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M6
verification:
  - unit
---

## Statement

The plugin **shall** read versions from `yarn.lock` files in both the classic
and the Berry format, **shall** select the entry whose descriptor matches the
range declared in `package.json` (also as `npm:<range>`), and, where the range
is written differently from every descriptor, **shall** accept the locked
version only when the lock holds a single version of the package. The lock
**shall** apply to the `package.json` files in its directory and below.

## Rationale

A yarn.lock can hold several versions of one package; only the declared range
tells which one the project uses.

## Acceptance criteria

1. In a classic lock holding `left-pad` at two versions, the range `1.x` selects
   `1.3.0`.
2. In a Berry lock, `react` declared `^18.2.0` resolves to `18.2.0`, pinned.
3. A dependency named `version-guard` inside an entry is not read as the entry's
   version.

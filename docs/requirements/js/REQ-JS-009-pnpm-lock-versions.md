---
id: REQ-JS-009
uuid: c237fca0-85ec-4873-91e5-33a3f64fe238
title: pnpm-lock.yaml versions
scope: js
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read versions from `pnpm-lock.yaml` (lockfile formats v5 to
v9) per importer, applying each importer's versions to the package directory the
importer names, **shall** drop the peer-dependency suffix `(…)` from a version,
and **shall** ignore `link:` entries.

## Rationale

In a pnpm workspace every package has its own resolved versions.

## Acceptance criteria

1. `react` at the root resolves to `18.3.1`, requested `^18.2.0`.
2. `react-dom/client` in `packages/ui` resolves to `18.3.1` from that importer,
   without the peer suffix.

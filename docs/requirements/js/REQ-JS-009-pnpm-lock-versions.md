---
id: REQ-JS-009
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
and **shall** ignore `link:` entries. Where the file holds several YAML
documents, as recent pnpm writes one locking the package manager itself
before the project's, the last document **shall** be the project's lock. A git
dependency **shall** be pinned to the commit its package's `resolution` names,
from its repository (REQ-JS-019).

## Rationale

In a pnpm workspace every package has its own resolved versions.

## Acceptance criteria

1. `react` at the root resolves to `18.3.1`, requested `^18.2.0`.
2. `react-dom/client` in `packages/ui` resolves to `18.3.1` from that importer,
   without the peer suffix.
3. Behind a first document locking pnpm itself, `a` resolves to the last
   document's `1.0.0`, and nothing of the first document is read.

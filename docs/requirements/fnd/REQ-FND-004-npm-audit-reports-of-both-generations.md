---
id: REQ-FND-004
uuid: 7451c197-1d88-4cf3-a868-f7a1a198d7c1
title: npm audit reports of both generations
scope: fnd
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M13
verification:
  - unit
---

## Statement

The system **shall** read `npm audit --json` reports of npm 7 and later
(per-package `vulnerabilities` with `via` chains) and of npm 6 and earlier (the
flat `advisories` table), naming an advisory by its GHSA or CVE identifier where
the report carries one.

## Rationale

Both report shapes are still produced by pipelines in use.

## Acceptance criteria

1. An npm 7 report yields a finding per advisory with the chain that pulls it
   in.
2. An npm 6 advisory with a CVE is named by the CVE.

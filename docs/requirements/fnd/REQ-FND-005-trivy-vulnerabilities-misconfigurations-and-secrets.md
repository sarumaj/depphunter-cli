---
id: REQ-FND-005
title: Trivy vulnerabilities, misconfigurations and secrets
scope: fnd
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** read Trivy JSON reports, including vulnerabilities,
misconfigurations and secret hits; a secret finding **shall not** carry the
secret value.

## Rationale

Trivy covers packages, infrastructure files and leaked secrets in one report.

## Acceptance criteria

1. A Trivy report with the three result kinds yields vulnerability findings on
   packages and lint findings on the target files with their lines.
2. The detail of a secret finding does not contain the matched value.

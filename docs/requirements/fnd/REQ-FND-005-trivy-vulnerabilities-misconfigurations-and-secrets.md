---
id: REQ-FND-005
uuid: 904f6932-1bde-4850-ad7a-868d9293b4d0
title: Trivy vulnerabilities, misconfigurations and secrets
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

The system **shall** read Trivy JSON reports, including vulnerabilities,
misconfigurations and secret hits; a secret finding **shall not** carry the
secret value.

## Rationale

Trivy covers packages, infrastructure files and leaked secrets in one report.

## Acceptance criteria

1. A Trivy report with the three result kinds yields vulnerability findings on
   packages and lint findings on the target files with their lines.
2. The detail of a secret finding does not contain the matched value.

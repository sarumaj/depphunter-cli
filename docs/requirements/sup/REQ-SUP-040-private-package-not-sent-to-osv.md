---
id: REQ-SUP-040
uuid: 4fa0b77d-0e58-46eb-8fb3-adf842e98d4e
title: A private package is not sent to the vulnerability database
scope: sup
type: constraint
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M16
verification:
  - unit
---

## Statement

The system **shall not** include a private package in a query to the OSV
vulnerability database.

## Rationale

The query would hand the name and version of internal code to a third party.

## Acceptance criteria

1. A run with `--private 'corp.example/*'` sends none of the corp.example
   packages to osv.dev.

## Notes

The OSV query itself is specified by scope [`fnd`](../fnd/). No automated test
checks that private packages are left out.

---
id: REQ-SUP-040
title: A private package is not sent to the vulnerability database
scope: sup
type: constraint
priority: must
status: implemented
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

The OSV query itself is specified by scope [`fnd`](../fnd/).

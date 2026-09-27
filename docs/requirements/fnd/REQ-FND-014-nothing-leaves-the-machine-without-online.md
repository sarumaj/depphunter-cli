---
id: REQ-FND-014
title: Nothing leaves the machine without online
scope: fnd
type: functional
priority: must
status: implemented
verification:
  - unit
  - inspection
---

## Statement

Without `--online` the findings layer **shall** make no network request; a run
with no reports named and without `--online` **shall** ask nothing and show no
vulnerability or lint findings.

## Rationale

Sending a dependency list to a third party requires explicit consent.

## Acceptance criteria

1. A run without `--online` creates no OSV client.
2. A repository with no reports and no `--online` asks nothing and shows no
   scanner findings.

## Notes

Broken-link findings of Markdown documents (scope `md`) are on by default and
appear in the same dataset, so the findings view is not necessarily empty in
that case.

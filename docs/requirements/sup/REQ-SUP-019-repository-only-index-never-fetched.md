---
id: REQ-SUP-019
uuid: d0846d32-4991-4fff-b152-e35d22cbd45f
title: A repository-only index is never fetched from
scope: sup
type: constraint
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M12
verification:
  - unit
---

## Statement

The index client **shall not** send any request to an index that only the
repository names and that the user has not vouched for.

## Rationale

Following the repository's index would let a repository choose where depphunter
sends requests.

## Acceptance criteria

1. With `--online`, a package whose index only the repository's `.npmrc` names
   produces no request, and the report records the reason.

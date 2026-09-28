---
id: REQ-SUP-019
title: A repository-only index is never fetched from
scope: sup
type: constraint
priority: must
status: implemented
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
2. A repository's index asked beside the public default (an extra pip index,
   a POM's repository) receives no request either; the package is asked of the
   public default instead, and the reason is recorded only when that does not
   have it
   ([REQ-SUP-063](REQ-SUP-063-additive-sources-fall-back-to-the-public-index.md)).

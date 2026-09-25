---
id: REQ-TRC-006
uuid: 01b1f6eb-719a-4142-9f53-106c4b6296c8
title: Distinct reasons for no answer
scope: trc
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M18
verification:
  - unit
---

## Statement

For a question nothing answered, the report **shall** record which of these
applies, each as a distinct reason: no lock file covers it and the run is
offline; its index is named only by the repository; it is private and its index
is the public one; a proxy requires a version it was not given; the ecosystem's
index cannot be asked; the ecosystem has no index depphunter asks; or a request
was made and failed (404, 401 or worse).

## Rationale

On the map all of these are the same absence.

## Acceptance criteria

1. A package declined for being private, one whose index only the repository
   names, one whose index answered 404 and one that nothing was asked about are
   four distinguishable entries with four different reasons.
2. The unanswered questions are grouped by reason with counts, commonest first.

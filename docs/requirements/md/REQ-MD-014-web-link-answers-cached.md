---
id: REQ-MD-014
uuid: af7ecd75-ad29-4234-adcd-f4146d935b24
title: Web link answers cached for a day
scope: md
type: functional
priority: must
status: partial
source:
  - docs/REQUIREMENTS.md M19
verification:
  - unit
---

## Statement

The system **shall** keep the answers to external link checks for a day and
reuse them in later runs, and **shall not** keep a refusal, a rate limit or a
server error.

## Rationale

Link rot is slow, and asking a hundred hosts on every re-analysis is the kind of
thing that gets a tool blocked. A refusal or a rate limit may be lifted on the
next run.

## Acceptance criteria

1. A second run within the time to live asks again only about the links whose
   answer was not kept.
2. Answers are kept for 24 hours.

## Notes

Partial: the link cache shares `findingsCacheTTL`, which is 6 hours, not a day
(criterion 2).

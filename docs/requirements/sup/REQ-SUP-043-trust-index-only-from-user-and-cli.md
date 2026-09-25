---
id: REQ-SUP-043
uuid: bf76dabb-71fc-46a7-bb8c-56ca44416d64
title: Only the user may vouch for an index
scope: sup
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** read `trust_indexes` only from the user's own
configuration, the environment and the command line; a `trust_indexes` entry in
a project configuration file **shall** be ignored.

## Rationale

A repository that could clear its own warning would leave no guard at all, which
is the whole point of the marking.

## Acceptance criteria

1. A project configuration naming `trust_indexes` changes nothing.
2. The user's configuration and `--trust-index` are both honoured.

## Notes

`DEPPHUNTER_TRUST_INDEXES` is also read.

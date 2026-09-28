---
id: REQ-SUP-041
title: A repository may declare its packages private
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** honor `private:` in the project configuration file in
addition to the user's configuration, the environment and the command line.

## Rationale

The only effect is that depphunter says less, and the repository is who would
know.

## Acceptance criteria

1. A pattern in the project configuration reaches the private patterns together
   with those from the user configuration, `DEPPHUNTER_PRIVATE` and `--private`.

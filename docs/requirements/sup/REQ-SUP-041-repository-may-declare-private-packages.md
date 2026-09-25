---
id: REQ-SUP-041
uuid: e7293a4b-27e5-412c-9dd1-f8153e580acf
title: A repository may declare its packages private
scope: sup
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M16
verification:
  - unit
---

## Statement

The system **shall** honour `private:` in the project configuration file in
addition to the user's configuration, the environment and the command line.

## Rationale

The only effect is that depphunter says less, and the repository is who would
know.

## Acceptance criteria

1. A pattern in the project configuration reaches the private patterns together
   with those from the user configuration, `DEPPHUNTER_PRIVATE` and `--private`.

---
id: REQ-CFG-007
uuid: 79b6ec8c-4681-46c1-bc39-c8cfb2fb747a
title: Invalid setting values rejected
scope: cfg
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M9
verification:
  - unit
---

## Statement

The system **shall** fail with an error, before analyzing anything, when a
setting from any source has a value that cannot be parsed for its type or lies
outside its allowed values.

## Rationale

A misspelt value that is silently replaced by a default leaves the user
wondering why the setting has no effect.

## Acceptance criteria

1. `DEPPHUNTER_OPEN=maybe` fails with an error.
2. `--theme neon` fails with an error naming the allowed values.
3. `--history-commits 0` fails with an error.

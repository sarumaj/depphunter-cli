---
id: REQ-CLI-003
uuid: f86db6ed-6a91-49b1-b841-cdf97a7048b6
title: POSIX flag syntax
scope: cli
type: interface
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M9
verification:
  - unit
  - integration
---

## Statement

The command **shall** parse its flags with POSIX/GNU syntax through pflag: a
long flag **shall** be accepted as `--flag value` and as `--flag=value`, a
boolean flag as `--flag`, and an unknown flag **shall** be rejected with an
error.

## Rationale

The command moved to cobra and pflag in M9 so that its flags behave like those
of other command-line tools.

## Acceptance criteria

1. `--theme dark` and `--theme=dark` have the same effect.
2. `--no-such-flag` exits with an error.
3. Flags may appear before or after the path argument.

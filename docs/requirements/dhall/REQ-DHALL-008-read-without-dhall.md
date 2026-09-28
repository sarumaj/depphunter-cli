---
id: REQ-DHALL-008
title: Read without dhall
scope: dhall
type: limitation
priority: should
status: implemented
verification:
  - unit
---

## Statement

Dhall files **shall** be read without running `dhall`: remote imports are
not fetched (their own imports are not followed and no `--online` query
exists), `env:` imports are dropped even when the variable names a file of
the repository, and a hash does not tell which version a URL serves.

## Rationale

Following a remote import means downloading and evaluating it.

## Acceptance criteria

1. `env:DHALL_LOCAL` is dropped and a remote import's target carries no
   dependencies.

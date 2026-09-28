---
id: REQ-NIM-009
title: The nimble and standard library islands
scope: nim
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

Nimble packages **shall** form the `nimble` island ("Nimble packages") and
the standard library the hidden `nim-std` island ("Nim standard library");
`nimble:` **shall** be a private-pattern prefix. Nimble packages **shall
not** be asked about by name and version in OSV, which has no Nim ecosystem
(one locked to a full commit is asked about by that commit, REQ-FND-026),
nor mapped from
Trivy, which has no Nim package type, and `--online` has no index to ask:
the official package list (nim-lang/packages' `packages.json`) maps names
to repositories but has no versions or dependencies. nimlangserver or
nimlsp serve `--lsp` references (REQ-LSP-002).

## Rationale

Nothing else in the tool knows nimble packages; they need their own island.

## Acceptance criteria

1. The plugin declares `nimble` and the Std `nim-std`, and every package
   target it returns is in one of them.

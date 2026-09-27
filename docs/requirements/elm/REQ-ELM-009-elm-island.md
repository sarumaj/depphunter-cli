---
id: REQ-ELM-009
uuid: 0412224c-2198-4467-bfa0-fdfeb27e1bf5
title: The elm island
scope: elm
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

Elm packages **shall** form the `elm` island ("Elm packages"), named
`author/name`; `elm:` **shall** be a private-pattern prefix; `--online`
**shall** ask package.elm-lang.org (REQ-SUP-058). Elm packages **shall not**
be asked about in OSV, which has no Elm ecosystem, nor mapped from Trivy,
which has no Elm package type. `elm-language-server --stdio` serves
`--lsp` references for `.elm` files (REQ-LSP-002).

## Rationale

Nothing else in the tool knows Elm packages; they need their own island.

## Acceptance criteria

1. The plugin declares one ecosystem, `elm`, and every package target it
   returns is in it.

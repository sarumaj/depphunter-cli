---
id: REQ-SUP-002
uuid: fe1f0c1d-b8f9-4e62-b148-456b0e148168
title: Pinning follows the ecosystem rule
scope: sup
type: constraint
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M12
verification:
  - unit
  - inspection
---

## Statement

Each language plugin **shall** decide whether a specifier pins by the rule of
its own ecosystem rather than by a rule shared across ecosystems.

## Rationale

The same string means different things per ecosystem: a version in `go.mod` is
the one the build selects, while the same string in `Cargo.toml` is a caret
range and npm reads "1.2" as 1.2.x.

## Acceptance criteria

1. A shortened npm version such as `1.2` is floating, while `1.2.3` is pinned.
2. Each plugin scope documents and tests its own pinning rule.

## Notes

The general helpers are `lang.Pinned`, `lang.PinnedSemver` and `lang.Commit` in
internal/lang/version.go. The per-ecosystem rules are specified by the plugin
scopes: [Go](../go/), [JavaScript](../js/), [Python](../py/), [Rust](../rs/),
[Java/Maven](../java/), [C#/NuGet](../cs/) and [PowerShell](../ps/). Pinning of
CI references (only a commit or an OCI digest pins) is specified by scope
[`ci`](../ci/).

---
id: REQ-RS-009
uuid: c95b860b-1f7e-400c-b70d-8310b11c1ded
title: ripgrep resolves without unresolved crates
scope: rs
type: non-functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M4
verification:
  - e2e
---

## Statement

The Rust plugin **shall** analyze the ripgrep repository with no unresolved
crate.

## Rationale

ripgrep is a multi-crate workspace with path, renamed and workspace-inherited
dependencies; it served as the reference project for the Rust plugin in M4.

## Acceptance criteria

1. An analysis of ripgrep reports 0 unresolved crates.

## Notes

Measured in the design log (M4). No automated test runs against ripgrep.

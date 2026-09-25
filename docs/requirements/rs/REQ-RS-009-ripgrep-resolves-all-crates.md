---
id: REQ-RS-009
uuid: c95b860b-1f7e-400c-b70d-8310b11c1ded
title: ripgrep resolves without unresolved crates
scope: rs
type: non-functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

The Rust plugin **shall** analyze the ripgrep repository with no unresolved
crate.

## Rationale

ripgrep is a multi-crate workspace with path, renamed and workspace-inherited
dependencies; it is the reference project for the Rust plugin.

## Acceptance criteria

1. An analysis of ripgrep reports 0 unresolved crates.

## Notes

A manual measurement; no automated test runs against ripgrep.

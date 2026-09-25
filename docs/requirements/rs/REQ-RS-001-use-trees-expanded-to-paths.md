---
id: REQ-RS-001
uuid: dc7d6cb2-a3bb-4a14-a10b-986d8b52429b
title: Use trees expanded to import paths
scope: rs
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Rust plugin **shall** expand every `use` declaration into the individual
paths it imports, flattening nested `{…}` groups, dropping `as` aliases, and
reducing `self` and glob (`*`) members to the path of their parent, with each
distinct path recorded once as an import on the line of the declaration.

## Rationale

A single `use` tree can import from several modules and crates at once; only its
expanded paths can be resolved to files and packages.

## Acceptance criteria

1. `crate::a::{self, b::{c, d as e}, f::*}` expands to `crate::a`,
   `crate::a::b::c`, `crate::a::b::d` and `crate::a::f`, in that order.
2. A path that occurs twice in one tree is recorded once.
3. `extern crate x` is recorded as an import of `x`.

## Notes

The Rust plugin parses with tree-sitter; the C# and PowerShell plugins do
not (see REQ-CS-005, REQ-PS-009).

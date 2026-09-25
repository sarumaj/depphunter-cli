---
id: REQ-RS-006
uuid: 7ec92bf7-14d9-4d9d-bc80-9daf15f7f5fe
title: Workspace-inherited Cargo dependencies
scope: rs
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M4
verification:
  - unit
---

## Statement

The Rust plugin **shall** resolve a member dependency written as
`{ workspace = true }` to the declaration of the same name in the
`[workspace.dependencies]` table, including its version, package name and path.

## Rationale

A workspace-inherited dependency carries no version of its own; without
following it to the workspace table the version would be lost.

## Acceptance criteria

1. In the test workspace, `use serde::Deserialize` resolves to `serde` with the
   version the workspace table requests (`1.0`), locked to `1.0.200`.

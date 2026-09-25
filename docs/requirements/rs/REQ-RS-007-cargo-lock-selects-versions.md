---
id: REQ-RS-007
uuid: 66eed579-e699-4c8d-b26e-65b4e0635571
title: Cargo.lock selects the version in use
scope: rs
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M4
  - docs/REQUIREMENTS.md M12
verification:
  - unit
---

## Statement

The Rust plugin **shall** report a crates.io dependency at the version
`Cargo.lock` records for it, keeping the manifest's requirement as the requested
version; where `Cargo.lock` holds several versions of one crate, it **shall**
choose the version the requirement names exactly, else the newest one the
requirement accepts when read as a caret range, else the newest one.

## Rationale

The lock file is what the build uses. Lock files often hold two versions of one
crate (syn 1 and syn 2), and which one a dependency means is decided by its own
requirement.

## Acceptance criteria

1. `serde = "1.0"` with `serde 1.0.200` in `Cargo.lock` resolves to version
   `1.0.200`, requested `1.0`, pinned.
2. With `syn 2.0.48` and `syn 1.0.109` locked, `syn = "2"` resolves to `2.0.48`.
3. With `old 0.3.1` locked, `old = "0.3"` resolves to `0.3.1`.
4. A crate that no manifest of its crate declares is reported unresolved.

## Notes

The transitive walk over `Cargo.lock` (`--resolve-depth`) belongs to scope
`sup`.

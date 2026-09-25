---
id: REQ-SUP-024
uuid: a8a25b03-0cbe-4d30-8ebe-4c7ec26cbd0a
title: Crate dependencies from the sparse index
scope: sup
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M12
verification:
  - unit
---

## Statement

The index client **shall** read a crate's dependencies from its line in the
registry's sparse index (RFC 2789): the line of the version asked for, or the
newest line when no version is given.

## Rationale

Every modern Cargo registry speaks the sparse protocol; crates.io serves its
index from a host of its own.

## Acceptance criteria

1. The sparse path of a crate is derived from its lower-cased name.
2. The configured crates.io URL is turned into `index.crates.io`, and a
   `sparse+` URL is used as given.
3. Against a stub index the client returns the crate's dependencies.

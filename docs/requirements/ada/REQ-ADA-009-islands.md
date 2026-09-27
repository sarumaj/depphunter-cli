---
id: REQ-ADA-009
title: Islands
scope: ada
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** declare two islands: `alire` ("Alire crates";
`alire:` is a private-pattern prefix) and `ada-std` ("Ada predefined
units", hidden like other standard libraries), and emit no other.

## Rationale

Crates are what an Ada project depends on; the predefined units ship with
the compiler.

## Acceptance criteria

1. Every fixture import resolves to a file, a directory, `alire`,
   `ada-std` or nothing.

---
id: REQ-CFG-008
uuid: a7dba8db-0f0b-4213-aec6-cae1aed38d37
title: Flags that turn settings off
scope: cfg
type: interface
priority: must
status: implemented
verification:
  - unit
---

## Statement

The flags `--no-open`, `--no-cache` and `--no-history` **shall** turn the
settings `open`, `cache` and `history` off, overriding every config file and
environment variable.

## Rationale

These settings are on by default, so the flag that changes them names the off
state.

## Acceptance criteria

1. `--no-open` with a project file `open: true` yields `open` off.
2. `--no-cache` yields `cache` off.
3. `--no-history` yields `history` off.

## Notes

`--no-vulns` and `--no-links` follow the same rule for the settings of scopes
`fnd` and `md`.

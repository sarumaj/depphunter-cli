---
id: REQ-MOD-011
uuid: 16fc0891-cdfd-43fe-9462-afd906ae322a
title: Private package flag
scope: mod
type: interface
priority: must
status: implemented
verification:
  - unit
---

## Statement

A package node **shall** be able to carry the boolean `private`, marking a
package the organization owns.

## Rationale

The flag lets every consumer of the document, and the network clients that act
on it, tell an internal package from a public one.

## Acceptance criteria

1. A package matched by a `--private` pattern carries `private: true`.
2. Without any declaration no node carries `private`.

## Notes

Which packages match, and what is withheld from them, is specified in scope
`sup`.

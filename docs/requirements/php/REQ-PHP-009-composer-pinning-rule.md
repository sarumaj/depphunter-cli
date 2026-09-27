---
id: REQ-PHP-009
uuid: 1f7af54c-2c8d-484d-b141-ecc8564958d8
title: Composer pinning rule
scope: php
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A package the lock or installed.json holds **shall** be pinned to that
version, with the `composer.json` constraint kept as requested when it
differs. Without one, a constraint **shall** pin only when it names one
version as Composer reads it: a bare version (`1.2.3`, `v1.2.3`, `1.2`),
`=1.2.3`, or a branch with a commit (`dev-main#<sha>`), ignoring a stability
flag (`@beta`) and reading an inline alias (`x as y`) for what it stands for;
ranges, `^`, `~`, wildcards, `||` and branches **shall** float.

## Rationale

Composer, unlike npm and Cargo, reads a bare version as exact, so the
manifest alone can pin.

## Acceptance criteria

1. `monolog/monolog` `^3.0` locked at 3.5.0 is pinned at 3.5.0, requested
   `^3.0`.
2. Unlocked, `1.1.0` pins and `^2.0` floats.
3. OSV is asked about a lock's `v6.4.2` as 6.4.2.

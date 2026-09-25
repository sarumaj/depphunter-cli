---
id: REQ-PS-002
uuid: 08688207-dfed-48d7-afc1-17fdf3cb34f8
title: Import-Module commands
scope: ps
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The PowerShell plugin **shall** record each module named by an `Import-Module`
(or `ipmo`) command as a dependency, reading the first positional argument and
every name given to `-Name`, including comma-separated lists continued over
lines, while skipping the values of parameters that take one
(`-RequiredVersion`, `-MinimumVersion`, `-Prefix`, …) and ignoring a preceding
variable assignment.

## Rationale

`Import-Module` is the common way to load a module at run time; `-Name A, B`
imports several at once.

## Acceptance criteria

1. An `Import-Module` line ending in a backtick, continued by `-Name B,` on the
   next line and `C` on the line after, yields dependencies on `B` and `C`.
2. `$psGet = Import-Module PowerShellGet -PassThru` yields a dependency on
   `PowerShellGet`.
3. `Import-Module SqlServer` with nothing declaring `SqlServer` resolves to an
   unresolved PowerShell Gallery package.

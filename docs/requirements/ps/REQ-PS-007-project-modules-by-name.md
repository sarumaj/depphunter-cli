---
id: REQ-PS-007
uuid: f69f4a32-6a8b-4186-bbbc-d1a7fec28dd8
title: Module names resolve to project modules
scope: ps
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The PowerShell plugin **shall** resolve a module name, case-insensitively, to
the project's module of that name, preferring its manifest (`.psd1`) over its
script module (`.psm1`), before considering built-in or gallery modules.

## Rationale

A repository that ships a module imports it by name; the manifest represents the
module as a whole.

## Acceptance criteria

1. `Import-Module Tools` resolves to `tools/Tools/Tools.psd1` when both
   `Tools.psd1` and `Tools.psm1` exist.

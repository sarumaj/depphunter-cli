---
id: REQ-CS-007
uuid: 6f397e3c-fbc5-4f4e-a081-be2724babdcb
title: NuGet wildcards and ranges float
scope: cs
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M12
verification:
  - unit
---

## Statement

The C# plugin **shall** mark a NuGet dependency as pinned when its version is a
plain version or a single-version range `[x]`, and **shall not** mark it pinned
when its version is a wildcard, a range or absent.

## Rationale

A `PackageReference` version is formally a minimum, but restore installs exactly
that version when it exists; the versions that move are the wildcards (`2.*`)
and the ranges.

## Acceptance criteria

1. `Version="8.0.0"` is pinned.
2. `Version="2.*"` is not pinned (floating).
3. `Version="[1.0,2.0)"` is not pinned.

## Notes

Criterion 3 is covered by the shared `lang.Pinned` tests rather than by a C#
test. The general floating concept belongs to scope `sup`.

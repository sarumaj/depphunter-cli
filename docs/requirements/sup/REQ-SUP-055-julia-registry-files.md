---
id: REQ-SUP-055
title: Julia package dependencies from a registry's files
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The index client **shall** read a Julia package's dependencies from a
registry laid out as files (the General registry,
`https://raw.githubusercontent.com/JuliaRegistries/General/master`, unless
configured otherwise): `<registry>/<dir>/Versions.toml` for the version -
the pinned one, else the newest release a `[compat]` range admits, else the
newest, never a yanked one - then the sections of `Deps.toml` and
`Compat.toml` whose range keys (`"1"`, `"0.21.0"`, `"0 - 0.20.0"`)
hold for it. `<dir>` is `<first letter>/<Name>` in General and what
`Registry.toml` lists in another registry (read once). A dependency's
version is its compat range for that release in the registry's notation,
hyphen ranges spaced (`0.1 - 0.3, 1`), pinned only when it is a whole
`1.2.3`; `julia` is left out and standard libraries are `julia-std`.
Registries other than General installed in a depot (`JULIA_DEPOT_PATH`,
else `~/.julia`) whose repository is on GitHub **shall** be discovered as
trusted sources scoped to the packages their `Registry.toml` lists.

## Rationale

pkg.julialang.org serves registries as tarballs; the registry's git
repository serves each package's files, three requests per package.

## Acceptance criteria

1. Against a stub registry, JSON 0.21.4, `0.21` and no version all give
   Dates and Mmap (standard libraries), Parsers `1 - 2, 3` and
   PrecompileTools 1.2.1 (pinned); 0.21.3 gives Parsers `0.0.0 - 1`; the
   yanked 1.0.0 is not chosen; `Registry.toml` is fetched once.
2. With nothing configured the index for `julia` is General's files URL,
   known; a depot's Acme registry on GitHub serves AcmeBilling from
   `raw.githubusercontent.com/acme/AcmeRegistry/HEAD`, General's packages
   stay with General, and a registry hosted elsewhere is not added.

## Notes

`JULIA_PKG_SERVER` is not read: a package server serves registries as
tarballs, not files.

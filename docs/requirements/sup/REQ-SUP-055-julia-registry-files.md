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
registry's files (the General registry,
`https://raw.githubusercontent.com/JuliaRegistries/General/master`, unless
configured otherwise): `<registry>/<dir>/Versions.toml` for the version -
the pinned one, else the newest release a `[compat]` range admits, else the
newest, never a yanked one - then the sections of `Deps.toml` and
`Compat.toml` whose range keys (`"1"`, `"0.21.0"`, `"0 - 0.20.0"`)
hold for it. `<dir>` is `<first letter>/<Name>` in General read over HTTP
and what `Registry.toml` lists otherwise (read once per registry). A
dependency's version is its compat range for that release in the registry's
notation, hyphen ranges spaced (`0.1 - 0.3, 1`), pinned only when it is a
whole `1.2.3`, and it carries the UUID `Deps.toml` gives it; `julia` is left
out and standard libraries are `julia-std`.

The registries installed in this machine's depots (`JULIA_DEPOT_PATH`,
`;`-separated on Windows and `:`-separated elsewhere, an empty entry being
the default depot; else `~/.julia`) **shall** be read from their copies,
whatever host they came from and without a request: `registries/<Name>/`
holding `Registry.toml`, and `registries/<Name>.toml` whose `path` names the
archive a Pkg server served (read once). General's copy **shall** be read in
place of its files over HTTP. Every other registry **shall** serve the
packages its `Registry.toml` lists, found by UUID (`lang.Target.Registry`,
from `Project.toml`, `Manifest.toml` or a registry's `Deps.toml`) in the
order the depots and their registries come, or by name for a package without
one; a package no installed registry lists under its UUID is General's. With
registries installed and General not among them, General **shall** not be
asked. Answers are cached per UUID.

## Rationale

Pkg keeps a copy of every registry it uses, private ones included, and looks
a package up by UUID in all of them: reading those copies answers for any
host with no network and no credentials, and keeps a private registry's
package from being confused with General's of the same name.

## Acceptance criteria

1. Against a stub registry, JSON 0.21.4, `0.21` and no version all give
   Dates and Mmap (standard libraries), Parsers `1 - 2, 3` and
   PrecompileTools 1.2.1 (pinned), each with its UUID; 0.21.3 gives Parsers
   `0.0.0 - 1`; the yanked 1.0.0 is not chosen; `Registry.toml` is fetched
   once.
2. With nothing configured the index for `julia` is General's files URL,
   known; a depot's Acme registry on GitHub serves AcmeBilling from
   `raw.githubusercontent.com/acme/AcmeRegistry/HEAD`, a registry on another
   host is its repository's URL (an ssh address without its user), and
   General's packages stay with General.
3. With `JULIA_DEPOT_PATH=~/corp-depot:`, an archive registry on a private
   host in `corp-depot` and General and Acme checked out in `~/.julia` answer
   without a request: CorpBilling `1.2` is 1.2.5 (1.3.0 yanked), JSON by
   General's UUID from General's copy, JSON by Corp's UUID from Corp's
   archive, JSON by an unknown UUID from nobody; the archive is read once.
4. Without General among installed registries, General's JSON is asked of
   nobody; with no registry installed, of General's files. General's own
   archive is recognized by its UUID without being opened; a registry path
   outside the registry is ignored.

## Notes

`JULIA_PKG_SERVER` is not asked: a Pkg server serves registries only as
whole archives by tree hash (`/registries`, then `/registry/<uuid>/<hash>`),
which would mean downloading all of General on every run; the copy it
installed in a depot is what is read. The depots bundled with a Julia
installation (which an empty `JULIA_DEPOT_PATH` entry also names) are not
known, since where Julia is installed is not. A registry only git serves with
no copy is not read, with a `no-copy` note
([REQ-TRC-017](../trc/REQ-TRC-017-resolver-notes.md)).

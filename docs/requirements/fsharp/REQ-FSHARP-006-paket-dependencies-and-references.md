---
id: REQ-FSHARP-006
title: paket.dependencies and paket.references
scope: fsharp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

Every `nuget` (and `clitool`) line of `paket.dependencies` **shall** be
an import of the NuGet package, every `github`, `gist`, `git` and
`http` line an import of that remote dependency, per group (`group
Build`), comments (`//`, `#`) and options (`redirects: force`) left
out. Every package line of a project's `paket.references` **shall** be an
import of the package in its group, and a `File:` line an import of the
remote dependency providing that file; a package no Paket file of its root
knows **shall** be unresolved. `exclude` and `alias` settings are not
packages.

A group's `references: strict` option needs no reading: it keeps Paket from
referencing the transitive packages of what `paket.references` names, and the
namespace rule (REQ-FSHARP-008) already links an `open` only to packages a
manifest names, never to one only `paket.lock` lists (unless the curated
namespace table names it, which strict code can open only when it references
it). The transitive packages are still installed, so `paket.lock`'s
dependencies stay on the map (REQ-FSHARP-007).

## Rationale

With Paket a project's packages are named in `paket.references` and
versioned in `paket.dependencies` and `paket.lock`; the `.fsproj` names
none.

## Acceptance criteria

1. `paket.dependencies`' nuget, github, git and http lines and the
   `Build` group's `nuget FAKE < 5` are imports with the targets of
   REQ-FSHARP-007/008.
2. `paket.references`' `Argu`, `Build/FAKE` and `File: Globbing.fs`
   resolve; `Unknown.Package` is unresolved.

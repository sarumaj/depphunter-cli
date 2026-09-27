---
id: REQ-NIX-001
title: Files claimed
scope: nix
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Nix plugin **shall** claim Nix expressions (`.nix`), `flake.lock`, niv's
`nix/sources.json` and npins' `npins/sources.json`, telling `flake.nix` (whose
inputs and outputs are read), `flake.lock` and the two pin files apart from
other Nix and JSON files by name. The output links Nix leaves in a project
(`result`, `result-dev`: symbolic links into the Nix store) **shall** not be
read.

## Rationale

Nix describes how a project is built and what it builds with, in files no
other plugin reads. A `result` link points outside the repository, at a build.

## Acceptance criteria

1. `flake.nix`, `x/flake.lock`, `nix/sources.json` and `npins/sources.json`
   are claimed with their own class; `sources.json` elsewhere is not claimed.
2. With `result` and `result-dev` linked to a directory of `.nix` files,
   only the project's own `default.nix` is analyzed.

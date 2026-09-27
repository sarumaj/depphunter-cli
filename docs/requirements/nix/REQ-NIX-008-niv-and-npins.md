---
id: REQ-NIX-008
uuid: 05786c14-2105-4ede-a103-ec410d1bfd71
title: niv and npins
scope: nix
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

niv's `nix/sources.json` and npins' `npins/sources.json` (GitHub, GitLab,
Forgejo and plain git repositories, releases, tarballs, NixOS channels and
PyPI pins) **shall** be read as packages of the `nix` island, named as
flake references are (a channel as `nixpkgs` at its release), versioned by
the revision (else the version or release), with the branch or release tag
as the requested version, and pinned by their revision or hash. Each source
**shall** be an import of its `sources.json`, and `sources.x` or `pins.x`
in a file that bound `sources = import ./nix/sources.nix` or `pins = import
./npins` **shall** be an edge to that source.

## Rationale

Before flakes, niv and npins are how Nix projects pin nixpkgs and friends.

## Acceptance criteria

1. The fixture's niv `nixpkgs` is `github.com/nixos/nixpkgs` at `fedcba9`,
   requested `nixos-23.11`, pinned; npins' channel is `nixpkgs` at
   `nixos-24.05.1234.abcdef0`, requested `nixos-24.05`.
2. `legacy.nix`'s `import sources.nixpkgs { }` and `pins.nixpkgs` are edges
   to those sources.

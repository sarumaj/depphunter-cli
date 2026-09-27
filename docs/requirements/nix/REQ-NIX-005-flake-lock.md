---
id: REQ-NIX-005
title: flake.lock pins and transitive inputs
scope: nix
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

`flake.lock` (versions 5 to 7) **shall** pin the inputs of the `flake.nix`
beside it: an input resolves through the root node's `inputs` (a follows
path taken from the root) to its node, named by the node's `original`
reference (REQ-NIX-004), versioned by the locked commit shortened to seven
characters (else the original ref, else the content hash's start), pinned
by the locked `rev` or `narHash`, with the original ref (a branch such as
`nixos-24.05`, a tag, FlakeHub's `0.1.*`) as the requested version. Every
node of the lock but the root **shall** be an import of `flake.lock`, and
`--resolve-depth` **shall** follow each node's own `inputs` offline.

## Rationale

The lock is what a flake builds with; its graph holds the inputs of inputs.

## Acceptance criteria

1. The fixture's nixpkgs input is `github.com/nixos/nixpkgs` at `ad57eef`,
   requested `nixos-24.05`, pinned.
2. flake-utils depends on `github.com/nix-systems/default`; home-manager and
   foo, whose `nixpkgs` follows the root's, depend on the root's nixpkgs.

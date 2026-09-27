---
id: REQ-NIX-009
title: Channels
scope: nix
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

A search path (`<nixpkgs>`, `<nixpkgs/nixos>`, `<home-manager>`) **shall**
be a floating package of the `nix` island named by its first element: what
it is depends on each machine's `NIX_PATH`. The registry alias of the same
name (REQ-NIX-004) is the same node.

## Rationale

`import <nixpkgs> { }` is the most common dependency of non-flake Nix code,
and the most unpinned one.

## Acceptance criteria

1. The fixture's `default.nix` imports `<nixpkgs>` and `<nixpkgs/nixos>`:
   one floating `nixpkgs` edge.

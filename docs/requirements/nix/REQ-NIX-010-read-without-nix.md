---
id: REQ-NIX-010
uuid: 367abff5-c3be-4240-808b-423a1c827857
title: Read without evaluating Nix
scope: nix
type: limitation
priority: should
status: implemented
verification:
  - unit
---

## Statement

Nix **shall** be read without evaluating it: paths built with `${...}` or
`+`, `import` of computed values, `callPackage` through variables, overlays
and module options are not followed; nixpkgs packages are recognized by
name only (a local set called `fpkgs` is taken for nixpkgs) and only in the
package lists of REQ-NIX-007; the nixpkgs fetchers of package sources are
not read. There is no Nix registry or package index for `--online`; the
GitHub API is not asked about flake inputs. OSV has no Nix ecosystem and
Trivy no Nix package type, so no advisories are asked for.

## Rationale

Evaluating Nix needs Nix and the network; the map is built from files alone.

## Acceptance criteria

1. `pkgs.fetchgit { ... }` in the fixture's `legacy.nix` yields no import.

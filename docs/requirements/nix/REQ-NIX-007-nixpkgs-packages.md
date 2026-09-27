---
id: REQ-NIX-007
title: Nixpkgs packages
scope: nix
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The elements of package lists - attributes named `buildInputs`,
`nativeBuildInputs`, `propagatedBuildInputs`,
`propagatedNativeBuildInputs`, `checkInputs`, `nativeCheckInputs`,
`packages` (`mkShell`, `home.packages`) and `systemPackages`
(`environment.systemPackages`), through `++`, `lib.optional(s)`, `mkIf`,
`if` and `let` - **shall** be packages of the `nixpkgs` island named by
their attribute path when they come from nixpkgs: `pkgs.hello` (any name
containing `pkgs`, `legacyPackages.<system>.` skipped), a name under `with
pkgs;` (or `with pkgs.python3Packages;`, which prefixes it), and a formal
argument of the file's `callPackage`-style function (`{ openssl }:`).
`.override`/`.overrideAttrs`/`.withPackages` calls name their package;
let-bound names, other function calls and `stdenv`, `lib` and the like are
not packages. Their version **shall** be the governing nixpkgs' (the nearest
flake's `nixpkgs` input, else its NixOS/nixpkgs input, else niv's or npins'
`nixpkgs`), pinned when that is. Inside nixpkgs itself (a repository with
`pkgs/top-level/all-packages.nix`) a package **shall** be its
`pkgs/by-name/<xx>/<name>/package.nix`, anything else dropped.

## Rationale

These lists are a Nix project's system dependencies. Reading only them keeps
the island to what a project builds with.

## Acceptance criteria

1. The fixture's `shell.nix` gives git, nodejs_20, openssl, zlib, systemd,
   cmake and python3, all at nixpkgs `ad57eef`, pinned; `pkgs/hello` gives
   openssl, zlib and `python3Packages.setuptools` but not its let-bound
   `local` or `writeShellScriptBin` results.
2. A shell under an unlocked flake gives floating packages at `nixos-24.05`.

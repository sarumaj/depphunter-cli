---
id: REQ-NIX-004
uuid: d41607f1-e62f-42ec-8664-50f905492044
title: Flake inputs named by URL
scope: nix
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A `flake.nix`'s inputs (`inputs.x.url = ...;`, `inputs = { x.url = ...; y
= { url = ...; flake = false; }; }`, attribute-set references with `type`,
`owner`, `repo`, `ref`, `rev`) and the arguments of `outputs` no input
declares **shall** be imports of packages of the `nix` island, named by
`lang.RepoName` of their source: `github:o/r` is `github.com/o/r` (GitHub
names folded to lower case), `gitlab:` is `gitlab.com/...`, `sourcehut:~u/r`
is `git.sr.ht/~u/r`, `git+https://` and `hg+https://` by the repository,
archives by their repository (`/archive/<ref>`, `/-/archive/`,
`api.github.com/repos/o/r/tarball/<ref>`) or download (a trailing
`-<version>` becoming the version), FlakeHub by `flakehub.com/f/o/r`, and a
NixOS channel tarball as `nixpkgs`. A registry name (`nixpkgs`,
`flake:nixpkgs/nixos-24.05`, an undeclared outputs argument) **shall** be
the registry alias itself (`nixpkgs`, `systems`), not what the registry
resolves it to. `path:` inputs **shall** be edges to that directory's
`flake.nix` (else the directory); `follows` **shall** give no node of its
own but the followed input's. `inputs.x` in other files **shall** be an
edge to the nearest flake's input `x`. `builtins.fetchTarball`,
`fetchGit`, `fetchTree` and `getFlake` with literal arguments are named
likewise; the nixpkgs fetchers of package sources (`pkgs.fetchgit`,
`fetchFromGitHub`) are not read.

## Rationale

A flake's inputs are its dependencies; a URL is their only name.

## Acceptance criteria

1. The fixture's `github:NixOS/nixpkgs/nixos-24.05` is
   `github.com/nixos/nixpkgs`, `gitlab:acme/tools/v1.4.0` is
   `gitlab.com/acme/tools`, `https://example.org/downloads/baz-1.2.tar.gz` is
   `example.org/downloads/baz` 1.2, `path:./sub` is `sub/flake.nix`,
   `same.follows = "nixpkgs"` is the nixpkgs input and the undeclared
   `systems` argument the registry's `systems`.
2. `inputs.foo.packages...` in a NixOS module is the flake's `foo`.

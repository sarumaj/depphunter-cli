---
id: REQ-NIX-002
uuid: 928e9328-e4a8-4e53-9b0f-54e8bcd206dd
title: Imports and paths
scope: nix
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

`import ./x.nix`, `callPackage ./x { }` (also `pkgs.callPackage`,
`callPackages` and `callPackageWith`) and the paths listed in a NixOS
module's `imports` (lists joined with `++` too) **shall** be edges to the
named file, a directory meaning its `default.nix`. Any other relative path
literal (`builtins.readFile ./VERSION`, `src = ./src`, `patches = [
./fix.patch ]`, a path interpolated in a string, `"${./script.sh}"`)
**shall** be an edge to that file or directory. `./.`, absolute paths, `~/`
paths, paths built with `${...}` and paths leaving the repository are not
edges; a path to nothing in the repository is dropped.

## Rationale

These are how Nix files compose; a module tree or an overlay is only
visible through them.

## Acceptance criteria

1. In the fixture, `import ./lib` is `lib/default.nix`, `imports = [
   ./services.nix ./users ] ++ [ ../lib/options.nix ]` gives three edges, and
   `builtins.readFile ../lib/VERSION` is an edge to that file.
2. `${./src/hello.c}` inside an indented string is an edge, while the
   string's text (`import ./fake.nix`) and an escaped `''${./escaped.nix}`
   are not.

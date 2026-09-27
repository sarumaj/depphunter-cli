---
id: REQ-NIX-006
title: Pinning without a lock
scope: nix
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A reference without a lock entry **shall** pin when it names a commit (the
`rev=` parameter, a 40-hex path segment or archive name) or a content hash
(`narHash=`, a fetcher's `sha256`/`hash`, a pinned FlakeHub URL); a tag (a
`v1.2.3`-like ref or `refs/tags/...`) **shall** be shown and neither pin nor
float; a branch (`nixos-24.05`, `main`, `refs/heads/...`), a channel, a
registry name or no ref at all **shall** float.

## Rationale

The repository-wide rule: only a commit or a hash names one source.

## Acceptance criteria

1. In `examples/nolock`, `github:acme/commit/<commit>` pins, `v2.0.1` is
   neither, `nixos-24.05` and `git+ssh://...?ref=refs/heads/main` float and a
   `narHash` pins.
2. `builtins.fetchTarball` of a GitHub archive of a commit pins, `fetchGit`
   with `ref = "main"` floats.

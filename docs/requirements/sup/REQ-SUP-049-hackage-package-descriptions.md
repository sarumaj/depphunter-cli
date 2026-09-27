---
id: REQ-SUP-049
uuid: f6b691f3-d905-4714-9c85-e7915fd29149
title: Haskell package dependencies from Hackage
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The index client **shall** read a Haskell package's dependencies from a
Hackage server (`https://hackage.haskell.org` unless configured otherwise):
the package's preferred versions (`<server>/package/<name>/preferred`, JSON),
then the package description of the version asked for, else of the newest
normal version (`<server>/package/<name>-<version>/<name>.cabal`), whose
libraries' `build-depends` (named sublibraries, conditional blocks and the
common stanzas they import included, GHC's own packages and the package's
sublibraries left out) are the answer, `==x` as a pinned version. The
`repository` stanzas of `cabal.project` and `cabal.project.local` are the
repository's indexes; those of `~/.config/cabal/config`, `~/.cabal/config`
and the files `CABAL_CONFIG` or `CABAL_DIR` name are this machine's. Hackage
itself is never recorded as a repository's own index.

## Rationale

Hackage serves every release's package description, revisions applied; its
preferred-versions document tells normal from deprecated releases.

## Acceptance criteria

1. Against a stub Hackage, acme-json 1.2.0 depends on bytestring, containers
   `^>=0.6` (from a common stanza), text `2.0.2` (pinned) and vector (under a
   flag), not on base, template-haskell, its sublibrary or its test suite's
   hspec; a range gets the newest normal version, not a deprecated one.
2. `cabal.project`'s head.hackage repository is the repository's; a mirror in
   `~/.cabal/config` or `$CABAL_DIR/config` is this machine's.

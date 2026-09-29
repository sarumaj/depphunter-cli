---
id: REQ-SUP-049
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
sublibraries left out) are the answer, `==x` as a pinned version; a package
the server does not list, or lists without a normal version, is not there.
The `repository` stanzas of `cabal.project` and `cabal.project.local` are the
repository's indexes; those of the configuration file cabal reads
([REQ-SUP-064](REQ-SUP-064-tool-configuration-locations.md)) are this
machine's. Hackage itself is never recorded as a repository's own index.

As cabal combines the packages of every repository configured, each
repository **shall** be asked before Hackage and Hackage after it; a stanza
named `hackage.haskell.org` with another URL **shall** replace Hackage, and a
configuration that lists repositories without Hackage **shall** leave it out.
When `active-repositories` is set (the repository's `cabal.project.local`
over its `cabal.project` over this machine's configuration), only the
repositories it names **shall** be asked, the list searched last to first as
cabal does: `:rest` standing for every configured repository it does not
name (Hackage among them unless the configuration leaves it out), `:none`
for none, and a `:override` or `:merge` mode ignored beyond that order. A
local repository (`file:`, `file+noindex:`) is not asked.

## Rationale

Hackage serves every release's package description, revisions applied; its
preferred-versions document tells normal from deprecated releases.

## Acceptance criteria

1. Against a stub Hackage, acme-json 1.2.0 depends on bytestring, containers
   `^>=0.6` (from a common stanza), text `2.0.2` (pinned) and vector (under a
   flag), not on base, template-haskell, its sublibrary or its test suite's
   hspec; a range gets the newest normal version, not a deprecated one.
2. `cabal.project`'s head.hackage repository is the repository's, asked
   before Hackage; a mirror named `hackage.haskell.org` in `~/.cabal/config`
   is this machine's and replaces Hackage; `$CABAL_DIR/config` listing only a
   company repository leaves Hackage out.
3. `active-repositories: hackage.haskell.org, corp:override` asks corp, then
   Hackage; `:rest, corp` asks corp first; `:none` asks nothing; the
   repository's `cabal.project.local` wins over its `cabal.project` and over
   this machine's list, which returns when the repository stops setting one.
4. End to end, a package the company repository has is never named to
   Hackage, and one it lacks (or has only deprecated releases of) is answered
   by Hackage.

## Notes

cabal merges the versions of every active repository (an `:override`
repository hiding the earlier ones' versions of the packages it has);
depphunter takes the first repository in its order that has the package.

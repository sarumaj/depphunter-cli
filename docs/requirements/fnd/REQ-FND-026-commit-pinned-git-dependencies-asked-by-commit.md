---
id: REQ-FND-026
title: Commit-pinned git dependencies asked about by commit
scope: fnd
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

With `--online`, the system **shall** also ask the OSV database about every
external package the map fixes to a full git commit (40 hex digits, or 64 for
SHA-256), in the same batch and by the commit alone (`{"commit": "<sha>"}`),
whatever the package's ecosystem, and place each advisory the commit matches
on that package, reported by the source `OSV (git commit)`, with the fixed
commit of the affected `GIT` range of its repository and a detail naming the
commit. The commit **shall** be read from a full-commit version (Carthage,
Zig, Paket GitHub and git files, CMake `FetchContent`, Terraform git module
sources, SwiftPM revisions, git submodules, jsonnet-bundler, dub, fpm, nimble,
haxelib, Alire, Quicklisp, Soldeer, CocoaPods, CRAN remotes, Hackage,
opam, Clojure and Julia git dependencies, mix, rebar3 and Gleam git
dependencies, PureScript git packages, Puppet git modules, Bazel
`git_override` and `git_repository`), from a shard's
`<version>+git.commit.<sha>`, from an npm `github:owner/repo#<sha>` or
`git+<url>#<sha>` version, from Composer's `dev-<branch>#<sha>`, and from the
checkout a plugin records beside a version that shows something else: a
Bundler `GIT` section's revision, a Cargo.lock `git+…#<sha>` source, a
Python direct reference `@ git+<url>@<sha>`, a composer.lock branch
package's source reference and a Nix flake input, niv or npins source locked
to a commit (whose version shows the commit shortened).

A commit **shall** be sent only when its repository is on a public forge
(github.com, gitlab.com, bitbucket.org, codeberg.org, sr.ht) or, where the
package names no repository, when it belongs to an ecosystem whose plugin
records a repository on any other host as its origin (shards, Alire, haxelib,
Soldeer, fpm, nimble, Quicklisp) and has no origin; never when a private
pattern (`--private`, `GOPRIVATE`) matches the package or its repository, or
the package is private for any reason other than having been installed from
a repository on a public forge. A package private only for that reason is
asked about by its commit and never by its name and version, and a version
that is itself a git reference is not sent as a version.

An advisory that both the package's name and version and its commit return,
by id or by one of its aliases, **shall** be reported once, under the name and
version. Two packages at one commit **shall** be one question.

## Rationale

Git dependencies are how the ecosystems OSV does not cover get their code,
and how the others get unreleased code; OSV records the commit ranges of the
repositories its advisories affect, so a commit can be answered for where a
name cannot. A commit of a public repository discloses nothing its forge does
not publish; a commit of a repository on the organization's own host does.

## Acceptance criteria

1. The batch holds `{"commit": …}` queries for full commits only; a shortened
   commit (Bun's seven digits) is not sent.
2. A zig package's commit that OSV matches yields a finding on that package,
   source `OSV (git commit)`, fixed at its repository's `GIT` range fix.
3. A gem from GitHub (private by origin) is asked about by its commit and not
   by its name and version; a Carthage dependency on a company host, a
   package a private pattern matches and one whose repository a private
   pattern matches are not asked about by commit.
4. An advisory returned for a Swift package's name and version and, under an
   alias, for its commit is one finding.

## Notes

A shortened commit could match another repository's commit, so it is not a
pin anybody can be asked about: Bun's `bun.lock` git resolutions and a Nix
tarball input without a locked commit are not asked about by commit. Nor are
an npm package-lock or yarn.lock git dependency whose version is a release, a
git submodule with a relative URL (on the superproject's host), a Racket
package pinned by its catalog's checksum, a GitHub Actions or GitLab CI
reference (the host may be GitHub Enterprise or the instance), and Go
pseudo-versions (Go has its own OSV ecosystem).

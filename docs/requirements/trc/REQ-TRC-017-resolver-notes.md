---
id: REQ-TRC-017
title: Resolvers and indexes add notes
scope: trc
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The report **shall** carry notes: what a plugin's resolver or the index client
knows about the run that no single question shows. Each note **shall** name
the plugin (the walk's, when the index client gives it), the project file it
is about when there is one, a code (`lock-unread`, `lock-flat`,
`lock-ignored`, `no-key`, `forbidden`, `unmapped`, `no-release`,
`no-copy`, `helper-not-run`) and a sentence. A note
given more than once **shall** be kept once, and the notes **shall** be listed
in the same order in every run (by plugin, file, code, sentence). A
resolver's notes **shall** be recorded whether or not the walk ran.

The notes **shall** appear in the text digest (one per line, whole, at most
50 with a count of the rest), in the Markdown document (a table) and in the
JSON (`notes`). At least these **shall** be noted:

- `bun.lockb` without a `bun.lock` or `yarn.lock` beside it, which is not
  read ([REQ-JS-017](../js/REQ-JS-017-bun-lockb-not-read.md));
- a lock that pins versions but records no edges, so that offline the walk
  adds nothing past it: `rebar.lock` without a `mix.lock` beside it, a Conan 2
  `conan.lock`, `Package.resolved` without `.build/checkouts`,
  `pubspec.lock`, opam's `*.opam.locked` without a `dune.lock` directory, and
  `luarocks.lock`;
- a lock the scan left out (git-ignored or excluded) that a resolver read from
  disk: `rebar.lock`, `mix.lock`, `Package.resolved`, `pubspec.lock`,
  `luarocks.lock` and Gleam's `manifest.toml`;
- a Hex organization this machine has no key for, and one whose API refused
  the key sent (403)
  ([REQ-SUP-047](../sup/REQ-SUP-047-hex-api.md));
- a NuGet package no `packageSourceMapping` pattern covers while a mapping is
  in force, which NuGet itself would not restore
  ([REQ-SUP-065](../sup/REQ-SUP-065-nuget-configuration-layers-and-source-mapping.md));
- a CPAN release MetaCPAN does not describe: a pinned version it has no
  release of, or a mirror's distribution
  ([REQ-SUP-053](../sup/REQ-SUP-053-metacpan-releases.md));
- an opam repository, Alire index, Julia registry or CocoaPods spec
  repository only git serves, with no copy on this machine
  ([REQ-SUP-054](../sup/REQ-SUP-054-opam-repository-files.md),
  [REQ-SUP-061](../sup/REQ-SUP-061-alire-community-index.md),
  [REQ-SUP-055](../sup/REQ-SUP-055-julia-registry-files.md),
  [REQ-SUP-074](../sup/REQ-SUP-074-cocoapods-spec-repository-clones.md));
- a Bazel registry a `.bazelrc` names a credential helper for, which is not
  run ([REQ-BAZEL-011](../bazel/REQ-BAZEL-011-read-without-bazel.md)), and a
  Conan remote that wants a login this machine does not hold while Conan's
  `auth_remote.py` plugin could supply one, which is not run
  ([REQ-AUTH-035](../auth/REQ-AUTH-035-conan-remote-logins.md)).

## Rationale

These cases leave packages on the map without dependencies, and nothing in
the per-question part of the report says why: a flat lock answers nothing
and is indistinguishable from a package with none.

## Acceptance criteria

1. The same note given twice is kept once, a note without a plugin is filed
   under the walk's, and the notes are sorted.
2. The digest, the Markdown and the JSON carry the notes; a report without
   notes has no notes heading.
3. A project with only `bun.lockb` yields one `lock-unread` note under the
   JavaScript plugin.
4. Each note source above yields its note, and not when the lock beside it or
   the checkouts answer instead.

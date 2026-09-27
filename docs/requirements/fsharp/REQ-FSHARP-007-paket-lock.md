---
id: REQ-FSHARP-007
title: paket.lock pins and dependencies
scope: fsharp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The F# plugin **shall** read `paket.lock`'s groups and their NUGET,
GITHUB, GIST, GIT and HTTP sections, and make every entry an import. A
locked NuGet package **shall** be pinned at its locked version (the Main
group's when several groups lock it), with the `paket.dependencies`
constraint as the requested version when it differs; the lock's
dependency lines **shall** answer `--resolve-depth` offline, each at its
own locked version. Remote dependencies **shall** form a separate island
named by where they come from: `github.com/owner/repo`,
`gist.github.com/owner/id`, or a git or HTTP URL without its scheme; a
locked commit pins them (the `paket.dependencies` ref as requested), a
commit written in `paket.dependencies` pins them too, another ref is
shown and floats, and no ref and every HTTP file float. Each directory
with a `paket.dependencies` (or `paket.lock`) **shall** be a root of its
own: a file sees the nearest root's versions first.

## Rationale

The lock is what Paket restores, and it records the graph between the
packages; files from GitHub are pinned by commit.

## Acceptance criteria

1. `Argu` resolves to 6.1.1, pinned, requested `>= 6.1`; `FAKE` of the
   Build group to 4.64.18; `fsharp/FAKE` to its locked commit; the git
   dependency locked at a commit to that commit, requested `master`.
2. Two roots locking `Argu` at 6.1.1 and 5.5.0 give each file its own
   root's version and constraint; `Argu`'s lock dependency `FSharp.Core`
   comes back pinned at 8.0.400.

---
id: REQ-SUP-079
title: Configuration files in the directories above the analyzed one
scope: sup
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The configuration files Yarn Berry and NuGet look for in every directory
above a project up to the file system's root **shall** be read in the
directories above the analyzed directory: Yarn's (the name
`YARN_RC_FILENAME` gives it, else `.yarnrc.yml`) and NuGet's (on Windows
`NuGet.Config`; elsewhere the first of `nuget.config`, `NuGet.config` and
`NuGet.Config`), closest first.

Those directories **shall** be split at the top of the checkout the analyzed
directory belongs to: the nearest directory above it holding a `.git` (a
directory, or a worktree's file), unless the analyzed directory holds one
itself. A file of a directory between the analyzed directory and that top
**shall** be the repository's, like the files the scan finds: its registries
and feeds are not trusted, and it gives this machine's credentials nothing
([REQ-AUTH-023](../auth/REQ-AUTH-023-repository-feed-credentials-from-the-environment.md)).
A file of a directory outside the checkout (or of any directory above the
analyzed one, when no `.git` is found) **shall** be this machine's
configuration: its registries and feeds are trusted, and its credentials are
read ([REQ-AUTH-024](../auth/REQ-AUTH-024-yarn-and-bun-credentials.md),
[REQ-AUTH-004](../auth/REQ-AUTH-004-nuget-package-source-credentials.md)).

Yarn's files **shall** be merged as Yarn merges them, key by key at every
depth, the closest winning: the repository's among themselves (a scanned file
over those of the directories above it), and this machine's with the home
directory's `.yarnrc.yml`, which comes last. NuGet's **shall** take their
place in NuGet's order: the repository's after the scanned `nuget.config`
files, this machine's after them and before the user's `NuGet.Config`
([REQ-SUP-065](REQ-SUP-065-nuget-configuration-layers-and-source-mapping.md)).

## Rationale

A developer keeps a `.yarnrc.yml` or a `nuget.config` in a directory above
all of their checkouts - `~/work`, a drive's root - to point every project at
the company registry; Yarn and NuGet read it for each project below. Without
it the private registry is neither asked nor sent its credential. A file
inside the checkout the scan did not reach (a subdirectory was analyzed) is
still the repository's, and must not choose where this machine's secrets go.

## Acceptance criteria

1. With a `.git` in a directory above the analyzed one, the directories up to
   it are the repository's and the rest this machine's; with the `.git` in the
   analyzed directory, or none, all of them are this machine's.
2. `YARN_RC_FILENAME` names the file looked for; the home's `.yarnrc.yml`
   comes last and once.
3. A `.yarnrc.yml` above the checkout gives trusted registries, and its
   scope token, merged with the home's scope registry, reaches that registry;
   one between gives an untrusted registry, and a scanned file's
   `${NPM_TOKEN}` merged with its registry is lent there once vouched for.
4. NuGet's sources come out farthest first: the additional user files, the
   user's `NuGet.Config`, the file above the checkout (trusted), the file
   between (untrusted), the project's (untrusted); only the file above the
   checkout gives a credential.

## Notes

When depphunter is pointed at a directory that is not a checkout, and none
above it holds a `.git`, every file above it is taken as this machine's, as
the user's own configuration would be. The analyzed directory is the one
depphunter is given; files of other package directories in the repository
are merged along their own paths up to the checkout's top.
